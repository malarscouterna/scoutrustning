package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/malarscouterna/ms-utrustning/api/internal/auth"
	"github.com/malarscouterna/ms-utrustning/api/internal/db"
	"github.com/malarscouterna/ms-utrustning/api/internal/i18n"
	"github.com/malarscouterna/ms-utrustning/api/internal/notifications"
)

type IssueHandler struct {
	Q        *db.Queries
	Perms    *PermissionCache
	Notifier notifications.Notifier
	BaseURL  string
}

func (h *IssueHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/{id}", h.Get)
	r.Put("/{id}", h.Update)
	r.Post("/{id}/comments", h.AddComment)
	r.Put("/{id}/assignees", h.ReplaceAssignees)
	r.Post("/{id}/assignees", h.AddAssignee)
	r.Delete("/{id}/assignees/{userId}", h.RemoveAssignee)
	r.Post("/{id}/articles", h.AddArticle)
	r.Delete("/{id}/articles/{articleId}", h.RemoveArticle)
	return r
}

func (h *IssueHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	var statuses []string
	if v := r.URL.Query().Get("status"); v != "" {
		statuses = strings.Split(v, ",")
	}
	mine := r.URL.Query().Get("mine") == "true"
	var articleID pgtype.UUID
	if v := r.URL.Query().Get("article_id"); v != "" {
		id, err := parseUUID(v)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid article_id")
			return
		}
		articleID = id
	}

	issues, err := h.Q.ListIssues(r.Context(), db.ListIssuesParams{
		GroupID:   claims.GroupID,
		Statuses:  statuses,
		Mine:      mine,
		UserID:    claims.MemberID,
		ArticleID: articleID,
	})
	if err != nil {
		slog.Error("failed to list issues", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list issues")
		return
	}

	type issueWithArticles struct {
		db.ListIssuesRow
		Articles []db.ListIssueArticlesRow `json:"articles"`
	}

	result := make([]issueWithArticles, 0, len(issues))
	for _, issue := range issues {
		articles, _ := h.Q.ListIssueArticles(r.Context(), db.ListIssueArticlesParams{
			IssueID: issue.ID, GroupID: claims.GroupID,
		})
		if articles == nil {
			articles = []db.ListIssueArticlesRow{}
		}
		result = append(result, issueWithArticles{issue, articles})
	}

	WriteJSON(w, http.StatusOK, result)
}

