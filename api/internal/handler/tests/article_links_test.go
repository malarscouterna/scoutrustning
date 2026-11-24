package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/scoutrustning/api/internal/handler"
	"github.com/malarscouterna/scoutrustning/api/internal/linkcheck"
	"github.com/malarscouterna/scoutrustning/api/internal/testutil"
)

func setupArticleLinkEnv(t *testing.T) *testutil.TestEnv {
	t.Helper()
	env := testutil.SetupTestEnv(t)
	perms := handler.NewPermissionCache(env.Queries)
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries, Pool: env.Pool, Perms: perms}).Routes())
		r.Mount("/group-settings", (&handler.GroupSettingsHandler{
			Q: env.Queries, Pool: env.Pool, Perms: perms, Links: linkcheck.NewForTest(),
		}).Routes())
	})
	return env
}

func linkTestIDs(t *testing.T, env *testutil.TestEnv) (locationID, categoryID string) {
	t.Helper()
	if err := env.Pool.QueryRow(context.Background(),
		`SELECT id::text FROM locations WHERE group_id = '766' LIMIT 1`).Scan(&locationID); err != nil {
		t.Fatalf("location: %v", err)
	}
	if err := env.Pool.QueryRow(context.Background(),
		`SELECT id::text FROM categories WHERE group_id = '766' LIMIT 1`).Scan(&categoryID); err != nil {
		t.Fatalf("category: %v", err)
	}
	return locationID, categoryID
}

func articleLinkPayload(t *testing.T, env *testutil.TestEnv, commercial, common string, links []map[string]any) map[string]any {
	t.Helper()
	locationID, categoryID := linkTestIDs(t, env)
	return map[string]any{
		"commercial_name":      commercial,
		"common_name":          common,
		"category_id":          categoryID,
		"location_id":          locationID,
		"status":               "ok",
		"individually_tracked": true,
		"approval_level":       "none",
		"links":                links,
	}
}

func createArticleWithLinks(t *testing.T, client *testutil.TestClient, env *testutil.TestEnv, commercial, common string, links []map[string]any) string {
	t.Helper()
	body, _ := json.Marshal(articleLinkPayload(t, env, commercial, common, links))
	resp, err := client.Post("/api/v0/articles", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create article: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create article: expected 201, got %d", resp.StatusCode)
	}
	var article map[string]any
	json.NewDecoder(resp.Body).Decode(&article)
	return article["id"].(string)
}

func articleLinks(t *testing.T, client *testutil.TestClient, articleID string) []map[string]any {
	t.Helper()
	resp, err := client.Get("/api/v0/articles/" + articleID)
	if err != nil {
		t.Fatalf("get article: %v", err)
	}
	defer resp.Body.Close()
	var article map[string]any
	json.NewDecoder(resp.Body).Decode(&article)
	raw, _ := article["links"].([]any)
	links := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		links = append(links, item.(map[string]any))
	}
	return links
}

