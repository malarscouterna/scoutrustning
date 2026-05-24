package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/scoutrustning/api/internal/handler"
	"github.com/malarscouterna/scoutrustning/api/internal/testutil"
)

func TestInventoryManagement(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries, Perms: handler.NewPermissionCache(env.Queries)}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
		r.Mount("/group-settings", (&handler.GroupSettingsHandler{Q: env.Queries, Pool: env.Pool, Perms: handler.NewPermissionCache(env.Queries)}).Routes())
	})

	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")

	// Get seed IDs
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

	t.Run("group settings CRUD", func(t *testing.T) {
		// GET returns defaults when no settings exist
		resp, _ := manager.Get("/api/v0/group-settings")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var settings map[string]any
		json.NewDecoder(resp.Body).Decode(&settings)
		resp.Body.Close()
		if settings["default_approval_level"] != "none" {
			t.Errorf("expected default_approval_level none, got %v", settings["default_approval_level"])
		}

		// PUT saves settings
		body := map[string]any{
			"notification_email_from": "test@example.com",
			"default_approval_level":  "none",
		}
		b, _ := json.Marshal(body)
		resp, _ = manager.Put("/api/v0/group-settings", bytes.NewReader(b))
		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, respBody)
		}
		json.NewDecoder(resp.Body).Decode(&settings)
		resp.Body.Close()
		if settings["notification_email_from"] != "test@example.com" {
			t.Errorf("expected email test@example.com, got %v", settings["notification_email_from"])
		}

		// GET returns saved settings; gchat_webhook_url was removed in migration 00009
		resp, _ = manager.Get("/api/v0/group-settings")
		json.NewDecoder(resp.Body).Decode(&settings)
		resp.Body.Close()
		if _, hasKey := settings["gchat_configured"]; !hasKey {
			t.Errorf("expected gchat_configured in group settings response, got keys: %v", settings)
		}
	})

	t.Run("SMTP settings save and mask", func(t *testing.T) {
		// Encryption key must be set for smtp_key to be accepted.
		t.Setenv("SETTINGS_ENCRYPTION_KEY", "0000000000000000000000000000000000000000000000000000000000000001")

		body := map[string]any{
			"notification_email_from": "utrustning@example.com",
			"smtp_host":               "smtp.example.com",
			"smtp_port":               587,
			"smtp_tls":                "starttls",
			"smtp_user":               "apikey",
			"smtp_key":                "secret-api-key",
		}
		b, _ := json.Marshal(body)
		resp, _ := manager.Put("/api/v0/group-settings", bytes.NewReader(b))
		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, respBody)
		}
		var settings map[string]any
		json.NewDecoder(resp.Body).Decode(&settings)
		resp.Body.Close()

		if settings["smtp_host"] != "smtp.example.com" {
			t.Errorf("expected smtp_host smtp.example.com, got %v", settings["smtp_host"])
		}
		if settings["smtp_key_set"] != true {
			t.Errorf("expected smtp_key_set true, got %v", settings["smtp_key_set"])
		}
		if settings["smtp_key_masked"] == "" {
			t.Errorf("expected smtp_key_masked to be set")
		}

		// Saving without smtp_key (nil) must preserve the existing key.
		body2 := map[string]any{
			"smtp_host": "smtp.example.com",
			"smtp_port": 587,
			"smtp_tls":  "starttls",
			"smtp_user": "apikey",
		}
		b2, _ := json.Marshal(body2)
		resp, _ = manager.Put("/api/v0/group-settings", bytes.NewReader(b2))
		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200 on key-preserve, got %d: %s", resp.StatusCode, respBody)
		}
		var settings2 map[string]any
		json.NewDecoder(resp.Body).Decode(&settings2)
		resp.Body.Close()
		if settings2["smtp_key_set"] != true {
			t.Errorf("expected smtp_key_set to be preserved, got %v", settings2["smtp_key_set"])
		}

		// Clearing SMTP (disabling group SMTP) must clear the key.
		body3 := map[string]any{
			"smtp_host": "",
			"smtp_port": 587,
			"smtp_tls":  "starttls",
			"smtp_user": "",
			"smtp_key":  "",
		}
		b3, _ := json.Marshal(body3)
		resp, _ = manager.Put("/api/v0/group-settings", bytes.NewReader(b3))
		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200 on key-clear, got %d: %s", resp.StatusCode, respBody)
		}
		var settings3 map[string]any
		json.NewDecoder(resp.Body).Decode(&settings3)
		resp.Body.Close()
		if settings3["smtp_key_set"] != false {
			t.Errorf("expected smtp_key_set false after clear, got %v", settings3["smtp_key_set"])
		}
	})

	t.Run("leader cannot access group settings", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/group-settings")
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("location delete blocked by articles", func(t *testing.T) {
		// Create an article in the location
		body := map[string]any{
			"common_name": "Test Article",
			"category_id": catID,
			"location_id": locID,
		}
		b, _ := json.Marshal(body)
		resp, _ := manager.Post("/api/v0/articles", bytes.NewReader(b))
		var article map[string]any
		json.NewDecoder(resp.Body).Decode(&article)
		resp.Body.Close()

		// Try to delete the location
		resp, _ = manager.Delete("/api/v0/locations/" + locID)
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("expected 409, got %d", resp.StatusCode)
		}
		var errBody map[string]any
		json.NewDecoder(resp.Body).Decode(&errBody)
		resp.Body.Close()
		if errBody["error"] != "has_articles" {
			t.Errorf("expected error has_articles, got %v", errBody["error"])
		}
		count := errBody["count"].(float64)
		if count < 1 {
			t.Errorf("expected count >= 1, got %v", count)
		}

		// Clean up
		manager.Delete("/api/v0/articles/" + article["id"].(string))
	})

	t.Run("category delete blocked by articles", func(t *testing.T) {
		body := map[string]any{
			"common_name": "Test Article 2",
			"category_id": catID,
			"location_id": locID,
		}
		b, _ := json.Marshal(body)
		resp, _ := manager.Post("/api/v0/articles", bytes.NewReader(b))
		var article map[string]any
		json.NewDecoder(resp.Body).Decode(&article)
		resp.Body.Close()

		resp, _ = manager.Delete("/api/v0/categories/" + catID)
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("expected 409, got %d", resp.StatusCode)
		}
		var errBody map[string]any
		json.NewDecoder(resp.Body).Decode(&errBody)
		resp.Body.Close()
		if errBody["error"] != "has_articles" {
			t.Errorf("expected error has_articles, got %v", errBody["error"])
		}

		manager.Delete("/api/v0/articles/" + article["id"].(string))
	})

	t.Run("empty location can be deleted", func(t *testing.T) {
		// Create a new location
		b, _ := json.Marshal(map[string]any{"name": "Temp Location", "sort_order": 99})
		resp, _ := manager.Post("/api/v0/locations", bytes.NewReader(b))
		var loc map[string]any
		json.NewDecoder(resp.Body).Decode(&loc)
		resp.Body.Close()

		resp, _ = manager.Delete("/api/v0/locations/" + loc["id"].(string))
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("article with manager_notes and purchase fields", func(t *testing.T) {
		body := map[string]any{
			"common_name":     "Primus 5",
			"commercial_name": "Primus Stormkök",
			"category_id":     catID,
			"location_id":     locID,
			"manager_notes":   "Borrowed from Kustscouterna",
			"purchase_date":   "2024-03-15",
			"purchase_price":  "1299.50",
		}
		b, _ := json.Marshal(body)
		resp, _ := manager.Post("/api/v0/articles", bytes.NewReader(b))
		if resp.StatusCode != http.StatusCreated {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201, got %d: %s", resp.StatusCode, respBody)
		}
		var article map[string]any
		json.NewDecoder(resp.Body).Decode(&article)
		resp.Body.Close()
		articleID := article["id"].(string)

		if article["manager_notes"] != "Borrowed from Kustscouterna" {
			t.Errorf("expected manager_notes, got %v", article["manager_notes"])
		}

		// GET returns the fields
		resp, _ = manager.Get("/api/v0/articles/" + articleID)
		json.NewDecoder(resp.Body).Decode(&article)
		resp.Body.Close()
		if article["manager_notes"] != "Borrowed from Kustscouterna" {
			t.Errorf("expected manager_notes on GET, got %v", article["manager_notes"])
		}

		// Update manager_notes
		body["manager_notes"] = "Updated note"
		body["status"] = "ok"
		body["approval_level"] = "none"
		b, _ = json.Marshal(body)
		resp, _ = manager.Put("/api/v0/articles/"+articleID, bytes.NewReader(b))
		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, respBody)
		}
		json.NewDecoder(resp.Body).Decode(&article)
		resp.Body.Close()
		if article["manager_notes"] != "Updated note" {
			t.Errorf("expected updated manager_notes, got %v", article["manager_notes"])
		}

		manager.Delete("/api/v0/articles/" + articleID)
	})
}