func (h *IssueHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	var req struct {
		ArticleID   string   `json:"article_id"`
		Severity    string   `json:"severity"`
		Description string   `json:"description"`
		BookingID   *string  `json:"booking_id"`
		ImageIds    []string `json:"image_ids"`
		Count       int      `json:"count"` // for quantity-tracked articles: number of units affected (default 1)
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ArticleID == "" {
		WriteError(w, http.StatusBadRequest, "article_id required")
		return
	}
	validSeverities := map[string]bool{"usable": true, "unusable": true, "missing": true}
	if !validSeverities[req.Severity] {
		WriteError(w, http.StatusBadRequest, "severity must be usable, unusable, or missing")
		return
	}
	if req.Description == "" {
		WriteError(w, http.StatusBadRequest, "description required")
		return
	}
	if req.Count < 1 {
		req.Count = 1
	}

	articleID, err := parseUUID(req.ArticleID)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid article_id")
		return
	}

	article, err := h.Q.GetArticle(r.Context(), db.GetArticleParams{ID: articleID, GroupID: claims.GroupID})
	if err != nil {
		WriteError(w, http.StatusNotFound, "article not found")
		return
	}

	lang := "sv"
	if c, err := r.Cookie("paraglide_lang"); err == nil && i18n.Supported(c.Value) {
		lang = c.Value
	}
	name := article.CommercialName
	if name == "" {
		name = article.CommonName
	}
	title := i18n.T(lang, "issue_title", map[string]string{
		"commercialName": name,
		"severity":       i18n.T(lang, "issue_severity_"+req.Severity),
	})

	var bookingID pgtype.UUID
	if req.BookingID != nil && *req.BookingID != "" {
		bookingID, err = parseUUID(*req.BookingID)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid booking_id")
			return
		}
	}

	issue, err := h.Q.CreateIssue(r.Context(), db.CreateIssueParams{
		GroupID:     claims.GroupID,
		Title:       title,
		Description: req.Description,
		Severity:    req.Severity,
		ReporterID:  claims.MemberID,
		BookingID:   bookingID,
	})
	if err != nil {
		slog.Error("failed to create issue", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to create issue")
		return
	}

	// Collect article IDs to link. For quantity-tracked articles, link `count` units from the same group.
	articleIDs := []pgtype.UUID{articleID}
	if !article.IndividuallyTracked && req.Count > 1 {
		groupIDs, _ := h.Q.ListArticleIDsInGroup(r.Context(), db.ListArticleIDsInGroupParams{
			GroupID:        claims.GroupID,
			CommercialName: article.CommercialName,
			LocationID:     article.LocationID,
		})
		// Build set starting with the primary article, then fill up to count from group
		seen := map[pgtype.UUID]bool{articleID: true}
		for _, gid := range groupIDs {
			if len(articleIDs) >= req.Count {
				break
			}
			if !seen[gid] {
				articleIDs = append(articleIDs, gid)
				seen[gid] = true
			}
		}
	}

	for _, aid := range articleIDs {
		if err := h.Q.AddIssueArticle(r.Context(), db.AddIssueArticleParams{
			IssueID:   issue.ID,
			ArticleID: aid,
			GroupID:   claims.GroupID,
		}); err != nil {
			slog.Error("failed to link issue article", "error", err)
			WriteError(w, http.StatusInternalServerError, "failed to create issue")
			return
		}
	}

	// Log creation event
	meta, _ := json.Marshal(map[string]any{
		"severity":   req.Severity,
		"image_ids":  req.ImageIds,
		"article_id": req.ArticleID,
	})
	h.Q.CreateIssueEvent(r.Context(), db.CreateIssueEventParams{
		IssueID:     issue.ID,
		GroupID:     claims.GroupID,
		ActorID:     claims.MemberID,
		EventType:   "comment",
		Description: req.Description,
		Metadata:    meta,
	})

	// Update article status for all linked articles
	for _, aid := range articleIDs {
		linkedArticle, err := h.Q.GetArticle(r.Context(), db.GetArticleParams{ID: aid, GroupID: claims.GroupID})
		if err == nil {
			h.deriveAndApplyArticleStatus(r, claims, aid, linkedArticle.Status, req.Severity)
		}
	}

	// Touch updated_at on the issue
	h.Q.UpdateIssue(r.Context(), db.UpdateIssueParams{
		ID:      issue.ID,
		GroupID: claims.GroupID,
	})

	if h.Notifier != nil {
		iss, n, q := issue, h.Notifier, h.Q
		go notifications.SendIssueCreated(context.Background(), q, n, iss, h.BaseURL)
	}

	issueDetail, _ := h.buildIssueDetail(r, claims, issue.ID)
	WriteJSON(w, http.StatusCreated, issueDetail)
}

func (h *IssueHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}

	detail, err := h.buildIssueDetail(r, claims, id)
	if err != nil {
		WriteError(w, http.StatusNotFound, "issue not found")
		return
	}
	WriteJSON(w, http.StatusOK, detail)
}

