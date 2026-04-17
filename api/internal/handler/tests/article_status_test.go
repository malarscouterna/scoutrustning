package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/ms-utrustning/api/internal/handler"
	"github.com/malarscouterna/ms-utrustning/api/internal/testutil"
)

func mountArticleStatusRoutes(env *testutil.TestEnv) {
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries, Perms: handler.NewPermissionCache(env.Queries)}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
		r.Mount("/bookings", (&handler.BookingHandler{Q: env.Queries}).Routes())
		r.Mount("/issues", (&handler.IssueHandler{Q: env.Queries, Perms: handler.NewPermissionCache(env.Queries)}).Routes())
	})
}

// createReportedUsableArticle creates an article with a reported_usable issue open on it.
func createReportedUsableArticle(t *testing.T, env *testutil.TestEnv, name string) string {
	t.Helper()
	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")

	articleID := createTestArticle(t, manager, name)
	b, _ := json.Marshal(map[string]any{
		"article_id":  articleID,
		"severity":    "usable",
		"description": "Minor scratch",
	})
	resp, _ := leader.Post("/api/v0/issues", bytes.NewReader(b))
	resp.Body.Close()
	return articleID
}

func TestAvailability_IncomingArticles(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountArticleStatusRoutes(env)

	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")

	// Create article, set to incoming with expected_available_date
	articleID := createTestArticle(t, manager, "IncomingTest")

	b, _ := json.Marshal(map[string]any{
		"status":                  "incoming",
		"comment":                 "Ordered, arriving June 10",
		"expected_available_date": "2026-06-10",
	})
	resp, _ := manager.Put("/api/v0/articles/"+articleID+"/status", bytes.NewReader(b))
	resp.Body.Close()

	t.Run("incoming article NOT available before expected date", func(t *testing.T) {
		resp, err := leader.Get("/api/v0/articles/availability/articles?start_date=2026-06-05&end_date=2026-06-08&commercial_name=IncomingTest")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		var articles []map[string]any
		json.NewDecoder(resp.Body).Decode(&articles)
		if len(articles) != 0 {
			t.Errorf("expected 0 available before expected date, got %d", len(articles))
		}
	})

	t.Run("incoming article available after expected date", func(t *testing.T) {
		resp, err := leader.Get("/api/v0/articles/availability/articles?start_date=2026-06-10&end_date=2026-06-15&commercial_name=IncomingTest")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		var articles []map[string]any
		json.NewDecoder(resp.Body).Decode(&articles)
		if len(articles) != 1 {
			t.Errorf("expected 1 available after expected date, got %d", len(articles))
		}
	})

	t.Run("incoming article bookable for future dates", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"start_date": "2026-06-15", "end_date": "2026-06-20"})
		resp, _ := leader.Post("/api/v0/bookings", bytes.NewReader(b))
		var booking map[string]any
		json.NewDecoder(resp.Body).Decode(&booking)
		resp.Body.Close()

		b, _ = json.Marshal(map[string]any{"commercial_name": "IncomingTest", "quantity": 1})
		resp, err := leader.Post("/api/v0/bookings/"+booking["id"].(string)+"/items", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
		}
	})
}

func TestAvailability_IncomingWithoutDateUnbookable(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountArticleStatusRoutes(env)

	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")

	articleID := createTestArticle(t, manager, "IncomingNoDate")

	// Set to incoming WITHOUT expected_available_date
	b, _ := json.Marshal(map[string]any{
		"status":  "incoming",
		"comment": "Ordered, no delivery date yet",
	})
	resp, _ := manager.Put("/api/v0/articles/"+articleID+"/status", bytes.NewReader(b))
	resp.Body.Close()

	t.Run("incoming without date is unbookable", func(t *testing.T) {
		resp, err := leader.Get("/api/v0/articles/availability/articles?start_date=2026-06-01&end_date=2026-12-31&commercial_name=IncomingNoDate")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		var articles []map[string]any
		json.NewDecoder(resp.Body).Decode(&articles)
		if len(articles) != 0 {
			t.Errorf("expected 0 available (no expected date), got %d", len(articles))
		}
	})
}

