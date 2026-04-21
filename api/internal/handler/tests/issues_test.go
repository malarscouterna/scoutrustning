package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/ms-utrustning/api/internal/handler"
	"github.com/malarscouterna/ms-utrustning/api/internal/testutil"
)

func mountIssueRoutes(env *testutil.TestEnv) {
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries, Perms: handler.NewPermissionCache(env.Queries)}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
		r.Mount("/bookings", (&handler.BookingHandler{Q: env.Queries}).Routes())
		r.Mount("/issues", (&handler.IssueHandler{Q: env.Queries, Perms: handler.NewPermissionCache(env.Queries)}).Routes())
	})
}

func createTestArticle(t *testing.T, client *testutil.TestClient, name string) string {
	t.Helper()
	resp, _ := client.Get("/api/v0/locations")
	var locations []map[string]any
	json.NewDecoder(resp.Body).Decode(&locations)
	resp.Body.Close()

	resp, _ = client.Get("/api/v0/categories")
	var categories []map[string]any
	json.NewDecoder(resp.Body).Decode(&categories)
	resp.Body.Close()

	b, _ := json.Marshal(map[string]any{
		"commercial_name":      name,
		"common_name":          name + " 1",
		"category_id":          categories[0]["id"],
		"location_id":          locations[0]["id"],
		"individually_tracked": true,
	})
	resp, _ = client.Post("/api/v0/articles", bytes.NewReader(b))
	var article map[string]any
	json.NewDecoder(resp.Body).Decode(&article)
	resp.Body.Close()
	return article["id"].(string)
}

func TestIssueFlow_CreateAndGet(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountIssueRoutes(env)

	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")
	articleID := createTestArticle(t, manager, "IssueCreate")

	var issueID string

	t.Run("leader creates issue", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{
			"article_id":  articleID,
			"severity":    "unusable",
			"description": "Tent pole snapped",
		})
		resp, err := leader.Post("/api/v0/issues", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
		}

		var issue map[string]any
		json.NewDecoder(resp.Body).Decode(&issue)

		issueID = issue["id"].(string)
		if issue["status"] != "open" {
			t.Errorf("expected status open, got %v", issue["status"])
		}
		if issue["severity"] != "unusable" {
			t.Errorf("expected severity unusable, got %v", issue["severity"])
		}
		if issue["title"] == "" {
			t.Error("expected auto-generated title to be set")
		}
	})

	t.Run("article status derived to reported_unusable", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/articles/" + articleID)
		defer resp.Body.Close()
		var article map[string]any
		json.NewDecoder(resp.Body).Decode(&article)
		if article["status"] != "reported_unusable" {
			t.Errorf("expected reported_unusable, got %v", article["status"])
		}
	})

	t.Run("issue detail has articles, events, assignees", func(t *testing.T) {
		resp, err := leader.Get("/api/v0/issues/" + issueID)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}

		var issue map[string]any
		json.NewDecoder(resp.Body).Decode(&issue)

		articles := issue["articles"].([]any)
		if len(articles) != 1 {
			t.Errorf("expected 1 linked article, got %d", len(articles))
		}
		events := issue["events"].([]any)
		if len(events) < 1 {
			t.Errorf("expected at least 1 event (creation comment), got %d", len(events))
		}
		assignees := issue["assignees"].([]any)
		if len(assignees) != 0 {
			t.Errorf("expected 0 assignees, got %d", len(assignees))
		}
	})

	t.Run("issue appears in list", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/issues")
		defer resp.Body.Close()
		var issues []any
		json.NewDecoder(resp.Body).Decode(&issues)
		if len(issues) == 0 {
			t.Error("expected at least 1 issue in list")
		}
	})

	t.Run("manager can change status to in_progress", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{
			"status":  "in_progress",
			"comment": "Looking into it",
		})
		resp, err := manager.Put("/api/v0/issues/"+issueID, bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}

		var issue map[string]any
		json.NewDecoder(resp.Body).Decode(&issue)
		if issue["status"] != "in_progress" {
			t.Errorf("expected in_progress, got %v", issue["status"])
		}
	})

	t.Run("article still reported_unusable while in_progress", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/articles/" + articleID)
		defer resp.Body.Close()
		var article map[string]any
		json.NewDecoder(resp.Body).Decode(&article)
		if article["status"] != "reported_unusable" {
			t.Errorf("expected reported_unusable, got %v", article["status"])
		}
	})

	t.Run("manager resolves issue", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{
			"status":  "resolved",
			"comment": "Fixed",
		})
		resp, err := manager.Put("/api/v0/issues/"+issueID, bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}
	})

	t.Run("article status returns to ok after resolve", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/articles/" + articleID)
		defer resp.Body.Close()
		var article map[string]any
		json.NewDecoder(resp.Body).Decode(&article)
		if article["status"] != "ok" {
			t.Errorf("expected ok after resolve, got %v", article["status"])
		}
	})
}