func (h *IssueHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
		Status      *string `json:"status"`
		Comment     string  `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Status changes require issue_resolve_role
	if req.Status != nil {
		perms := h.Perms.Get(r, claims.GroupID)
		if !auth.AccessAtLeast(claims.MaxAccess, perms.IssueResolve) {
			WriteError(w, http.StatusForbidden, "forbidden")
			return
		}
		validStatuses := map[string]bool{"open": true, "in_progress": true, "resolved": true, "archived": true}
		if !validStatuses[*req.Status] {
			WriteError(w, http.StatusBadRequest, "invalid status")
			return
		}
	}

	existing, err := h.Q.GetIssue(r.Context(), db.GetIssueParams{ID: id, GroupID: claims.GroupID})
	if err != nil {
		WriteError(w, http.StatusNotFound, "issue not found")
		return
	}

	params := db.UpdateIssueParams{ID: id, GroupID: claims.GroupID}
	if req.Title != nil {
		params.Title = pgtype.Text{String: *req.Title, Valid: true}
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}
	if req.Status != nil {
		params.Status = pgtype.Text{String: *req.Status, Valid: true}
	}

	if _, err := h.Q.UpdateIssue(r.Context(), params); err != nil {
		slog.Error("failed to update issue", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to update issue")
		return
	}

	// Log status change event if status changed
	if req.Status != nil && *req.Status != existing.Status {
		meta, _ := json.Marshal(map[string]string{
			"old_status": existing.Status,
			"new_status": *req.Status,
		})
		h.Q.CreateIssueEvent(r.Context(), db.CreateIssueEventParams{
			IssueID:     id,
			GroupID:     claims.GroupID,
			ActorID:     claims.MemberID,
			EventType:   "status_change",
			Description: req.Comment,
			Metadata:    meta,
		})

		// Re-derive article statuses if resolved or archived
		if *req.Status == "resolved" || *req.Status == "archived" {
			articles, _ := h.Q.ListOpenIssueArticles(r.Context(), db.ListOpenIssueArticlesParams{
				IssueID: id, GroupID: claims.GroupID,
			})
			for _, artID := range articles {
				h.reapplyDerivedStatus(r, claims, artID)
			}
		}

		if h.Notifier != nil && *req.Status == "resolved" {
			if iss, err := h.Q.GetIssue(r.Context(), db.GetIssueParams{ID: id, GroupID: claims.GroupID}); err == nil {
				issue := db.IssueReport{
					ID: iss.ID, GroupID: iss.GroupID, Title: iss.Title,
					ReporterID: iss.ReporterID, Severity: iss.Severity, Status: iss.Status,
					Description: iss.Description, BookingID: iss.BookingID,
					CreatedAt: iss.CreatedAt, UpdatedAt: iss.UpdatedAt,
				}
				n, q := h.Notifier, h.Q
				go notifications.SendIssueResolved(context.Background(), q, n, issue, h.BaseURL)
			}
		}
	}

	detail, _ := h.buildIssueDetail(r, claims, id)
	WriteJSON(w, http.StatusOK, detail)
}

func (h *IssueHandler) AddComment(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req struct {
		Description string   `json:"description"`
		ImageIds    []string `json:"image_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Description == "" {
		WriteError(w, http.StatusBadRequest, "description required")
		return
	}

	if _, err := h.Q.GetIssue(r.Context(), db.GetIssueParams{ID: id, GroupID: claims.GroupID}); err != nil {
		WriteError(w, http.StatusNotFound, "issue not found")
		return
	}

	meta, _ := json.Marshal(map[string]any{"image_ids": req.ImageIds})
	if _, err := h.Q.CreateIssueEvent(r.Context(), db.CreateIssueEventParams{
		IssueID:     id,
		GroupID:     claims.GroupID,
		ActorID:     claims.MemberID,
		EventType:   "comment",
		Description: req.Description,
		Metadata:    meta,
	}); err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to add comment")
		return
	}

	// Touch updated_at
	h.Q.UpdateIssue(r.Context(), db.UpdateIssueParams{ID: id, GroupID: claims.GroupID})

	if h.Notifier != nil {
		if iss, err := h.Q.GetIssue(r.Context(), db.GetIssueParams{ID: id, GroupID: claims.GroupID}); err == nil {
			issue := db.IssueReport{
				ID: iss.ID, GroupID: iss.GroupID, Title: iss.Title,
				ReporterID: iss.ReporterID, Severity: iss.Severity, Status: iss.Status,
				Description: iss.Description, BookingID: iss.BookingID,
				CreatedAt: iss.CreatedAt, UpdatedAt: iss.UpdatedAt,
			}
			n, q := h.Notifier, h.Q
			go notifications.SendIssueCommented(context.Background(), q, n, issue, h.BaseURL)
		}
	}

	detail, _ := h.buildIssueDetail(r, claims, id)
	WriteJSON(w, http.StatusCreated, detail)
}