func TestAvailability_UnderRepairWithExpectedDate(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountArticleStatusRoutes(env)

	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")

	articleID := createTestArticle(t, manager, "RepairTest")

	b, _ := json.Marshal(map[string]any{
		"status":                  "under_repair",
		"comment":                 "Sent for repair, back by June 15",
		"expected_available_date": "2026-06-15",
	})
	resp, _ := manager.Put("/api/v0/articles/"+articleID+"/status", bytes.NewReader(b))
	resp.Body.Close()

	t.Run("under_repair NOT available before expected date", func(t *testing.T) {
		resp, err := leader.Get("/api/v0/articles/availability/articles?start_date=2026-06-10&end_date=2026-06-14&commercial_name=RepairTest")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		var articles []map[string]any
		json.NewDecoder(resp.Body).Decode(&articles)
		if len(articles) != 0 {
			t.Errorf("expected 0 available before repair done, got %d", len(articles))
		}
	})

	t.Run("under_repair available after expected date", func(t *testing.T) {
		resp, err := leader.Get("/api/v0/articles/availability/articles?start_date=2026-06-15&end_date=2026-06-20&commercial_name=RepairTest")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		var articles []map[string]any
		json.NewDecoder(resp.Body).Decode(&articles)
		if len(articles) != 1 {
			t.Errorf("expected 1 available after repair date, got %d", len(articles))
		}
	})
}

func TestArticleStatus_ExpectedDateValidation(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountArticleStatusRoutes(env)

	manager := env.ClientAs("manager-equipment")
	articleID := createTestArticle(t, manager, "DateValidation")

	t.Run("expected_available_date rejected for ok status", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{
			"status":                  "ok",
			"comment":                 "test",
			"expected_available_date": "2026-06-10",
		})
		resp, err := manager.Put("/api/v0/articles/"+articleID+"/status", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("expected_available_date accepted for incoming", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{
			"status":                  "incoming",
			"comment":                 "Ordered",
			"expected_available_date": "2026-06-10",
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
		dateVal, _ := article["expected_available_date"].(string)
		if dateVal == "" || !containsDate(dateVal, "2026-06-10") {
			t.Errorf("expected date containing 2026-06-10, got %v", article["expected_available_date"])
		}
	})

	t.Run("expected_available_date accepted for under_repair", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{
			"status":                  "under_repair",
			"comment":                 "Repair",
			"expected_available_date": "2026-07-01",
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
	})

	t.Run("invalid status rejected", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"status": "booked", "comment": "test"})
		resp, err := manager.Put("/api/v0/articles/"+articleID+"/status", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 for removed status 'booked', got %d", resp.StatusCode)
		}
	})
}

func TestReturnFlow_ReturnOkPreservesCondition(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountArticleStatusRoutes(env)

	leader := env.ClientAs("leader-yggdrasil")

	// Create article with an open issue (reported_usable)
	articleID := createReportedUsableArticle(t, env, "OrthoReturn")

	// Verify it's reported_usable
	resp, _ := leader.Get("/api/v0/articles/" + articleID)
	var art map[string]any
	json.NewDecoder(resp.Body).Decode(&art)
	resp.Body.Close()
	if art["status"] != "reported_usable" {
		t.Fatalf("expected reported_usable, got %v", art["status"])
	}

	// Book it, pick up, return OK
	bookingID, itemIDs, _ := setupReturnEnvWithArticle(t, env, articleID)

	b, _ := json.Marshal(map[string]any{"return_status": "returned_ok"})
	resp, _ = leader.Put("/api/v0/bookings/"+bookingID+"/items/"+itemIDs[0]+"/return", bytes.NewReader(b))
	resp.Body.Close()

	t.Run("returned_ok preserves existing condition", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/articles/" + articleID)
		defer resp.Body.Close()
		var article map[string]any
		json.NewDecoder(resp.Body).Decode(&article)
		if article["status"] != "reported_usable" {
			t.Errorf("expected reported_usable preserved after returned_ok, got %v", article["status"])
		}
	})
}