func TestArticleLinksVisibilityAndCRUD(t *testing.T) {
	env := setupArticleLinkEnv(t)
	manager := env.ClientAs("manager-equipment")
	book := env.ClientAs("leader-yggdrasil")
	trusted := env.ClientAs("project-unit-leader")

	id := createArticleWithLinks(t, manager, env, "Kanon", "Kanon 1", []map[string]any{
		{"label": "Användarinstruktion", "url": "https://example.com/manual", "visibility": "public"},
		{"label": "Reparationsmanual", "url": "https://example.com/repair", "visibility": "trusted"},
	})

	t.Run("visibility filtering", func(t *testing.T) {
		if got := len(articleLinks(t, manager, id)); got != 2 {
			t.Errorf("manager: expected 2 links, got %d", got)
		}
		if got := len(articleLinks(t, trusted, id)); got != 2 {
			t.Errorf("trusted: expected 2 links, got %d", got)
		}
		bookLinks := articleLinks(t, book, id)
		if len(bookLinks) != 1 || bookLinks[0]["visibility"] != "public" {
			t.Errorf("book: expected only the public link, got %v", bookLinks)
		}
	})

	t.Run("link editing requires article edit permission", func(t *testing.T) {
		resp, _ := book.Get("/api/v0/articles/" + id + "/links")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("book-level GET /links: expected 403, got %d", resp.StatusCode)
		}

		body, _ := json.Marshal(map[string]any{"label": "X", "url": "https://example.com/x", "visibility": "public"})
		resp, _ = book.Post("/api/v0/articles/"+id+"/links", bytes.NewReader(body))
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("book-level POST /links: expected 403, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("manager can list, update and delete links", func(t *testing.T) {
		resp, _ := manager.Get("/api/v0/articles/" + id + "/links")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var links []map[string]any
		json.NewDecoder(resp.Body).Decode(&links)
		if len(links) != 2 {
			t.Fatalf("expected 2 links, got %d", len(links))
		}

		target := links[0]
		body, _ := json.Marshal(map[string]any{
			"label": "Uppdaterad instruktion", "url": target["url"], "visibility": "public",
		})
		resp, _ = manager.Put("/api/v0/articles/"+id+"/links/"+target["id"].(string), bytes.NewReader(body))
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("update link: expected 200, got %d", resp.StatusCode)
		}
		resp.Body.Close()

		var updated map[string]any
		body, _ = json.Marshal(map[string]any{"label": "X", "url": "https://example.com/other", "visibility": "public"})
		resp, _ = manager.Post("/api/v0/articles/"+id+"/links", bytes.NewReader(body))
		json.NewDecoder(resp.Body).Decode(&updated)
		resp.Body.Close()

		resp, _ = manager.Delete("/api/v0/articles/" + id + "/links/" + updated["id"].(string))
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("delete link: expected 204, got %d", resp.StatusCode)
		}
		resp.Body.Close()

		links = articleLinks(t, manager, id)
		if len(links) != 2 {
			t.Errorf("expected 2 links after add+delete, got %d", len(links))
		}
	})

	t.Run("renaming the product type moves links", func(t *testing.T) {
		payload := articleLinkPayload(t, env, "Kanon Renamed", "Kanon 1", nil)
		body, _ := json.Marshal(payload)
		resp, _ := manager.Put("/api/v0/articles/"+id, bytes.NewReader(body))
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("rename: expected 200, got %d", resp.StatusCode)
		}
		resp.Body.Close()

		if got := len(articleLinks(t, manager, id)); got != 2 {
			t.Errorf("expected links to follow the rename, got %d", got)
		}
	})

	t.Run("deleting the last copy removes links", func(t *testing.T) {
		resp, _ := manager.Delete("/api/v0/articles/" + id)
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("delete article: expected 204, got %d", resp.StatusCode)
		}
		resp.Body.Close()

		var count int
		if err := env.Pool.QueryRow(context.Background(),
			`SELECT count(*) FROM article_links WHERE group_id = '766'`).Scan(&count); err != nil {
			t.Fatalf("count links: %v", err)
		}
		if count != 0 {
			t.Errorf("expected no orphaned links, got %d", count)
		}
	})
}