func (h *IssueHandler) ReplaceAssignees(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}

	perms := h.Perms.Get(r, claims.GroupID)
	if !auth.AccessAtLeast(claims.MaxAccess, perms.IssueResolve) {
		WriteError(w, http.StatusForbidden, "forbidden")
		return
	}

	var req struct {
		UserIDs []string `json:"user_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if _, err := h.Q.GetIssue(r.Context(), db.GetIssueParams{ID: id, GroupID: claims.GroupID}); err != nil {
		WriteError(w, http.StatusNotFound, "issue not found")
		return
	}

	if err := h.Q.ReplaceIssueAssignees(r.Context(), db.ReplaceIssueAssigneesParams{
		IssueID: id, GroupID: claims.GroupID,
	}); err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to update assignees")
		return
	}

	for _, userID := range req.UserIDs {
		h.Q.AddIssueAssignee(r.Context(), db.AddIssueAssigneeParams{
			IssueID: id, UserID: userID, GroupID: claims.GroupID,
		})
	}

	// Log assignment event
	meta, _ := json.Marshal(map[string]any{"user_ids": req.UserIDs})
	h.Q.CreateIssueEvent(r.Context(), db.CreateIssueEventParams{
		IssueID:   id,
		GroupID:   claims.GroupID,
		ActorID:   claims.MemberID,
		EventType: "assignment",
		Metadata:  meta,
	})

	h.Q.UpdateIssue(r.Context(), db.UpdateIssueParams{ID: id, GroupID: claims.GroupID})

	assignees, _ := h.Q.ListIssueAssignees(r.Context(), db.ListIssueAssigneesParams{
		IssueID: id, GroupID: claims.GroupID,
	})
	if assignees == nil {
		assignees = []db.ListIssueAssigneesRow{}
	}
	WriteJSON(w, http.StatusOK, assignees)
}

func (h *IssueHandler) AddAssignee(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if !claims.IsManager() {
		WriteError(w, http.StatusForbidden, "forbidden")
		return
	}

	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
		WriteError(w, http.StatusBadRequest, "user_id required")
		return
	}

	if _, err := h.Q.GetIssue(r.Context(), db.GetIssueParams{ID: id, GroupID: claims.GroupID}); err != nil {
		WriteError(w, http.StatusNotFound, "issue not found")
		return
	}

	if err := h.Q.InsertIssueAssignee(r.Context(), db.InsertIssueAssigneeParams{
		IssueID: id, UserID: req.UserID, GroupID: claims.GroupID,
	}); err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			WriteError(w, http.StatusConflict, "already_assigned")
			return
		}
		WriteError(w, http.StatusInternalServerError, "failed to add assignee")
		return
	}

	assigneeName := req.UserID
	if u, err := h.Q.GetUser(r.Context(), db.GetUserParams{ID: req.UserID, GroupID: claims.GroupID}); err == nil {
		assigneeName = u.Name
	}
	meta, _ := json.Marshal(map[string]string{"user_id": req.UserID, "user_name": assigneeName, "action": "added"})
	h.Q.CreateIssueEvent(r.Context(), db.CreateIssueEventParams{
		IssueID:   id,
		GroupID:   claims.GroupID,
		ActorID:   claims.MemberID,
		EventType: "assignment",
		Metadata:  meta,
	})
	h.Q.UpdateIssue(r.Context(), db.UpdateIssueParams{ID: id, GroupID: claims.GroupID})

	if h.Notifier != nil {
		if iss, err := h.Q.GetIssue(r.Context(), db.GetIssueParams{ID: id, GroupID: claims.GroupID}); err == nil {
			issue := db.IssueReport{ID: iss.ID, GroupID: iss.GroupID, Title: iss.Title, ReporterID: iss.ReporterID}
			assigneeID, n, q := req.UserID, h.Notifier, h.Q
			go notifications.SendIssueAssignedToMe(context.Background(), q, n, issue, assigneeID, h.BaseURL)
		}
	}

	detail, _ := h.buildIssueDetail(r, claims, id)
	WriteJSON(w, http.StatusCreated, detail)
}

func (h *IssueHandler) RemoveAssignee(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if !claims.IsManager() {
		WriteError(w, http.StatusForbidden, "forbidden")
		return
	}

	userID := chi.URLParam(r, "userId")

	if _, err := h.Q.GetIssue(r.Context(), db.GetIssueParams{ID: id, GroupID: claims.GroupID}); err != nil {
		WriteError(w, http.StatusNotFound, "issue not found")
		return
	}

	if err := h.Q.DeleteIssueAssignee(r.Context(), db.DeleteIssueAssigneeParams{
		IssueID: id, UserID: userID, GroupID: claims.GroupID,
	}); err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to remove assignee")
		return
	}

	removedName := userID
	if u, err := h.Q.GetUser(r.Context(), db.GetUserParams{ID: userID, GroupID: claims.GroupID}); err == nil {
		removedName = u.Name
	}
	meta, _ := json.Marshal(map[string]string{"user_id": userID, "user_name": removedName, "action": "removed"})
	h.Q.CreateIssueEvent(r.Context(), db.CreateIssueEventParams{
		IssueID:   id,
		GroupID:   claims.GroupID,
		ActorID:   claims.MemberID,
		EventType: "assignment",
		Metadata:  meta,
	})
	h.Q.UpdateIssue(r.Context(), db.UpdateIssueParams{ID: id, GroupID: claims.GroupID})

	w.WriteHeader(http.StatusNoContent)
}

func (h *IssueHandler) AddArticle(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}

	perms := h.Perms.Get(r, claims.GroupID)
	if !auth.AccessAtLeast(claims.MaxAccess, perms.IssueResolve) {
		WriteError(w, http.StatusForbidden, "forbidden")
		return
	}

	var req struct {
		ArticleID string `json:"article_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ArticleID == "" {
		WriteError(w, http.StatusBadRequest, "article_id required")
		return
	}

	articleID, err := parseUUID(req.ArticleID)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid article_id")
		return
	}

	issue, err := h.Q.GetIssue(r.Context(), db.GetIssueParams{ID: id, GroupID: claims.GroupID})
	if err != nil {
		WriteError(w, http.StatusNotFound, "issue not found")
		return
	}

	if _, err := h.Q.GetArticle(r.Context(), db.GetArticleParams{ID: articleID, GroupID: claims.GroupID}); err != nil {
		WriteError(w, http.StatusNotFound, "article not found")
		return
	}

	h.Q.AddIssueArticle(r.Context(), db.AddIssueArticleParams{
		IssueID: id, ArticleID: articleID, GroupID: claims.GroupID,
	})

	// Update article status if issue is active
	if issue.Status == "open" || issue.Status == "in_progress" {
		existingStatus, _ := h.Q.GetArticleCurrentStatus(r.Context(), db.GetArticleCurrentStatusParams{
			ID: articleID, GroupID: claims.GroupID,
		})
		h.deriveAndApplyArticleStatus(r, claims, articleID, existingStatus, issue.Severity)
	}

	meta, _ := json.Marshal(map[string]string{"article_id": req.ArticleID})
	h.Q.CreateIssueEvent(r.Context(), db.CreateIssueEventParams{
		IssueID:   id,
		GroupID:   claims.GroupID,
		ActorID:   claims.MemberID,
		EventType: "article_added",
		Metadata:  meta,
	})
	h.Q.UpdateIssue(r.Context(), db.UpdateIssueParams{ID: id, GroupID: claims.GroupID})

	w.WriteHeader(http.StatusNoContent)
}

