package tests

import (
	"bytes"
	"encoding/json"
	"encoding/base64"
	"io"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/ms-utrustning/api/internal/handler"
	"github.com/malarscouterna/ms-utrustning/api/internal/testutil"
)

func TestBrowseManagerMode(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
		r.Mount("/bookings", (&handler.BookingHandler{Q: env.Queries}).Routes())
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

	// Helper to create an article
	createArticle := func(name, commercial string, tracked bool) string {
		body := map[string]any{
			"common_name":         name,
			"commercial_name":     commercial,
			"category_id":         catID,
			"location_id":         locID,
			"individually_tracked": tracked,
			"description":         "Test description",
			"instructions":        "Test instructions",
			"approval_level":      "none",
			"manager_notes":       "Test notes",
		}
		b, _ := json.Marshal(body)
		resp, _ := manager.Post("/api/v0/articles", bytes.NewReader(b))
		var article map[string]any
		json.NewDecoder(resp.Body).Decode(&article)
		resp.Body.Close()
		return article["id"].(string)
	}

	t.Run("bulk status change", func(t *testing.T) {
		id1 := createArticle("Bulk1", "BulkTest", true)
		id2 := createArticle("Bulk2", "BulkTest", true)
		defer manager.Delete("/api/v0/articles/" + id1)
		defer manager.Delete("/api/v0/articles/" + id2)

		body := map[string]any{
			"article_ids": []string{id1, id2},
			"status":      "under_repair",
		}
		b, _ := json.Marshal(body)
		resp, _ := manager.Put("/api/v0/articles/bulk", bytes.NewReader(b))
		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, respBody)
		}
		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		if int(result["updated"].(float64)) != 2 {
			t.Errorf("expected 2 updated, got %v", result["updated"])
		}

		// Verify status changed
		resp, _ = manager.Get("/api/v0/articles/" + id1)
		var a map[string]any
		json.NewDecoder(resp.Body).Decode(&a)
		resp.Body.Close()
		if a["status"] != "under_repair" {
			t.Errorf("expected under_repair, got %v", a["status"])
		}
	})

	t.Run("bulk location move", func(t *testing.T) {
		// Create a second location
		b, _ := json.Marshal(map[string]any{"name": "Bulk Move Target", "sort_order": 99})
		resp, _ := manager.Post("/api/v0/locations", bytes.NewReader(b))
		var loc2 map[string]any
		json.NewDecoder(resp.Body).Decode(&loc2)
		resp.Body.Close()
		loc2ID := loc2["id"].(string)
		defer manager.Delete("/api/v0/locations/" + loc2ID)

		id1 := createArticle("Move1", "MoveTest", true)
		defer manager.Delete("/api/v0/articles/" + id1)

		body := map[string]any{
			"article_ids": []string{id1},
			"location_id": loc2ID,
		}
		b, _ = json.Marshal(body)
		resp, _ = manager.Put("/api/v0/articles/bulk", bytes.NewReader(b))
		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, respBody)
		}
		resp.Body.Close()

		resp, _ = manager.Get("/api/v0/articles/" + id1)
		var a map[string]any
		json.NewDecoder(resp.Body).Decode(&a)
		resp.Body.Close()
		if a["location_id"] != loc2ID {
			t.Errorf("expected location %s, got %v", loc2ID, a["location_id"])
		}
	})

	t.Run("leader cannot bulk update", func(t *testing.T) {
		body := map[string]any{
			"article_ids": []string{"00000000-0000-0000-0000-000000000000"},
			"status":      "archived",
		}
		b, _ := json.Marshal(body)
		resp, _ := leader.Put("/api/v0/articles/bulk", bytes.NewReader(b))
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})

	t.Run("group count increase", func(t *testing.T) {
		// Create 2 quantity tracked articles
		id1 := createArticle("CountTest", "CountTest", false)
		id2 := createArticle("CountTest", "CountTest", false)
		defer manager.Delete("/api/v0/articles/" + id1)
		defer manager.Delete("/api/v0/articles/" + id2)

		// Increase to 4
		body := map[string]any{
			"commercial_name": "CountTest",
			"location_id":     locID,
			"new_count":       4,
		}
		b, _ := json.Marshal(body)
		resp, _ := manager.Post("/api/v0/articles/group-count", bytes.NewReader(b))
		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, respBody)
		}
		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		if int(result["count"].(float64)) != 4 {
			t.Errorf("expected count 4, got %v", result["count"])
		}

		// Verify event logged (use group-events since event is on representative)
		resp, _ = manager.Get("/api/v0/articles/" + id1 + "/group-events")
		var events map[string]any
		json.NewDecoder(resp.Body).Decode(&events)
		resp.Body.Close()
		evList := events["events"].([]any)
		found := false
		for _, e := range evList {
			ev := e.(map[string]any)
			if ev["event_type"] == "count_changed" {
				// metadata may be map or JSON string depending on serialization
				var meta map[string]any
				switch m := ev["metadata"].(type) {
				case map[string]any:
					meta = m
				case string:
					if decoded, err := base64.StdEncoding.DecodeString(m); err == nil {
						json.Unmarshal(decoded, &meta)
					} else {
						json.Unmarshal([]byte(m), &meta)
					}
				}
				if meta != nil && meta["old_count"] == "2" && meta["new_count"] == "4" {
					found = true
				}
			}
		}
		if !found {
			evJSON, _ := json.MarshalIndent(evList, "", "  ")
			t.Errorf("expected count_changed event with old_count=2, new_count=4, events: %s", evJSON)
		}
	})

	t.Run("group count decrease", func(t *testing.T) {
		id1 := createArticle("DecTest", "DecTest", false)
		id2 := createArticle("DecTest", "DecTest", false)
		id3 := createArticle("DecTest", "DecTest", false)
		defer manager.Delete("/api/v0/articles/" + id1)
		defer manager.Delete("/api/v0/articles/" + id2)
		defer manager.Delete("/api/v0/articles/" + id3)

		body := map[string]any{
			"commercial_name": "DecTest",
			"location_id":     locID,
			"new_count":       1,
		}
		b, _ := json.Marshal(body)
		resp, _ := manager.Post("/api/v0/articles/group-count", bytes.NewReader(b))
		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, respBody)
		}
		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		if int(result["count"].(float64)) != 1 {
			t.Errorf("expected count 1, got %v", result["count"])
		}
	})

	t.Run("group update applies to all articles", func(t *testing.T) {
		id1 := createArticle("GrpUpd1", "GrpUpdTest", false)
		id2 := createArticle("GrpUpd2", "GrpUpdTest", false)
		defer manager.Delete("/api/v0/articles/" + id1)
		defer manager.Delete("/api/v0/articles/" + id2)

		body := map[string]any{
			"commercial_name":     "GrpUpdTest",
			"common_name":         "GrpUpd1",
			"category_id":         catID,
			"location_id":         locID,
			"description":         "Updated group desc",
			"instructions":        "Updated group instr",
			"approval_level":      "low",
			"manager_notes":       "Updated group notes",
			"individually_tracked": false,
			"status":              "ok",
			"place":               "Shelf 5",
		}
		b, _ := json.Marshal(body)
		resp, _ := manager.Put("/api/v0/articles/"+id1+"?group=true", bytes.NewReader(b))
		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, respBody)
		}
		resp.Body.Close()

		// Verify second article got the update
		resp, _ = manager.Get("/api/v0/articles/" + id2)
		var a2 map[string]any
		json.NewDecoder(resp.Body).Decode(&a2)
		resp.Body.Close()
		if a2["description"] != "Updated group desc" {
			t.Errorf("expected updated description on sibling, got %v", a2["description"])
		}
		if a2["instructions"] != "Updated group instr" {
			t.Errorf("expected updated instructions on sibling, got %v", a2["instructions"])
		}
	})

	t.Run("shared field propagation on individually tracked save", func(t *testing.T) {
		id1 := createArticle("Prop1", "PropTest", true)
		id2 := createArticle("Prop2", "PropTest", true)
		defer manager.Delete("/api/v0/articles/" + id1)
		defer manager.Delete("/api/v0/articles/" + id2)

		// Update id1 with new description
		body := map[string]any{
			"commercial_name":     "PropTest",
			"common_name":         "Prop1",
			"category_id":         catID,
			"location_id":         locID,
			"description":         "Propagated description",
			"instructions":        "Propagated instructions",
			"manager_notes":       "Propagated notes",
			"approval_level":      "high",
			"individually_tracked": true,
			"status":              "ok",
		}
		b, _ := json.Marshal(body)
		resp, _ := manager.Put("/api/v0/articles/"+id1, bytes.NewReader(b))
		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, respBody)
		}
		resp.Body.Close()

		// Verify shared fields propagated to id2
		resp, _ = manager.Get("/api/v0/articles/" + id2)
		var a2 map[string]any
		json.NewDecoder(resp.Body).Decode(&a2)
		resp.Body.Close()
		if a2["description"] != "Propagated description" {
			t.Errorf("expected propagated description, got %v", a2["description"])
		}
		if a2["instructions"] != "Propagated instructions" {
			t.Errorf("expected propagated instructions, got %v", a2["instructions"])
		}
		if a2["manager_notes"] != "Propagated notes" {
			t.Errorf("expected propagated notes, got %v", a2["manager_notes"])
		}
		// approval_level should NOT propagate (per-item)
		if a2["approval_level"] != "none" {
			t.Errorf("expected approval_level to stay none (per-item), got %v", a2["approval_level"])
		}
	})

	t.Run("group events aggregates across articles", func(t *testing.T) {
		id1 := createArticle("EvGrp1", "EvGrpTest", false)
		id2 := createArticle("EvGrp2", "EvGrpTest", false)
		defer manager.Delete("/api/v0/articles/" + id1)
		defer manager.Delete("/api/v0/articles/" + id2)

		// Report issue on id1
		b, _ := json.Marshal(map[string]any{"status": "reported_usable", "comment": "Wobbly"})
		resp, _ := manager.Put("/api/v0/articles/"+id1+"/status", bytes.NewReader(b))
		resp.Body.Close()

		// Report issue on id2
		b, _ = json.Marshal(map[string]any{"status": "reported_unusable", "comment": "Broken"})
		resp, _ = manager.Put("/api/v0/articles/"+id2+"/status", bytes.NewReader(b))
		resp.Body.Close()

		// Group events should include both
		resp, _ = manager.Get("/api/v0/articles/" + id1 + "/group-events")
		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, respBody)
		}
		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		evList := result["events"].([]any)
		if len(evList) < 2 {
			t.Errorf("expected at least 2 group events, got %d", len(evList))
		}
	})

	t.Run("leader cannot use group-count", func(t *testing.T) {
		body := map[string]any{
			"commercial_name": "X",
			"location_id":     locID,
			"new_count":       5,
		}
		b, _ := json.Marshal(body)
		resp, _ := leader.Post("/api/v0/articles/group-count", bytes.NewReader(b))
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	})
}
