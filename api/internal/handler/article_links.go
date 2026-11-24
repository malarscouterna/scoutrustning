package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/malarscouterna/scoutrustning/api/internal/auth"
	"github.com/malarscouterna/scoutrustning/api/internal/db"
)

const (
	maxArticleLinks     = 20
	maxArticleLinkLabel = 120
	maxArticleLinkURL   = 2048
)

// ArticleLinkInput is a link supplied with an article create/update request.
type ArticleLinkInput struct {
	Label      string `json:"label"`
	URL        string `json:"url"`
	Visibility string `json:"visibility"`
}

// ArticleLinkView is the public shape of a link in article responses.
type ArticleLinkView struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	URL        string `json:"url"`
	Visibility string `json:"visibility"`
}

// editArticleLinkView adds checker state for the editor/settings views.
type editArticleLinkView struct {
	ID             string `json:"id"`
	Label          string `json:"label"`
	URL            string `json:"url"`
	Visibility     string `json:"visibility"`
	LastStatus     string `json:"last_status"`
	LastHTTPStatus *int32 `json:"last_http_status"`
	LastError      string `json:"last_error"`
	FailureKind    string `json:"failure_kind"`
	LastCheckedAt  any    `json:"last_checked_at"`
	Acknowledged   bool   `json:"acknowledged"`
}

// linkScopeKey mirrors the SQL expression that scopes links to a product
// type: the commercial name, falling back to the common name when empty.
func linkScopeKey(commercialName, commonName string) string {
	if commercialName != "" {
		return commercialName
	}
	return commonName
}

func normalizeArticleLinks(in []ArticleLinkInput) ([]ArticleLinkInput, error) {
	if len(in) > maxArticleLinks {
		return nil, errors.New("too many links")
	}
	out := make([]ArticleLinkInput, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, link := range in {
		label := strings.TrimSpace(link.Label)
		if label == "" || utf8.RuneCountInString(label) > maxArticleLinkLabel {
			return nil, errors.New("link label must be 1-120 characters")
		}
		raw := strings.TrimSpace(link.URL)
		if raw == "" || len(raw) > maxArticleLinkURL || strings.ContainsAny(raw, " \t\r\n") {
			return nil, errors.New("link url must be a valid http(s) address")
		}
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, errors.New("link url must be a valid http(s) address")
		}
		visibility := link.Visibility
		if visibility == "" {
			visibility = "public"
		}
		if visibility != "public" && visibility != "trusted" {
			return nil, errors.New("link visibility must be public or trusted")
		}
		if _, ok := seen[raw]; ok {
			return nil, errors.New("duplicate link url")
		}
		seen[raw] = struct{}{}
		out = append(out, ArticleLinkInput{Label: label, URL: raw, Visibility: visibility})
	}
	return out, nil
}

// replaceArticleLinks makes the given list the full set for a scope. Existing
// rows with the same URL are updated in place so checker state is preserved.
func replaceArticleLinks(ctx context.Context, q *db.Queries, groupID, scope, createdBy string, links []ArticleLinkInput) error {
	urls := make([]string, 0, len(links))
	for _, link := range links {
		if _, err := q.UpsertArticleLink(ctx, db.UpsertArticleLinkParams{
			GroupID:    groupID,
			ScopeKey:   scope,
			Label:      link.Label,
			Url:        link.URL,
			Visibility: link.Visibility,
			CreatedBy:  createdBy,
		}); err != nil {
			return err
		}
		urls = append(urls, link.URL)
	}
	if len(urls) == 0 {
		return q.DeleteArticleLinksForScope(ctx, db.DeleteArticleLinksForScopeParams{
			GroupID: groupID, ScopeKey: scope,
		})
	}
	return q.DeleteArticleLinksNotIn(ctx, db.DeleteArticleLinksNotInParams{
		GroupID: groupID, ScopeKey: scope, Urls: urls,
	})
}

// upsertArticleLinks adds links to a scope without removing existing ones.
// Used when creating an article: creating another copy of a product type must
// not wipe the shared link set.
func upsertArticleLinks(ctx context.Context, q *db.Queries, groupID, scope, createdBy string, links []ArticleLinkInput) error {
	for _, link := range links {
		if _, err := q.UpsertArticleLink(ctx, db.UpsertArticleLinkParams{
			GroupID:    groupID,
			ScopeKey:   scope,
			Label:      link.Label,
			Url:        link.URL,
			Visibility: link.Visibility,
			CreatedBy:  createdBy,
		}); err != nil {
			return err
		}
	}
	return nil
}

// moveArticleLinks follows a product type rename.
func moveArticleLinks(ctx context.Context, q *db.Queries, groupID, oldScope, newScope string) error {
	if oldScope == "" || newScope == "" || oldScope == newScope {
		return nil
	}
	if err := q.DeleteArticleLinkDuplicates(ctx, db.DeleteArticleLinkDuplicatesParams{
		GroupID: groupID, NewScopeKey: newScope, OldScopeKey: oldScope,
	}); err != nil {
		return err
	}
	_, err := q.MoveArticleLinks(ctx, db.MoveArticleLinksParams{
		GroupID: groupID, OldScopeKey: oldScope, NewScopeKey: newScope,
	})
	return err
}

