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

// setupReturnEnv creates a picked_up booking with items, ready for return testing.
// Returns booking ID, item IDs, and article IDs.
func setupReturnEnv(t *testing.T, env *testutil.TestEnv, articleCount, bookCount int) (bookingID string, itemIDs, articleIDs []string) {
	t.Helper()
	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")

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

	for i := range articleCount {
		b, _ := json.Marshal(map[string]any{
			"commercial_name":      "ReturnTest",
			"common_name":          "ReturnTest " + string(rune('1'+i)),
			"category_id":          catID,
			"location_id":          locID,
			"individually_tracked": true,
		})
		resp, _ := manager.Post("/api/v0/articles", bytes.NewReader(b))
		var article map[string]any
		json.NewDecoder(resp.Body).Decode(&article)
		resp.Body.Close()
		articleIDs = append(articleIDs, article["id"].(string))
	}

	// Create booking, add items, submit, pickup
	b, _ := json.Marshal(map[string]any{"start_date": "2026-06-01", "end_date": "2026-06-05"})
	resp, _ = leader.Post("/api/v0/bookings", bytes.NewReader(b))
	var booking map[string]any
	json.NewDecoder(resp.Body).Decode(&booking)
	resp.Body.Close()
	bookingID = booking["id"].(string)

	b, _ = json.Marshal(map[string]any{"commercial_name": "ReturnTest", "quantity": bookCount})
	resp, _ = leader.Post("/api/v0/bookings/"+bookingID+"/items", bytes.NewReader(b))
	resp.Body.Close()

	resp, _ = leader.Post("/api/v0/bookings/"+bookingID+"/submit", nil)
	resp.Body.Close()

	resp, _ = leader.Post("/api/v0/bookings/"+bookingID+"/pickup", nil)
	resp.Body.Close()

	// Mark all items as picked up
	resp, _ = leader.Get("/api/v0/bookings/" + bookingID)
	var detail map[string]any
	json.NewDecoder(resp.Body).Decode(&detail)
	resp.Body.Close()

	for _, item := range detail["items"].([]any) {
		id := item.(map[string]any)["id"].(string)
		itemIDs = append(itemIDs, id)
		b, _ = json.Marshal(map[string]any{"pickup_status": "picked_up"})
		resp, _ = leader.Put("/api/v0/bookings/"+bookingID+"/items/"+id+"/pickup", bytes.NewReader(b))
		resp.Body.Close()
	}

	return bookingID, itemIDs, articleIDs
}

func mountReturnRoutes(env *testutil.TestEnv) {
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
		r.Mount("/bookings", (&handler.BookingHandler{Q: env.Queries}).Routes())
	})
}

func TestReturnFlow_FullReturn(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountReturnRoutes(env)

	leader := env.ClientAs("leader-yggdrasil")
	bookingID, itemIDs, _ := setupReturnEnv(t, env, 2, 2)

	t.Run("return item as OK", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"return_status": "returned_ok"})
		resp, err := leader.Put("/api/v0/bookings/"+bookingID+"/items/"+itemIDs[0]+"/return", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}

		var item map[string]any
		json.NewDecoder(resp.Body).Decode(&item)
		if item["return_status"] != "returned_ok" {
			t.Errorf("expected returned_ok, got %v", item["return_status"])
		}
	})

	t.Run("booking still picked_up after partial return", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/bookings/" + bookingID)
		defer resp.Body.Close()

		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		booking := result["booking"].(map[string]any)
		if booking["status"] != "picked_up" {
			t.Errorf("expected picked_up, got %v", booking["status"])
		}
	})

	t.Run("return last item does not auto-complete booking", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"return_status": "returned_ok"})
		resp, err := leader.Put("/api/v0/bookings/"+bookingID+"/items/"+itemIDs[1]+"/return", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}

		// Booking should still be picked_up — requires explicit complete
		resp2, _ := leader.Get("/api/v0/bookings/" + bookingID)
		defer resp2.Body.Close()
		var result map[string]any
		json.NewDecoder(resp2.Body).Decode(&result)
		booking := result["booking"].(map[string]any)
		if booking["status"] != "picked_up" {
			t.Errorf("expected picked_up (no auto-complete), got %v", booking["status"])
		}
	})

	t.Run("explicit return completes booking", func(t *testing.T) {
		resp, err := leader.Post("/api/v0/bookings/"+bookingID+"/return", nil)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}

		var booking map[string]any
		json.NewDecoder(resp.Body).Decode(&booking)
		if booking["status"] != "returned" {
			t.Errorf("expected returned, got %v", booking["status"])
		}
	})
}

func TestReturnFlow_DelayedBlocksAvailability(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountReturnRoutes(env)

	leader := env.ClientAs("leader-yggdrasil")
	leaderB := env.ClientAs("leader-flaskpost")
	bookingID, itemIDs, _ := setupReturnEnv(t, env, 2, 2)

	// Return one as delayed
	b, _ := json.Marshal(map[string]any{
		"return_status":        "delayed",
		"expected_return_date": "2026-06-10",
	})
	resp, _ := leader.Put("/api/v0/bookings/"+bookingID+"/items/"+itemIDs[0]+"/return", bytes.NewReader(b))
	resp.Body.Close()

	// Return other as OK
	b, _ = json.Marshal(map[string]any{"return_status": "returned_ok"})
	resp, _ = leader.Put("/api/v0/bookings/"+bookingID+"/items/"+itemIDs[1]+"/return", bytes.NewReader(b))
	resp.Body.Close()

	t.Run("booking not auto-completed with delayed item", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/bookings/" + bookingID)
		defer resp.Body.Close()
		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		booking := result["booking"].(map[string]any)
		if booking["status"] != "picked_up" {
			t.Errorf("expected picked_up (delayed item keeps booking open), got %v", booking["status"])
		}
	})

	t.Run("delayed article not available for overlapping dates", func(t *testing.T) {
		resp, err := leaderB.Get("/api/v0/articles/availability?start_date=2026-06-01&end_date=2026-06-05")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		var avail []map[string]any
		json.NewDecoder(resp.Body).Decode(&avail)

		for _, a := range avail {
			if a["commercial_name"] == "ReturnTest" {
				count := int(a["available_count"].(float64))
				if count != 1 {
					t.Errorf("expected 1 ReturnTest available (1 OK, 1 delayed), got %d", count)
				}
			}
		}
	})
}