func TestIssueFlow_EnglishTitle(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountIssueRoutes(env)

	manager := env.ClientAs("manager-equipment")
	articleID := createTestArticle(t, manager, "EnglishTitle")

	b, _ := json.Marshal(map[string]any{
		"article_id":  articleID,
		"severity":    "unusable",
		"description": "Testing English title generation",
	})
	req, _ := http.NewRequest("POST", env.Server.URL+"/api/v0/issues", bytes.NewReader(b))
	req.Header.Set("X-Dev-Role-Override", "leader-yggdrasil")
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "paraglide_lang", Value: "en"})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
	}

	var issue map[string]any
	json.NewDecoder(resp.Body).Decode(&issue)
	title, _ := issue["title"].(string)
	if !strings.Contains(title, "Unusable") {
		t.Errorf("expected English severity in title, got %q", title)
	}
	if strings.Contains(title, "Ej användbar") {
		t.Errorf("expected English title, got Swedish: %q", title)
	}
}

func TestIssueFlow_SeverityPriority(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountIssueRoutes(env)

	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")
	articleID := createTestArticle(t, manager, "SeverityPriority")

	// Create usable issue first
	b, _ := json.Marshal(map[string]any{
		"article_id":  articleID,
		"severity":    "usable",
		"description": "Minor scratch",
	})
	resp, _ := leader.Post("/api/v0/issues", bytes.NewReader(b))
	var issue1 map[string]any
	json.NewDecoder(resp.Body).Decode(&issue1)
	resp.Body.Close()
	issue1ID := issue1["id"].(string)

	t.Run("single usable issue gives reported_usable", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/articles/" + articleID)
		defer resp.Body.Close()
		var article map[string]any
		json.NewDecoder(resp.Body).Decode(&article)
		if article["status"] != "reported_usable" {
			t.Errorf("expected reported_usable, got %v", article["status"])
		}
	})

	// Add a missing issue
	b, _ = json.Marshal(map[string]any{
		"article_id":  articleID,
		"severity":    "missing",
		"description": "Cannot find it",
	})
	resp, _ = leader.Post("/api/v0/issues", bytes.NewReader(b))
	var issue2 map[string]any
	json.NewDecoder(resp.Body).Decode(&issue2)
	resp.Body.Close()
	issue2ID := issue2["id"].(string)

	t.Run("missing issue escalates to reported_missing", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/articles/" + articleID)
		defer resp.Body.Close()
		var article map[string]any
		json.NewDecoder(resp.Body).Decode(&article)
		if article["status"] != "reported_missing" {
			t.Errorf("expected reported_missing, got %v", article["status"])
		}
	})

	// Resolve the missing issue; usable one remains
	b, _ = json.Marshal(map[string]any{"status": "resolved"})
	resp, _ = manager.Put("/api/v0/issues/"+issue2ID, bytes.NewReader(b))
	resp.Body.Close()

	t.Run("after missing resolved, drops back to reported_usable", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/articles/" + articleID)
		defer resp.Body.Close()
		var article map[string]any
		json.NewDecoder(resp.Body).Decode(&article)
		if article["status"] != "reported_usable" {
			t.Errorf("expected reported_usable after missing resolved, got %v", article["status"])
		}
	})

	// Resolve the usable issue too
	b, _ = json.Marshal(map[string]any{"status": "resolved"})
	resp, _ = manager.Put("/api/v0/issues/"+issue1ID, bytes.NewReader(b))
	resp.Body.Close()

	t.Run("after all resolved, article back to ok", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/articles/" + articleID)
		defer resp.Body.Close()
		var article map[string]any
		json.NewDecoder(resp.Body).Decode(&article)
		if article["status"] != "ok" {
			t.Errorf("expected ok after all resolved, got %v", article["status"])
		}
	})
}