func (h *IssueHandler) RemoveArticle(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}
	articleID, err := parseUUID(chi.URLParam(r, "articleId"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid article id")
		return
	}

	perms := h.Perms.Get(r, claims.GroupID)
	if !auth.AccessAtLeast(claims.MaxAccess, perms.IssueResolve) {
		WriteError(w, http.StatusForbidden, "forbidden")
		return
	}

	if _, err := h.Q.GetIssue(r.Context(), db.GetIssueParams{ID: id, GroupID: claims.GroupID}); err != nil {
		WriteError(w, http.StatusNotFound, "issue not found")
		return
	}

	h.Q.RemoveIssueArticle(r.Context(), db.RemoveIssueArticleParams{
		IssueID: id, ArticleID: articleID, GroupID: claims.GroupID,
	})

	// Re-derive the article status now that this issue no longer links it
	h.reapplyDerivedStatus(r, claims, articleID)

	meta, _ := json.Marshal(map[string]string{"article_id": formatUUID(articleID)})
	h.Q.CreateIssueEvent(r.Context(), db.CreateIssueEventParams{
		IssueID:   id,
		GroupID:   claims.GroupID,
		ActorID:   claims.MemberID,
		EventType: "article_removed",
		Metadata:  meta,
	})
	h.Q.UpdateIssue(r.Context(), db.UpdateIssueParams{ID: id, GroupID: claims.GroupID})

	w.WriteHeader(http.StatusNoContent)
}