// setupReturnEnvWithArticle creates a picked_up booking containing the given article,
// with the item already marked as picked_up. Returns booking ID and item IDs.
func setupReturnEnvWithArticle(t *testing.T, env *testutil.TestEnv, articleID string) (bookingID string, itemIDs []string, articleIDs []string) {
	t.Helper()
	leader := env.ClientAs("leader-yggdrasil")

	// Get article details to know commercial_name
	resp, _ := leader.Get("/api/v0/articles/" + articleID)
	var art map[string]any
	json.NewDecoder(resp.Body).Decode(&art)
	resp.Body.Close()
	commercialName := art["commercial_name"].(string)

	now := time.Now()
	b, _ := json.Marshal(map[string]any{"start_date": now.Format("2006-01-02"), "end_date": now.AddDate(0, 0, 5).Format("2006-01-02")})
	resp, _ = leader.Post("/api/v0/bookings", bytes.NewReader(b))
	var booking map[string]any
	json.NewDecoder(resp.Body).Decode(&booking)
	resp.Body.Close()
	bookingID = booking["id"].(string)

	b, _ = json.Marshal(map[string]any{"commercial_name": commercialName, "quantity": 1})
	resp, _ = leader.Post("/api/v0/bookings/"+bookingID+"/items", bytes.NewReader(b))
	resp.Body.Close()

	resp, _ = leader.Post("/api/v0/bookings/"+bookingID+"/submit", nil)
	resp.Body.Close()

	resp, _ = leader.Post("/api/v0/bookings/"+bookingID+"/pickup", nil)
	resp.Body.Close()

	// Get item IDs and mark as picked up
	resp, _ = leader.Get("/api/v0/bookings/" + bookingID)
	var detail map[string]any
	json.NewDecoder(resp.Body).Decode(&detail)
	resp.Body.Close()

	for _, item := range detail["items"].([]any) {
		id := item.(map[string]any)["id"].(string)
		itemIDs = append(itemIDs, id)
		articleIDs = append(articleIDs, item.(map[string]any)["article_id"].(string))
		b, _ = json.Marshal(map[string]any{"pickup_status": "picked_up"})
		resp, _ = leader.Put("/api/v0/bookings/"+bookingID+"/items/"+id+"/pickup", bytes.NewReader(b))
		resp.Body.Close()
	}

	return bookingID, itemIDs, articleIDs
}

func TestPickupFlow_UndoPreservesCondition(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountArticleStatusRoutes(env)

	leader := env.ClientAs("leader-yggdrasil")

	// Create article with reported_usable via issue
	articleID := createReportedUsableArticle(t, env, "UndoPickup")

	// Create booking spanning today, submit, pickup
	now := time.Now()
	b, _ := json.Marshal(map[string]any{"start_date": now.Format("2006-01-02"), "end_date": now.AddDate(0, 0, 5).Format("2006-01-02")})
	resp, _ := leader.Post("/api/v0/bookings", bytes.NewReader(b))
	var booking map[string]any
	json.NewDecoder(resp.Body).Decode(&booking)
	resp.Body.Close()
	bookingID := booking["id"].(string)

	b, _ = json.Marshal(map[string]any{"commercial_name": "UndoPickup", "quantity": 1})
	resp, _ = leader.Post("/api/v0/bookings/"+bookingID+"/items", bytes.NewReader(b))
	resp.Body.Close()

	resp, _ = leader.Post("/api/v0/bookings/"+bookingID+"/submit", nil)
	resp.Body.Close()

	resp, _ = leader.Post("/api/v0/bookings/"+bookingID+"/pickup", nil)
	resp.Body.Close()

	// Get item ID
	resp, _ = leader.Get("/api/v0/bookings/" + bookingID)
	var detail map[string]any
	json.NewDecoder(resp.Body).Decode(&detail)
	resp.Body.Close()
	itemID := detail["items"].([]any)[0].(map[string]any)["id"].(string)

	// Mark as picked_up, then undo
	b, _ = json.Marshal(map[string]any{"pickup_status": "picked_up"})
	resp, _ = leader.Put("/api/v0/bookings/"+bookingID+"/items/"+itemID+"/pickup", bytes.NewReader(b))
	resp.Body.Close()

	b, _ = json.Marshal(map[string]any{"pickup_status": ""})
	resp, _ = leader.Put("/api/v0/bookings/"+bookingID+"/items/"+itemID+"/pickup", bytes.NewReader(b))
	resp.Body.Close()

	t.Run("undo pickup preserves reported_usable", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/articles/" + articleID)
		defer resp.Body.Close()
		var article map[string]any
		json.NewDecoder(resp.Body).Decode(&article)
		if article["status"] != "reported_usable" {
			t.Errorf("expected reported_usable preserved after pickup undo, got %v", article["status"])
		}
	})
}

func containsDate(val, date string) bool {
	return len(val) >= len(date) && val[:len(date)] == date
}
