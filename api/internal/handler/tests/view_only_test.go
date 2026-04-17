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

func TestAccess_ViewOnlyRestrictions(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.V1(func(r chi.Router) {
		perms := handler.NewPermissionCache(env.Queries)
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries, Perms: perms}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
		r.Mount("/teams", (&handler.TeamHandler{Q: env.Queries}).Routes())
		r.Mount("/group-settings", (&handler.GroupSettingsHandler{Q: env.Queries, Perms: perms}).Routes())
		r.Mount("/issues", (&handler.IssueHandler{Q: env.Queries, Perms: perms}).Routes())
	})

	viewer := env.ClientAs("view-only")
	manager := env.ClientAs("manager-equipment")

	// Create an article for testing
	resp, _ := manager.Get("/api/v0/locations")
	var locations []map[string]any
	json.NewDecoder(resp.Body).Decode(&locations)
	resp.Body.Close()
	locID := locations[0]["id"].(string)

	resp, _ = manager.Get("/api/v0/categories")
	var categories []map[string]any
	json.NewDecoder(resp.Body).Decode(&categories)
	resp.Body.Close()
	catID := categories[0]["id"].(string)

	b, _ := json.Marshal(map[string]any{
		"commercial_name": "TestItem", "common_name": "TestItem 1",
		"category_id": catID, "location_id": locID, "individually_tracked": true,
	})
	resp, _ = manager.Post("/api/v0/articles", bytes.NewReader(b))
	var article map[string]any
	json.NewDecoder(resp.Body).Decode(&article)
	resp.Body.Close()
	articleID := article["id"].(string)

	t.Run("can browse articles", func(t *testing.T) {
		resp, _ := viewer.Get("/api/v0/articles")
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("can list locations", func(t *testing.T) {
		resp, _ := viewer.Get("/api/v0/locations")
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("can list categories", func(t *testing.T) {
		resp, _ := viewer.Get("/api/v0/categories")
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("can list teams", func(t *testing.T) {
		resp, _ := viewer.Get("/api/v0/teams")
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("can report issue", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{
			"article_id":  articleID,
			"severity":    "usable",
			"description": "Looks worn",
		})
		resp, _ := viewer.Post("/api/v0/issues", bytes.NewReader(b))
		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("expected 201, got %d: %s", resp.StatusCode, body)
		}
		resp.Body.Close()
	})

	t.Run("can add article note", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"message": "Just noticed this"})
		resp, _ := viewer.Post("/api/v0/articles/"+articleID+"/events", bytes.NewReader(b))
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("expected 204, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("cannot create article", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{
			"commercial_name": "Nope", "common_name": "Nope 1",
			"category_id": catID, "location_id": locID,
		})
		resp, _ := viewer.Post("/api/v0/articles", bytes.NewReader(b))
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("cannot delete article", func(t *testing.T) {
		resp, _ := viewer.Delete("/api/v0/articles/" + articleID)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("cannot create location", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"name": "Nope"})
		resp, _ := viewer.Post("/api/v0/locations", bytes.NewReader(b))
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("cannot create team", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"name": "Nope", "type": "troop"})
		resp, _ := viewer.Post("/api/v0/teams", bytes.NewReader(b))
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("cannot access group settings", func(t *testing.T) {
		resp, _ := viewer.Get("/api/v0/group-settings")
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("cannot set manager-only status", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"status": "ok", "comment": "Fixed"})
		resp, _ := viewer.Put("/api/v0/articles/"+articleID+"/status", bytes.NewReader(b))
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})
}