func TestArticleLinksRespectConfiguredEditRole(t *testing.T) {
	env := setupArticleLinkEnv(t)
	if _, err := env.Pool.Exec(context.Background(),
		`UPDATE group_settings SET article_edit_role = 'book' WHERE group_id = '766'`); err != nil {
		t.Fatalf("update settings: %v", err)
	}

	manager := env.ClientAs("manager-equipment")
	book := env.ClientAs("leader-yggdrasil")
	viewer := env.ClientAs("view-only")

	id := createArticleWithLinks(t, manager, env, "Tält", "Tält 1", nil)

	resp, _ := book.Get("/api/v0/articles/" + id + "/links")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("book-level editor should list links, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	body, _ := json.Marshal(map[string]any{
		"label": "Packinstruktion", "url": "https://example.com/pack", "visibility": "public",
	})
	resp, _ = book.Post("/api/v0/articles/"+id+"/links", bytes.NewReader(body))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("book-level editor should add links, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp, _ = viewer.Get("/api/v0/articles/" + id + "/links")
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("view-only should not list links, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestArticleLinkCheckerAndBrokenList(t *testing.T) {
	env := setupArticleLinkEnv(t)
	manager := env.ClientAs("manager-equipment")
	book := env.ClientAs("leader-yggdrasil")

	var flakyMode atomic.Int32 // 0 = 200, 1 = 404, 2 = 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			w.WriteHeader(http.StatusOK)
			return
		}
		switch flakyMode.Load() {
		case 0:
			w.WriteHeader(http.StatusOK)
		case 1:
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	id := createArticleWithLinks(t, manager, env, "Stormkök", "Stormkök 1", []map[string]any{
		{"label": "Användarinstruktion", "url": server.URL + "/ok", "visibility": "public"},
		{"label": "Reparationsmanual", "url": server.URL + "/flaky", "visibility": "public"},
	})

	flakyMode.Store(1)
	stats, err := linkcheck.RunGroup(context.Background(), env.Queries, linkcheck.NewForTest(), "766", 100)
	if err != nil {
		t.Fatalf("check links: %v", err)
	}
	if stats.Checked != 2 || stats.Broken != 1 {
		t.Fatalf("expected 2 checked / 1 broken, got %+v", stats)
	}

	broken := func() []map[string]any {
		resp, _ := manager.Get("/api/v0/group-settings/broken-links")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("broken links: expected 200, got %d", resp.StatusCode)
		}
		var rows []map[string]any
		json.NewDecoder(resp.Body).Decode(&rows)
		return rows
	}

	rows := broken()
	if len(rows) != 1 {
		t.Fatalf("expected 1 broken link, got %d", len(rows))
	}
	if rows[0]["article_name"] != "Stormkök" || rows[0]["failure_kind"] != "http:404" {
		t.Errorf("unexpected broken row: %v", rows[0])
	}
	linkID := rows[0]["id"].(string)

	t.Run("non-managers cannot read the broken list", func(t *testing.T) {
		resp, _ := book.Get("/api/v0/group-settings/broken-links")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("approval marks the failure instead of hiding it", func(t *testing.T) {
		resp, _ := manager.Post("/api/v0/group-settings/broken-links/"+linkID+"/ack", nil)
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("ack: expected 204, got %d", resp.StatusCode)
		}
		resp.Body.Close()

		rows := broken()
		if len(rows) != 1 || rows[0]["acknowledged"] != true {
			t.Fatalf("acknowledged link should still be listed as acknowledged, got %v", rows)
		}

		// Undo makes it an active failure again.
		resp, _ = manager.Delete("/api/v0/group-settings/broken-links/" + linkID + "/ack")
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("unack: expected 204, got %d", resp.StatusCode)
		}
		resp.Body.Close()
		rows = broken()
		if len(rows) != 1 || rows[0]["acknowledged"] != false {
			t.Fatalf("unacknowledged link should be active again, got %v", rows)
		}

		// Approve again, then a different failure re-surfaces it as active.
		resp, _ = manager.Post("/api/v0/group-settings/broken-links/"+linkID+"/ack", nil)
		resp.Body.Close()
		flakyMode.Store(2)
		if _, err := linkcheck.RunGroup(context.Background(), env.Queries, linkcheck.NewForTest(), "766", 100); err != nil {
			t.Fatalf("check links: %v", err)
		}
		rows = broken()
		if len(rows) != 1 || rows[0]["failure_kind"] != "http:500" || rows[0]["acknowledged"] != false {
			t.Fatalf("expected the new failure to resurface as active, got %v", rows)
		}
	})

	t.Run("success clears status and approval", func(t *testing.T) {
		flakyMode.Store(0)
		resp, _ := manager.Post("/api/v0/group-settings/check-links", nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("check-links: expected 200, got %d", resp.StatusCode)
		}
		var stats map[string]any
		json.NewDecoder(resp.Body).Decode(&stats)
		resp.Body.Close()
		if int(stats["broken"].(float64)) != 0 {
			t.Errorf("expected no broken links, got %v", stats)
		}
		if int(stats["total"].(float64)) != 2 || stats["truncated"] != false {
			t.Errorf("expected 2 total and not truncated, got %v", stats)
		}
		if got := len(broken()); got != 0 {
			t.Errorf("expected empty broken list, got %d", got)
		}

		for _, link := range articleLinks(t, manager, id) {
			if _, exposed := link["last_status"]; exposed {
				t.Errorf("article responses should not expose checker internals: %v", link)
			}
		}
	})
}

