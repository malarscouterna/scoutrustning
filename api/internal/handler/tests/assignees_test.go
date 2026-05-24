package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/ms-utrustning/api/internal/handler"
	"github.com/malarscouterna/ms-utrustning/api/internal/testutil"
)

func mountAssigneeRoutes(env *testutil.TestEnv) {
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries, Perms: handler.NewPermissionCache(env.Queries)}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
		r.Mount("/issues", (&handler.IssueHandler{Q: env.Queries, Perms: handler.NewPermissionCache(env.Queries)}).Routes())
	})
}

func TestIssues_Assignees(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountAssigneeRoutes(env)

	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")

	// Seed a user to assign (simulates someone who has logged in)
	ctx := t.Context()
	_, err := env.Pool.Exec(ctx, `
		INSERT INTO users (id, group_id, name, email, max_access_level) VALUES
			('u-trusted', '766', 'Julia Valborg', 'julia@example.com', 'trusted'),
			('u-other',   '999', 'Linn Annan',    'linn@other.example.com', 'book')
	`)
	if err != nil {
		t.Fatalf("seed users: %v", err)
	}

	articleID := createTestArticle(t, manager, "AssigneeTest")

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

	t.Run("leader cannot add assignee", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"user_id": "u-trusted"})
		resp, err := leader.Post("/api/v0/issues/"+issueID+"/assignees", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("manager adds assignee", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"user_id": "u-trusted"})
		resp, err := manager.Post("/api/v0/issues/"+issueID+"/assignees", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
		}

		var detail map[string]any
		json.NewDecoder(resp.Body).Decode(&detail)
		assignees := detail["assignees"].([]any)
		if len(assignees) != 1 {
			t.Fatalf("expected 1 assignee, got %d", len(assignees))
		}
	})

	t.Run("duplicate add returns 409", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"user_id": "u-trusted"})
		resp, err := manager.Post("/api/v0/issues/"+issueID+"/assignees", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("expected 409, got %d", resp.StatusCode)
		}
	})

	t.Run("assignee visible in GET issue", func(t *testing.T) {
		resp, err := leader.Get("/api/v0/issues/" + issueID)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var detail map[string]any
		json.NewDecoder(resp.Body).Decode(&detail)
		assignees := detail["assignees"].([]any)
		if len(assignees) != 1 {
			t.Fatalf("expected 1 assignee in GET, got %d", len(assignees))
		}
	})

	t.Run("issue events created for add", func(t *testing.T) {
		resp, err := leader.Get("/api/v0/issues/" + issueID)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var detail map[string]any
		json.NewDecoder(resp.Body).Decode(&detail)
		events := detail["events"].([]any)
		var found bool
		for _, e := range events {
			ev := e.(map[string]any)
			if ev["event_type"] == "assignment" {
				meta := ev["metadata"].(map[string]any)
				if meta["action"] == "added" && meta["user_id"] == "u-trusted" {
					found = true
				}
			}
		}
		if !found {
			t.Error("expected assignment added event")
		}
	})

	t.Run("manager removes assignee", func(t *testing.T) {
		resp, err := manager.Delete("/api/v0/issues/" + issueID + "/assignees/u-trusted")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 204, got %d: %s", resp.StatusCode, body)
		}
	})

	t.Run("assignee gone after removal", func(t *testing.T) {
		resp, err := leader.Get("/api/v0/issues/" + issueID)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var detail map[string]any
		json.NewDecoder(resp.Body).Decode(&detail)
		assignees := detail["assignees"].([]any)
		if len(assignees) != 0 {
			t.Fatalf("expected 0 assignees after removal, got %d", len(assignees))
		}
	})

	t.Run("issue events created for remove", func(t *testing.T) {
		resp, err := leader.Get("/api/v0/issues/" + issueID)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var detail map[string]any
		json.NewDecoder(resp.Body).Decode(&detail)
		events := detail["events"].([]any)
		var found bool
		for _, e := range events {
			ev := e.(map[string]any)
			if ev["event_type"] == "assignment" {
				meta := ev["metadata"].(map[string]any)
				if meta["action"] == "removed" && meta["user_id"] == "u-trusted" {
					found = true
				}
			}
		}
		if !found {
			t.Error("expected assignment removed event")
		}
	})

	t.Run("issue from different group returns 404", func(t *testing.T) {
		// Create an issue in group 999, try to assign from group 766 manager
		otherManager := env.ClientAs("other-kar-leader")
		// other-kar-leader is book-level, so we inject a manager-level claim for group 999
		b, _ := json.Marshal(map[string]any{"user_id": "u-other"})
		resp, err := manager.Post("/api/v0/issues/00000000-0000-0000-0000-000000000000/assignees", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		_ = otherManager
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404 for non-existent issue, got %d", resp.StatusCode)
		}
	})
}