func TestIssueFlow_Comments(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountIssueRoutes(env)

	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")
	articleID := createTestArticle(t, manager, "IssueComments")

	b, _ := json.Marshal(map[string]any{
		"article_id":  articleID,
		"severity":    "usable",
		"description": "Strap fraying",
	})
	resp, _ := leader.Post("/api/v0/issues", bytes.NewReader(b))
	var issue map[string]any
	json.NewDecoder(resp.Body).Decode(&issue)
	resp.Body.Close()
	issueID := issue["id"].(string)

	t.Run("any user can add comment", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"description": "Will check tomorrow"})
		resp, err := manager.Post("/api/v0/issues/"+issueID+"/comments", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
		}
	})

	t.Run("comment without text rejected", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"description": ""})
		resp, err := leader.Post("/api/v0/issues/"+issueID+"/comments", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("events include all comments", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/issues/" + issueID)
		defer resp.Body.Close()
		var detail map[string]any
		json.NewDecoder(resp.Body).Decode(&detail)

		events := detail["events"].([]any)
		if len(events) < 2 {
			t.Errorf("expected at least 2 events (creation + comment), got %d", len(events))
		}
	})
}

func TestIssueFlow_Assignees(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountIssueRoutes(env)

	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")
	articleID := createTestArticle(t, manager, "IssueAssignee")

	b, _ := json.Marshal(map[string]any{
		"article_id":  articleID,
		"severity":    "unusable",
		"description": "Broken zipper",
	})
	resp, _ := leader.Post("/api/v0/issues", bytes.NewReader(b))
	var issue map[string]any
	json.NewDecoder(resp.Body).Decode(&issue)
	resp.Body.Close()
	issueID := issue["id"].(string)

	t.Run("leader cannot set assignees", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"user_ids": []string{}})
		resp, err := leader.Put("/api/v0/issues/"+issueID+"/assignees", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("manager can set assignees", func(t *testing.T) {
		// manager-equipment persona has member_id "3000002" (from dev-personas.json)
		managerID := "3000002"

		b, _ := json.Marshal(map[string]any{"user_ids": []string{managerID}})
		resp, err := manager.Put("/api/v0/issues/"+issueID+"/assignees", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}

		var assignees []any
		json.NewDecoder(resp.Body).Decode(&assignees)
		if len(assignees) != 1 {
			t.Errorf("expected 1 assignee, got %d", len(assignees))
		}
	})
}

func TestIssueFlow_AccessControl(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountIssueRoutes(env)

	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")
	articleID := createTestArticle(t, manager, "ACIssue")

	b, _ := json.Marshal(map[string]any{
		"article_id":  articleID,
		"severity":    "usable",
		"description": "Test",
	})
	resp, _ := leader.Post("/api/v0/issues", bytes.NewReader(b))
	var issue map[string]any
	json.NewDecoder(resp.Body).Decode(&issue)
	resp.Body.Close()
	issueID := issue["id"].(string)

	t.Run("leader cannot change issue status", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"status": "resolved"})
		resp, err := leader.Put("/api/v0/issues/"+issueID, bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("issue requires article_id", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{
			"severity":    "usable",
			"description": "No article",
		})
		resp, err := leader.Post("/api/v0/issues", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("issue requires description", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{
			"article_id": articleID,
			"severity":   "usable",
		})
		resp, err := leader.Post("/api/v0/issues", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("invalid severity rejected", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{
			"article_id":  articleID,
			"severity":    "broken",
			"description": "Test",
		})
		resp, err := leader.Post("/api/v0/issues", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("article status endpoint no longer accepts reported statuses", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"status": "reported_unusable", "comment": "Test"})
		resp, err := leader.Put("/api/v0/articles/"+articleID+"/status", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 for reported status via article endpoint, got %d", resp.StatusCode)
		}
	})

	t.Run("article status endpoint no longer accepts lost", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"status": "lost", "comment": "Test"})
		resp, err := leader.Put("/api/v0/articles/"+articleID+"/status", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 for lost status via article endpoint, got %d", resp.StatusCode)
		}
	})
}

