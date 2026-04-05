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

func mountIssueRoutes(env *testutil.TestEnv) {
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
		r.Mount("/bookings", (&handler.BookingHandler{Q: env.Queries}).Routes())
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

func TestIssueFlow_ReportAndResolve(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountIssueRoutes(env)

	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")
	articleID := createTestArticle(t, manager, "IssueTest")

	t.Run("leader reports issue as usable", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{
			"status":  "reported_usable",
			"comment": "Tent has a tear",
		})
		resp, err := leader.Put("/api/v0/articles/"+articleID+"/status", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}
		var article map[string]any
		json.NewDecoder(resp.Body).Decode(&article)
		if article["status"] != "reported_usable" {
			t.Errorf("expected reported_usable, got %v", article["status"])
		}
	})

	t.Run("report as unusable updates status", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{
			"status":  "reported_unusable",
			"comment": "Pole snapped",
		})
		resp, _ := leader.Put("/api/v0/articles/"+articleID+"/status", bytes.NewReader(b))
		defer resp.Body.Close()
		var article map[string]any
		json.NewDecoder(resp.Body).Decode(&article)
		if article["status"] != "reported_unusable" {
			t.Errorf("expected reported_unusable, got %v", article["status"])
		}
	})

	t.Run("manager updates status to under_repair without resolving", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{
			"status":  "under_repair",
			"comment": "Sent to repair shop",
		})
		resp, err := manager.Put("/api/v0/articles/"+articleID+"/status", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}
		var article map[string]any
		json.NewDecoder(resp.Body).Decode(&article)
		if article["status"] != "under_repair" {
			t.Errorf("expected under_repair, got %v", article["status"])
		}
	})

	t.Run("manager resolves by setting status back to ok", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{
			"status":  "ok",
			"comment": "Repair complete",
		})
		resp, _ := manager.Put("/api/v0/articles/"+articleID+"/status", bytes.NewReader(b))
		defer resp.Body.Close()
		var article map[string]any
		json.NewDecoder(resp.Body).Decode(&article)
		if article["status"] != "ok" {
			t.Errorf("expected ok, got %v", article["status"])
		}
	})

	t.Run("events contain full history", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/articles/" + articleID + "/events")
		defer resp.Body.Close()
		var events []map[string]any
		json.NewDecoder(resp.Body).Decode(&events)

		if len(events) < 3 {
			t.Fatalf("expected at least 3 events, got %d", len(events))
		}

		types := map[string]bool{}
		for _, e := range events {
			types[e["event_type"].(string)] = true
		}
		if !types["issue_reported"] {
			t.Error("expected issue_reported event")
		}
		if !types["issue_resolved"] {
			t.Error("expected issue_resolved event")
		}
		if !types["status_change"] {
			t.Error("expected status_change event")
		}
	})
}

func TestIssueFlow_AccessControl(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountIssueRoutes(env)

	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")
	articleID := createTestArticle(t, manager, "ACTest")

	t.Run("leader cannot set manager-only status", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"status": "ok"})
		resp, err := leader.Put("/api/v0/articles/"+articleID+"/status", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("report without comment rejected", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"status": "reported_usable"})
		resp, err := leader.Put("/api/v0/articles/"+articleID+"/status", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("missing status rejected", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"comment": "Test"})
		resp, err := leader.Put("/api/v0/articles/"+articleID+"/status", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("leader can report lost", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"status": "lost", "comment": "Cannot find it"})
		resp, err := leader.Put("/api/v0/articles/"+articleID+"/status", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}
		var article map[string]any
		json.NewDecoder(resp.Body).Decode(&article)
		if article["status"] != "lost" {
			t.Errorf("expected lost, got %v", article["status"])
		}
	})
}

func TestIssueFlow_ReturnCreatesEvent(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountIssueRoutes(env)

	leader := env.ClientAs("leader-yggdrasil")
	bookingID, itemIDs, articleIDs := setupReturnEnv(t, env, 1, 1)

	b, _ := json.Marshal(map[string]any{
		"return_status": "reported_unusable",
		"notes":         "Cracked frame",
	})
	resp, _ := leader.Put("/api/v0/bookings/"+bookingID+"/items/"+itemIDs[0]+"/return", bytes.NewReader(b))
	resp.Body.Close()

	t.Run("reported_unusable return creates article event", func(t *testing.T) {
		resp, err := leader.Get("/api/v0/articles/" + articleIDs[0] + "/events")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var events []map[string]any
		json.NewDecoder(resp.Body).Decode(&events)

		found := false
		for _, e := range events {
			if e["event_type"] == "issue_reported" {
				found = true
				if e["description"] != "Cracked frame" {
					t.Errorf("expected description 'Cracked frame', got %v", e["description"])
				}
			}
		}
		if !found {
			t.Error("expected issue_reported event from reported_unusable return")
		}
	})

	t.Run("article status is reported_unusable", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/articles/" + articleIDs[0])
		defer resp.Body.Close()
		var article map[string]any
		json.NewDecoder(resp.Body).Decode(&article)
		if article["status"] != "reported_unusable" {
			t.Errorf("expected reported_unusable, got %v", article["status"])
		}
	})
}