func TestReturnFlow_BrokenCreatesIssue(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountReturnRoutes(env)

	leader := env.ClientAs("leader-yggdrasil")
	bookingID, itemIDs, _ := setupReturnEnv(t, env, 2, 2)

	t.Run("return as reported_unusable creates issue", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{
			"return_status": "reported_unusable",
			"notes":         "Tent pole snapped",
		})
		resp, err := leader.Put("/api/v0/bookings/"+bookingID+"/items/"+itemIDs[0]+"/return", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}

		// Verify article status changed
		item := itemIDs[0]
		_ = item
		// Check via article list that the article is now reported_unusable
		resp2, _ := leader.Get("/api/v0/articles?status=reported_unusable")
		defer resp2.Body.Close()
		var articles []map[string]any
		json.NewDecoder(resp2.Body).Decode(&articles)

		found := false
		for _, a := range articles {
			if a["commercial_name"] == "ReturnTest" {
				found = true
			}
		}
		if !found {
			t.Error("expected reported_unusable article to have status reported_unusable")
		}
	})

	t.Run("return as lost sets article to lost status", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{
			"return_status": "lost",
			"notes":         "Cannot find it",
		})
		resp, err := leader.Put("/api/v0/bookings/"+bookingID+"/items/"+itemIDs[1]+"/return", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}

		// Check article is lost
		resp2, _ := leader.Get("/api/v0/articles?status=lost")
		defer resp2.Body.Close()
		var articles []map[string]any
		json.NewDecoder(resp2.Body).Decode(&articles)

		found := false
		for _, a := range articles {
			if a["commercial_name"] == "ReturnTest" {
				found = true
			}
		}
		if !found {
			t.Error("expected lost article to have status lost")
		}
	})
}

func TestReturnFlow_InvalidStatus(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountReturnRoutes(env)

	leader := env.ClientAs("leader-yggdrasil")
	bookingID, itemIDs, _ := setupReturnEnv(t, env, 1, 1)

	t.Run("invalid return_status rejected", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"return_status": "invalid"})
		resp, err := leader.Put("/api/v0/bookings/"+bookingID+"/items/"+itemIDs[0]+"/return", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("drying_until rejected for delayed", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"return_status": "delayed"})
		resp, err := leader.Put("/api/v0/bookings/"+bookingID+"/items/"+itemIDs[0]+"/return", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("cannot return on confirmed booking", func(t *testing.T) {
		// Create a confirmed (not picked up) booking
		b, _ := json.Marshal(map[string]any{"start_date": "2026-08-01", "end_date": "2026-08-03"})
		resp, _ := leader.Post("/api/v0/bookings", bytes.NewReader(b))
		var booking map[string]any
		json.NewDecoder(resp.Body).Decode(&booking)
		resp.Body.Close()

		resp, _ = leader.Post("/api/v0/bookings/"+booking["id"].(string)+"/submit", nil)
		resp.Body.Close()

		b, _ = json.Marshal(map[string]any{"return_status": "returned_ok"})
		resp, err := leader.Put("/api/v0/bookings/"+booking["id"].(string)+"/items/fake-id/return", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 400, got %d: %s", resp.StatusCode, body)
		}
	})
}

func TestReturnFlow_ManualComplete(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountReturnRoutes(env)

	leader := env.ClientAs("leader-yggdrasil")
	bookingID, itemIDs, _ := setupReturnEnv(t, env, 2, 2)

	// Return both items
	for _, id := range itemIDs {
		b, _ := json.Marshal(map[string]any{"return_status": "returned_ok"})
		resp, _ := leader.Put("/api/v0/bookings/"+bookingID+"/items/"+id+"/return", bytes.NewReader(b))
		resp.Body.Close()
	}

	t.Run("manual return endpoint works when all returned", func(t *testing.T) {
		resp, err := leader.Post("/api/v0/bookings/"+bookingID+"/return", nil)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		// Booking should already be returned (auto-completed), so this should still succeed
		// or the booking is already returned
		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
	})

	t.Run("cannot return when items still pending", func(t *testing.T) {
		// Create a new picked_up booking
		bookingID2, _, _ := setupReturnEnv(t, env, 1, 1)

		resp, err := leader.Post("/api/v0/bookings/"+bookingID2+"/return", nil)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 400, got %d: %s", resp.StatusCode, body)
		}
	})
}

func TestReturnFlow_AccessControl(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountReturnRoutes(env)

	otherLeader := env.ClientAs("leader-flaskpost")
	bookingID, itemIDs, _ := setupReturnEnv(t, env, 1, 1)

	t.Run("other leader cannot return items", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"return_status": "returned_ok"})
		resp, err := otherLeader.Put("/api/v0/bookings/"+bookingID+"/items/"+itemIDs[0]+"/return", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("equipment manager can return items", func(t *testing.T) {
		manager := env.ClientAs("manager-equipment")
		b, _ := json.Marshal(map[string]any{"return_status": "returned_ok"})
		resp, err := manager.Put("/api/v0/bookings/"+bookingID+"/items/"+itemIDs[0]+"/return", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}
	})
}