func TestIssueFlow_MineFilter(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountIssueRoutes(env)

	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")
	leaderB := env.ClientAs("leader-flaskpost")

	articleID := createTestArticle(t, manager, "MineFiler")

	// Leader A creates an issue
	b, _ := json.Marshal(map[string]any{
		"article_id":  articleID,
		"severity":    "usable",
		"description": "Minor issue",
	})
	resp, _ := leader.Post("/api/v0/issues", bytes.NewReader(b))
	resp.Body.Close()

	t.Run("reporter sees issue with mine=true", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/issues?mine=true")
		defer resp.Body.Close()
		var issues []any
		json.NewDecoder(resp.Body).Decode(&issues)
		if len(issues) == 0 {
			t.Error("expected at least 1 issue for reporter with mine=true")
		}
	})

	t.Run("unrelated leader does not see with mine=true", func(t *testing.T) {
		resp, _ := leaderB.Get("/api/v0/issues?mine=true")
		defer resp.Body.Close()
		var issues []any
		json.NewDecoder(resp.Body).Decode(&issues)
		if len(issues) != 0 {
			t.Errorf("expected 0 issues for unrelated leader with mine=true, got %d", len(issues))
		}
	})
}

func TestIssueFlow_ArticleArchived(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountIssueRoutes(env)

	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")
	articleID := createTestArticle(t, manager, "ArchiveIssue")

	b, _ := json.Marshal(map[string]any{
		"article_id":  articleID,
		"severity":    "unusable",
		"description": "Broken",
	})
	resp, _ := leader.Post("/api/v0/issues", bytes.NewReader(b))
	var issue map[string]any
	json.NewDecoder(resp.Body).Decode(&issue)
	resp.Body.Close()
	issueID := issue["id"].(string)

	// Archive the article via the article status endpoint (manager-only)
	b, _ = json.Marshal(map[string]any{"status": "archived", "comment": "Retired"})
	resp, _ = manager.Put("/api/v0/articles/"+articleID+"/status", bytes.NewReader(b))
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("archive failed: %d %s", resp.StatusCode, body)
	}
	resp.Body.Close()

	t.Run("issue remains accessible after article archived", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/issues/" + issueID)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})
}

func TestIssueFlow_FilterByArticle(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountIssueRoutes(env)

	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")

	article1 := createTestArticle(t, manager, "FilterArticle1")
	article2 := createTestArticle(t, manager, "FilterArticle2")

	b, _ := json.Marshal(map[string]any{
		"article_id":  article1,
		"severity":    "usable",
		"description": "Issue on article 1",
	})
	resp, _ := leader.Post("/api/v0/issues", bytes.NewReader(b))
	resp.Body.Close()

	b, _ = json.Marshal(map[string]any{
		"article_id":  article2,
		"severity":    "unusable",
		"description": "Issue on article 2",
	})
	resp, _ = leader.Post("/api/v0/issues", bytes.NewReader(b))
	resp.Body.Close()

	t.Run("filter by article_id returns only matching issues", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/issues?article_id=" + article1)
		defer resp.Body.Close()
		var issues []map[string]any
		json.NewDecoder(resp.Body).Decode(&issues)
		if len(issues) != 1 {
			t.Errorf("expected 1 issue for article1, got %d", len(issues))
		}
	})
}