// buildIssueDetail assembles the full issue detail response.
func (h *IssueHandler) buildIssueDetail(r *http.Request, claims auth.Claims, id pgtype.UUID) (any, error) {
	issue, err := h.Q.GetIssue(r.Context(), db.GetIssueParams{ID: id, GroupID: claims.GroupID})
	if err != nil {
		return nil, err
	}

	articles, _ := h.Q.ListIssueArticles(r.Context(), db.ListIssueArticlesParams{
		IssueID: id, GroupID: claims.GroupID,
	})
	if articles == nil {
		articles = []db.ListIssueArticlesRow{}
	}

	assignees, _ := h.Q.ListIssueAssignees(r.Context(), db.ListIssueAssigneesParams{
		IssueID: id, GroupID: claims.GroupID,
	})
	if assignees == nil {
		assignees = []db.ListIssueAssigneesRow{}
	}

	events, _ := h.Q.ListIssueEvents(r.Context(), db.ListIssueEventsParams{
		IssueID: id, GroupID: claims.GroupID,
	})
	if events == nil {
		events = []db.ListIssueEventsRow{}
	}

	return map[string]any{
		"id":          formatUUID(issue.ID),
		"title":       issue.Title,
		"description": issue.Description,
		"severity":    issue.Severity,
		"status":      issue.Status,
		"reporter": map[string]string{
			"id":   issue.ReporterID,
			"name": issue.ReporterName,
		},
		"booking_id": func() any {
			if issue.BookingID.Valid {
				return formatUUID(issue.BookingID)
			}
			return nil
		}(),
		"articles":   articles,
		"assignees":  assignees,
		"events":     events,
		"created_at": issue.CreatedAt,
		"updated_at": issue.UpdatedAt,
	}, nil
}

// deriveAndApplyArticleStatus sets article status to reported_{severity} if it is worse
// than the current status (missing = unusable > usable > ok).
func (h *IssueHandler) deriveAndApplyArticleStatus(r *http.Request, claims auth.Claims, articleID pgtype.UUID, currentStatus, newSeverity string) {
	severityRank := map[string]int{"usable": 1, "unusable": 2, "missing": 2}
	statusRank := map[string]int{"ok": 0, "reported_usable": 1, "reported_unusable": 2, "reported_missing": 2}

	newStatus := "reported_" + newSeverity
	if severityRank[newSeverity] >= statusRank[currentStatus] {
		h.Q.UpdateArticleStatusDirect(r.Context(), db.UpdateArticleStatusDirectParams{
			ID: articleID, GroupID: claims.GroupID, Status: newStatus,
		})
	}
}

// reapplyDerivedStatus re-derives an article's status from remaining open issues.
func (h *IssueHandler) reapplyDerivedStatus(r *http.Request, claims auth.Claims, articleID pgtype.UUID) {
	row, err := h.Q.DeriveArticleStatus(r.Context(), db.DeriveArticleStatusParams{
		ArticleID: articleID, GroupID: claims.GroupID,
	})
	if err != nil {
		slog.Error("failed to derive article status", "error", err)
		return
	}

	// Only update if current status is a reported status (don't overwrite under_repair, archived, etc.)
	current, err := h.Q.GetArticleCurrentStatus(r.Context(), db.GetArticleCurrentStatusParams{
		ID: articleID, GroupID: claims.GroupID,
	})
	if err != nil {
		return
	}
	if strings.HasPrefix(current, "reported_") || current == "ok" {
		h.Q.UpdateArticleStatusDirect(r.Context(), db.UpdateArticleStatusDirectParams{
			ID: articleID, GroupID: claims.GroupID, Status: row,
		})
	}
}