// applyLinkScopeChange handles links after a rename. Links follow the rename
// only when the old product type has no remaining copies and the new name has
// no links of its own. If the old name is still in use, those copies keep the
// links; if the new name already exists, its links win (a deliberate merge).
func applyLinkScopeChange(ctx context.Context, q *db.Queries, groupID string, articleID pgtype.UUID, oldScope, newScope string) error {
	if oldScope == "" || newScope == "" || oldScope == newScope {
		return nil
	}
	inUse, err := q.ArticleScopeInUse(ctx, db.ArticleScopeInUseParams{
		GroupID: groupID, ScopeKey: oldScope, ExcludeID: articleID,
	})
	if err != nil {
		return err
	}
	if inUse {
		return nil
	}
	targetCount, err := q.CountArticleLinksForScope(ctx, db.CountArticleLinksForScopeParams{
		GroupID: groupID, ScopeKey: newScope,
	})
	if err != nil {
		return err
	}
	if targetCount > 0 {
		// Merging into an existing product type: keep its links, drop the old set.
		return q.DeleteArticleLinksForScope(ctx, db.DeleteArticleLinksForScopeParams{
			GroupID: groupID, ScopeKey: oldScope,
		})
	}
	return moveArticleLinks(ctx, q, groupID, oldScope, newScope)
}

func toArticleLinkView(link db.ArticleLink) ArticleLinkView {
	return ArticleLinkView{
		ID:         formatUUID(link.ID),
		Label:      link.Label,
		URL:        link.Url,
		Visibility: link.Visibility,
	}
}

func toEditArticleLinkView(link db.ArticleLink) editArticleLinkView {
	view := editArticleLinkView{
		ID:           formatUUID(link.ID),
		Label:        link.Label,
		URL:          link.Url,
		Visibility:   link.Visibility,
		LastStatus:   link.LastStatus,
		LastError:    link.LastError,
		FailureKind:  link.FailureKind,
		Acknowledged: link.AcknowledgedKind != "" && link.AcknowledgedKind == link.FailureKind,
	}
	if link.LastHttpStatus.Valid {
		code := link.LastHttpStatus.Int32
		view.LastHTTPStatus = &code
	}
	if link.LastCheckedAt.Valid {
		view.LastCheckedAt = link.LastCheckedAt.Time
	}
	return view
}

// fetchVisibleArticleLinks returns links for the given scopes, filtered to what
// the caller may see: trusted links only for trusted+ access.
func fetchVisibleArticleLinks(ctx context.Context, q *db.Queries, claims auth.Claims, scopes []string) map[string][]ArticleLinkView {
	result := make(map[string][]ArticleLinkView, len(scopes))
	if len(scopes) == 0 {
		return result
	}
	links, err := q.ListArticleLinksForScopes(ctx, db.ListArticleLinksForScopesParams{
		GroupID:        claims.GroupID,
		ScopeKeys:      scopes,
		IncludeTrusted: auth.AccessAtLeast(claims.MaxAccess, auth.AccessTrusted),
	})
	if err != nil {
		slog.Error("failed to list article links", "error", err)
		return result
	}
	for _, link := range links {
		result[link.ScopeKey] = append(result[link.ScopeKey], toArticleLinkView(link))
	}
	return result
}

// attachVisibleLinks adds a "links" array to articles and article-like rows.
// It works on the JSON shape rather than concrete row types because the article
// list has several variants (availability, bookings, plain list).
func attachVisibleLinks(ctx context.Context, q *db.Queries, claims auth.Claims, v any) any {
	if v == nil {
		return v
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return v
	}

	var rows []map[string]any
	switch typed := decoded.(type) {
	case []any:
		for _, item := range typed {
			if row, ok := item.(map[string]any); ok {
				rows = append(rows, row)
			}
		}
	case map[string]any:
		rows = append(rows, typed)
	default:
		return v
	}

	scopeSet := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if scope := scopeKeyFromJSON(row); scope != "" {
			scopeSet[scope] = struct{}{}
		}
	}
	scopes := make([]string, 0, len(scopeSet))
	for scope := range scopeSet {
		scopes = append(scopes, scope)
	}

	byScope := fetchVisibleArticleLinks(ctx, q, claims, scopes)
	for _, row := range rows {
		links := byScope[scopeKeyFromJSON(row)]
		if links == nil {
			links = []ArticleLinkView{}
		}
		row["links"] = links
	}
	return decoded
}

func scopeKeyFromJSON(row map[string]any) string {
	commercial, _ := row["commercial_name"].(string)
	if commercial != "" {
		return commercial
	}
	common, _ := row["common_name"].(string)
	return common
}

func (h *ArticleHandler) articleScopeKey(w http.ResponseWriter, r *http.Request) (string, bool) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return "", false
	}
	scope, err := h.Q.GetArticleScopeKey(r.Context(), db.GetArticleScopeKeyParams{ID: id, GroupID: claims.GroupID})
	if err != nil {
		WriteError(w, http.StatusNotFound, "article not found")
		return "", false
	}
	return scope, true
}