func renameArticle(t *testing.T, client *testutil.TestClient, env *testutil.TestEnv, id, commercial, common string) int {
	t.Helper()
	body, _ := json.Marshal(articleLinkPayload(t, env, commercial, common, nil))
	resp, err := client.Put("/api/v0/articles/"+id, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("rename article: %v", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

func TestArticleLinkCheckCancellationDoesNotMarkBroken(t *testing.T) {
	env := setupArticleLinkEnv(t)
	manager := env.ClientAs("manager-equipment")

	var once sync.Once
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(started) })
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer close(release)

	createArticleWithLinks(t, manager, env, "Långsam", "Långsam 1", []map[string]any{
		{"label": "Guide", "url": server.URL + "/slow", "visibility": "public"},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	var stats linkcheck.RunStats
	var err error
	go func() {
		stats, err = linkcheck.RunGroup(ctx, env.Queries, linkcheck.NewForTest(), "766", 100)
		close(done)
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("checker never reached the server")
	}
	cancel()
	<-done

	if err != nil {
		t.Fatalf("check links: %v", err)
	}
	if stats.Checked != 0 {
		t.Errorf("cancelled run should not count checks, got %+v", stats)
	}
	var status string
	if err := env.Pool.QueryRow(context.Background(),
		`SELECT last_status FROM article_links WHERE scope_key = 'Långsam'`).Scan(&status); err != nil {
		t.Fatalf("read status: %v", err)
	}
	if status != "unchecked" {
		t.Errorf("cancelled run must not persist a status, got %q", status)
	}
}

func TestCreatingAnotherCopyKeepsSharedLinks(t *testing.T) {
	env := setupArticleLinkEnv(t)
	manager := env.ClientAs("manager-equipment")

	first := createArticleWithLinks(t, manager, env, "Tält", "Tält 1", []map[string]any{
		{"label": "Guide", "url": "https://example.com/", "visibility": "public"},
	})
	second := createArticleWithLinks(t, manager, env, "Tält", "Tält 2", []map[string]any{})
	if got := len(articleLinks(t, manager, first)); got != 1 {
		t.Errorf("empty links on a new copy must not wipe the set, got %d", got)
	}
	if got := len(articleLinks(t, manager, second)); got != 1 {
		t.Errorf("new copy should see the shared link, got %d", got)
	}

	createArticleWithLinks(t, manager, env, "Tält", "Tält 3", nil)
	if got := len(articleLinks(t, manager, first)); got != 1 {
		t.Errorf("omitted links must not wipe the set, got %d", got)
	}

	createArticleWithLinks(t, manager, env, "Tält", "Tält 4", []map[string]any{
		{"label": "Repair", "url": "https://example.org/", "visibility": "trusted"},
	})
	if got := len(articleLinks(t, manager, first)); got != 2 {
		t.Errorf("links supplied on create should be added, got %d", got)
	}
}

func TestArticleLinkCapOnCreateEndpoint(t *testing.T) {
	env := setupArticleLinkEnv(t)
	manager := env.ClientAs("manager-equipment")
	id := createArticleWithLinks(t, manager, env, "Kap", "Kap 1", nil)

	post := func(label, url string) int {
		body, _ := json.Marshal(map[string]any{"label": label, "url": url, "visibility": "public"})
		resp, err := manager.Post("/api/v0/articles/"+id+"/links", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("post link: %v", err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}

	for i := 0; i < 20; i++ {
		if code := post(fmt.Sprintf("Länk %d", i), fmt.Sprintf("https://example.com/%d", i)); code != http.StatusCreated {
			t.Fatalf("link %d: expected 201, got %d", i, code)
		}
	}
	if code := post("Över", "https://example.com/over"); code != http.StatusBadRequest {
		t.Errorf("21st link: expected 400, got %d", code)
	}
	if code := post("Länk 0", "https://example.com/0"); code != http.StatusCreated {
		t.Errorf("re-saving an existing url at the cap should be allowed, got %d", code)
	}
}

func TestArticleLinkUpdateURLCollision(t *testing.T) {
	env := setupArticleLinkEnv(t)
	manager := env.ClientAs("manager-equipment")
	id := createArticleWithLinks(t, manager, env, "Krock", "Krock 1", []map[string]any{
		{"label": "A", "url": "https://example.com/a", "visibility": "public"},
		{"label": "B", "url": "https://example.com/b", "visibility": "public"},
	})

	resp, _ := manager.Get("/api/v0/articles/" + id + "/links")
	var links []map[string]any
	json.NewDecoder(resp.Body).Decode(&links)
	resp.Body.Close()
	ids := map[string]string{}
	for _, link := range links {
		ids[link["label"].(string)] = link["id"].(string)
	}

	put := func(linkID, label, url string) int {
		body, _ := json.Marshal(map[string]any{"label": label, "url": url, "visibility": "public"})
		resp, err := manager.Put("/api/v0/articles/"+id+"/links/"+linkID, bytes.NewReader(body))
		if err != nil {
			t.Fatalf("put link: %v", err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}

	if code := put(ids["B"], "B", "https://example.com/a"); code != http.StatusConflict {
		t.Errorf("changing to an existing url should 409, got %d", code)
	}
	if code := put(ids["A"], "A2", "https://example.com/a2"); code != http.StatusOK {
		t.Errorf("changing to a new url should 200, got %d", code)
	}
	if got := len(articleLinks(t, manager, id)); got != 2 {
		t.Errorf("expected 2 links after the URL change, got %d", got)
	}
}

func TestArticleLinkRenameSemantics(t *testing.T) {
	env := setupArticleLinkEnv(t)
	manager := env.ClientAs("manager-equipment")

	t.Run("sibling keeps links when one copy is renamed", func(t *testing.T) {
		first := createArticleWithLinks(t, manager, env, "Produkt", "Produkt 1", []map[string]any{
			{"label": "P-guide", "url": "https://example.com/p", "visibility": "public"},
		})
		second := createArticleWithLinks(t, manager, env, "Produkt", "Produkt 2", nil)

		if code := renameArticle(t, manager, env, first, "Produkt Ny", "Produkt 1"); code != http.StatusOK {
			t.Fatalf("rename: expected 200, got %d", code)
		}
		if got := len(articleLinks(t, manager, first)); got != 0 {
			t.Errorf("renamed copy should not take links while the old name is still used, got %d", got)
		}
		if got := len(articleLinks(t, manager, second)); got != 1 {
			t.Errorf("sibling should keep its links, got %d", got)
		}
	})

	t.Run("links follow a true rename", func(t *testing.T) {
		article := createArticleWithLinks(t, manager, env, "Ensam", "Ensam 1", []map[string]any{
			{"label": "E-guide", "url": "https://example.com/e", "visibility": "public"},
		})
		if code := renameArticle(t, manager, env, article, "Ensam Ny", "Ensam 1"); code != http.StatusOK {
			t.Fatalf("rename: expected 200, got %d", code)
		}
		if got := len(articleLinks(t, manager, article)); got != 1 {
			t.Errorf("links should follow the only copy's rename, got %d", got)
		}
	})

	t.Run("merging into an existing product keeps its links", func(t *testing.T) {
		old := createArticleWithLinks(t, manager, env, "Gammal", "Gammal 1", []map[string]any{
			{"label": "Gammal guide", "url": "https://example.com/gammal", "visibility": "public"},
		})
		target := createArticleWithLinks(t, manager, env, "Mal", "Mal 1", []map[string]any{
			{"label": "Mal guide", "url": "https://example.com/mal", "visibility": "public"},
		})

		if code := renameArticle(t, manager, env, old, "Mal", "Gammal 1"); code != http.StatusOK {
			t.Fatalf("rename: expected 200, got %d", code)
		}
		links := articleLinks(t, manager, target)
		if len(links) != 1 || links[0]["label"] != "Mal guide" {
			t.Errorf("target product's links should win, got %v", links)
		}
		var count int
		if err := env.Pool.QueryRow(context.Background(),
			`SELECT count(*) FROM article_links WHERE scope_key = 'Gammal'`).Scan(&count); err != nil {
			t.Fatalf("count links: %v", err)
		}
		if count != 0 {
			t.Errorf("old product's links should be dropped on merge, got %d", count)
		}
	})
}

func TestAvailabilityLinksUseCommonNameFallback(t *testing.T) {
	env := setupArticleLinkEnv(t)
	manager := env.ClientAs("manager-equipment")

	createArticleWithLinks(t, manager, env, "", "Namnlös A", []map[string]any{
		{"label": "A-guide", "url": "https://example.com/a", "visibility": "public"},
	})
	createArticleWithLinks(t, manager, env, "", "Namnlös B", []map[string]any{
		{"label": "B-guide", "url": "https://example.com/b", "visibility": "public"},
	})

	resp, _ := manager.Get("/api/v0/articles/availability?start_date=2030-01-01&end_date=2030-01-02")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("availability: expected 200, got %d", resp.StatusCode)
	}
	var groups []map[string]any
	json.NewDecoder(resp.Body).Decode(&groups)
	if len(groups) != 2 {
		t.Fatalf("expected one group per common name, got %d", len(groups))
	}
	labels := map[string]bool{}
	for _, group := range groups {
		links, _ := group["links"].([]any)
		if len(links) != 1 {
			t.Fatalf("expected one link per group, got %v", group)
		}
		labels[links[0].(map[string]any)["label"].(string)] = true
	}
	if !labels["A-guide"] || !labels["B-guide"] {
		t.Errorf("expected both fallback-scoped links, got %v", labels)
	}
}

func TestCheckNowReportsTotals(t *testing.T) {
	env := setupArticleLinkEnv(t)
	manager := env.ClientAs("manager-equipment")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	createArticleWithLinks(t, manager, env, "Total", "Total 1", []map[string]any{
		{"label": "A", "url": server.URL + "/a", "visibility": "public"},
		{"label": "B", "url": server.URL + "/b", "visibility": "public"},
	})

	partial, err := linkcheck.RunGroup(context.Background(), env.Queries, linkcheck.NewForTest(), "766", 1)
	if err != nil {
		t.Fatalf("check links: %v", err)
	}
	if partial.Total != 2 || partial.Checked != 1 || !partial.Truncated {
		t.Errorf("expected 1/2 checked and truncated, got %+v", partial)
	}

	full, err := linkcheck.RunGroup(context.Background(), env.Queries, linkcheck.NewForTest(), "766", 100)
	if err != nil {
		t.Fatalf("check links: %v", err)
	}
	if full.Checked != 2 || full.Truncated {
		t.Errorf("expected both checked and not truncated, got %+v", full)
	}
}