func (h *ArticleHandler) ListLinks(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	scope, ok := h.articleScopeKey(w, r)
	if !ok {
		return
	}
	links, err := h.Q.ListArticleLinksForScope(r.Context(), db.ListArticleLinksForScopeParams{
		GroupID: claims.GroupID, ScopeKey: scope,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to list links")
		return
	}
	views := make([]editArticleLinkView, 0, len(links))
	for _, link := range links {
		views = append(views, toEditArticleLinkView(link))
	}
	WriteJSON(w, http.StatusOK, views)
}

func (h *ArticleHandler) CreateLink(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	scope, ok := h.articleScopeKey(w, r)
	if !ok {
		return
	}
	var req ArticleLinkInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	links, err := normalizeArticleLinks([]ArticleLinkInput{req})
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	count, err := h.Q.CountArticleLinksForScope(r.Context(), db.CountArticleLinksForScopeParams{
		GroupID: claims.GroupID, ScopeKey: scope,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to read links")
		return
	}
	if count >= maxArticleLinks {
		// Re-saving an existing URL is an edit, not a new row, so it stays allowed.
		existing, err := h.Q.ListArticleLinksForScope(r.Context(), db.ListArticleLinksForScopeParams{
			GroupID: claims.GroupID, ScopeKey: scope,
		})
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to read links")
			return
		}
		exists := false
		for _, link := range existing {
			if link.Url == links[0].URL {
				exists = true
				break
			}
		}
		if !exists {
			WriteError(w, http.StatusBadRequest, "too many links")
			return
		}
	}
	link, err := h.Q.UpsertArticleLink(r.Context(), db.UpsertArticleLinkParams{
		GroupID:    claims.GroupID,
		ScopeKey:   scope,
		Label:      links[0].Label,
		Url:        links[0].URL,
		Visibility: links[0].Visibility,
		CreatedBy:  claims.MemberID,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to save link")
		return
	}
	WriteJSON(w, http.StatusCreated, toEditArticleLinkView(link))
}

func (h *ArticleHandler) UpdateLink(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	scope, ok := h.articleScopeKey(w, r)
	if !ok {
		return
	}
	linkID, err := parseUUID(chi.URLParam(r, "linkId"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid link id")
		return
	}
	var req ArticleLinkInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	links, err := normalizeArticleLinks([]ArticleLinkInput{req})
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	existing, err := h.Q.ListArticleLinksForScope(r.Context(), db.ListArticleLinksForScopeParams{
		GroupID: claims.GroupID, ScopeKey: scope,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to read links")
		return
	}
	var current *db.ArticleLink
	for i := range existing {
		if existing[i].ID == linkID {
			current = &existing[i]
			break
		}
	}
	if current == nil {
		WriteError(w, http.StatusNotFound, "link not found")
		return
	}
	if current.Url != links[0].URL {
		for _, other := range existing {
			if other.ID != linkID && other.Url == links[0].URL {
				WriteError(w, http.StatusConflict, "a link with that url already exists")
				return
			}
		}
	}

	tx, err := h.Pool.Begin(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to update link")
		return
	}
	defer tx.Rollback(r.Context())
	q := h.Q.WithTx(tx)

	if current.Url != links[0].URL {
		if _, err := q.DeleteArticleLink(r.Context(), db.DeleteArticleLinkParams{
			ID: linkID, GroupID: claims.GroupID, ScopeKey: scope,
		}); err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to update link")
			return
		}
	}
	link, err := q.UpsertArticleLink(r.Context(), db.UpsertArticleLinkParams{
		GroupID:    claims.GroupID,
		ScopeKey:   scope,
		Label:      links[0].Label,
		Url:        links[0].URL,
		Visibility: links[0].Visibility,
		CreatedBy:  current.CreatedBy,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to update link")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to update link")
		return
	}
	WriteJSON(w, http.StatusOK, toEditArticleLinkView(link))
}

func (h *ArticleHandler) DeleteLink(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	scope, ok := h.articleScopeKey(w, r)
	if !ok {
		return
	}
	linkID, err := parseUUID(chi.URLParam(r, "linkId"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid link id")
		return
	}
	affected, err := h.Q.DeleteArticleLink(r.Context(), db.DeleteArticleLinkParams{
		ID: linkID, GroupID: claims.GroupID, ScopeKey: scope,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to delete link")
		return
	}
	if affected == 0 {
		WriteError(w, http.StatusNotFound, "link not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// brokenArticleLinkView is what managers see in group settings.
type brokenArticleLinkView struct {
	ID             string `json:"id"`
	Label          string `json:"label"`
	URL            string `json:"url"`
	Visibility     string `json:"visibility"`
	ArticleName    string `json:"article_name"`
	LastHTTPStatus *int32 `json:"last_http_status"`
	LastError      string `json:"last_error"`
	FailureKind    string `json:"failure_kind"`
	LastCheckedAt  any    `json:"last_checked_at"`
	Acknowledged   bool   `json:"acknowledged"`
}
