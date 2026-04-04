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

func TestBookingFlow_FullLifecycle(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
		r.Mount("/bookings", (&handler.BookingHandler{Q: env.Queries}).Routes())
	})

	manager := env.ClientAs("equipment-manager")
	leader := env.ClientAs("leader-yggdrasil")

	// Setup: get location and category, create some articles
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

	// Create 3 Sibley tents
	for i := range 3 {
		body := map[string]any{
			"commercial_name":      "Sibley",
			"common_name":          "Sibley " + string(rune('1'+i)),
			"category_id":          catID,
			"location_id":          locID,
			"individually_tracked": true,
		}
		b, _ := json.Marshal(body)
		resp, _ := manager.Post("/api/v0/articles", bytes.NewReader(b))
		resp.Body.Close()
	}

	var bookingID string

	t.Run("check availability", func(t *testing.T) {
		resp, err := leader.Get("/api/v0/articles/availability?start_date=2026-06-01&end_date=2026-06-05")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}

		var avail []map[string]any
		json.NewDecoder(resp.Body).Decode(&avail)

		found := false
		for _, a := range avail {
			if a["commercial_name"] == "Sibley" {
				if int(a["available_count"].(float64)) != 3 {
					t.Errorf("expected 3 Sibley available, got %v", a["available_count"])
				}
				found = true
			}
		}
		if !found {
			t.Error("Sibley not found in availability")
		}
	})

	t.Run("create draft booking", func(t *testing.T) {
		body := map[string]any{
			"start_date": "2026-06-01",
			"end_date":   "2026-06-05",
			"notes":      "Hajk med Yggdrasil",
		}
		b, _ := json.Marshal(body)
		resp, err := leader.Post("/api/v0/bookings", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
		}

		var booking map[string]any
		json.NewDecoder(resp.Body).Decode(&booking)
		bookingID = booking["id"].(string)

		if booking["status"] != "draft" {
			t.Errorf("expected draft, got %v", booking["status"])
		}
	})

	t.Run("add 2 Sibley tents to booking", func(t *testing.T) {
		body := map[string]any{
			"commercial_name": "Sibley",
			"quantity":        2,
		}
		b, _ := json.Marshal(body)
		resp, err := leader.Post("/api/v0/bookings/"+bookingID+"/items", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
		}

		var items []map[string]any
		json.NewDecoder(resp.Body).Decode(&items)
		if len(items) != 2 {
			t.Fatalf("expected 2 items added, got %d", len(items))
		}
	})

	t.Run("availability shows 1 Sibley remaining", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/articles/availability?start_date=2026-06-01&end_date=2026-06-05")
		defer resp.Body.Close()

		var avail []map[string]any
		json.NewDecoder(resp.Body).Decode(&avail)

		for _, a := range avail {
			if a["commercial_name"] == "Sibley" {
				if int(a["available_count"].(float64)) != 1 {
					t.Errorf("expected 1 Sibley available after booking 2, got %v", a["available_count"])
				}
			}
		}
	})

	t.Run("submit booking auto-confirms (no approval needed)", func(t *testing.T) {
		resp, err := leader.Post("/api/v0/bookings/"+bookingID+"/submit", nil)
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
		if booking["status"] != "confirmed" {
			t.Errorf("expected confirmed (no approval needed), got %v", booking["status"])
		}
	})

	t.Run("get booking shows items with article details", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/bookings/" + bookingID)
		defer resp.Body.Close()

		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)

		items := result["items"].([]any)
		if len(items) != 2 {
			t.Fatalf("expected 2 items, got %d", len(items))
		}

		item := items[0].(map[string]any)
		if item["commercial_name"] == nil {
			t.Error("expected commercial_name on booking item")
		}
	})
}

func TestBookingFlow_NoDoubleBooking(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
		r.Mount("/bookings", (&handler.BookingHandler{Q: env.Queries}).Routes())
	})

	manager := env.ClientAs("equipment-manager")
	leaderA := env.ClientAs("leader-yggdrasil")
	leaderB := env.ClientAs("leader-spindlarna")

	// Setup: create 2 Sibley tents
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

	for i := range 2 {
		body := map[string]any{
			"commercial_name":      "Sibley",
			"common_name":          "Sibley " + string(rune('1'+i)),
			"category_id":          catID,
			"location_id":          locID,
			"individually_tracked": true,
		}
		b, _ := json.Marshal(body)
		resp, _ := manager.Post("/api/v0/articles", bytes.NewReader(b))
		resp.Body.Close()
	}

	// Leader A books 2 Sibley for June 5-8
	body := map[string]any{"start_date": "2026-06-05", "end_date": "2026-06-08"}
	b, _ := json.Marshal(body)
	resp, _ = leaderA.Post("/api/v0/bookings", bytes.NewReader(b))
	var bookingA map[string]any
	json.NewDecoder(resp.Body).Decode(&bookingA)
	resp.Body.Close()

	b, _ = json.Marshal(map[string]any{"commercial_name": "Sibley", "quantity": 2})
	resp, _ = leaderA.Post("/api/v0/bookings/"+bookingA["id"].(string)+"/items", bytes.NewReader(b))
	resp.Body.Close()

	// Leader B tries to book 1 Sibley for overlapping dates
	body = map[string]any{"start_date": "2026-06-07", "end_date": "2026-06-10"}
	b, _ = json.Marshal(body)
	resp, _ = leaderB.Post("/api/v0/bookings", bytes.NewReader(b))
	var bookingB map[string]any
	json.NewDecoder(resp.Body).Decode(&bookingB)
	resp.Body.Close()

	t.Run("leader B cannot book Sibley for overlapping dates", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"commercial_name": "Sibley", "quantity": 1})
		resp, err := leaderB.Post("/api/v0/bookings/"+bookingB["id"].(string)+"/items", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusConflict {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 409 conflict, got %d: %s", resp.StatusCode, body)
		}
	})

	t.Run("leader B can book Sibley for non-overlapping dates", func(t *testing.T) {
		body := map[string]any{"start_date": "2026-06-10", "end_date": "2026-06-12"}
		b, _ := json.Marshal(body)
		resp, _ := leaderB.Post("/api/v0/bookings", bytes.NewReader(b))
		var bookingC map[string]any
		json.NewDecoder(resp.Body).Decode(&bookingC)
		resp.Body.Close()

		b, _ = json.Marshal(map[string]any{"commercial_name": "Sibley", "quantity": 1})
		resp, err := leaderB.Post("/api/v0/bookings/"+bookingC["id"].(string)+"/items", bytes.NewReader(b))
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

func TestBookingFlow_UpdateConfirmedBooking(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
		r.Mount("/bookings", (&handler.BookingHandler{Q: env.Queries}).Routes())
	})

	manager := env.ClientAs("equipment-manager")
	leader := env.ClientAs("leader-yggdrasil")

	// Setup: create 3 Sibley tents
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

	for i := range 3 {
		body := map[string]any{
			"commercial_name":      "Sibley",
			"common_name":          "Sibley " + string(rune('1'+i)),
			"category_id":          catID,
			"location_id":          locID,
			"individually_tracked": true,
		}
		b, _ := json.Marshal(body)
		resp, _ := manager.Post("/api/v0/articles", bytes.NewReader(b))
		resp.Body.Close()
	}

	// Create and confirm a booking with 2 Sibley
	b, _ := json.Marshal(map[string]any{"start_date": "2026-07-01", "end_date": "2026-07-05"})
	resp, _ = leader.Post("/api/v0/bookings", bytes.NewReader(b))
	var booking map[string]any
	json.NewDecoder(resp.Body).Decode(&booking)
	resp.Body.Close()
	bookingID := booking["id"].(string)

	b, _ = json.Marshal(map[string]any{"commercial_name": "Sibley", "quantity": 2})
	resp, _ = leader.Post("/api/v0/bookings/"+bookingID+"/items", bytes.NewReader(b))
	resp.Body.Close()

	resp, _ = leader.Post("/api/v0/bookings/"+bookingID+"/submit", nil)
	resp.Body.Close()

	t.Run("update notes on confirmed booking", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"notes": "Updated notes"})
		resp, err := leader.Put("/api/v0/bookings/"+bookingID, bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}

		var updated map[string]any
		json.NewDecoder(resp.Body).Decode(&updated)
		if updated["notes"] != "Updated notes" {
			t.Errorf("expected updated notes, got %v", updated["notes"])
		}
	})

	t.Run("add item to confirmed booking", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"commercial_name": "Sibley", "quantity": 1})
		resp, err := leader.Post("/api/v0/bookings/"+bookingID+"/items", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
		}
	})

	t.Run("booking now has 3 items", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/bookings/" + bookingID)
		defer resp.Body.Close()

		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		items := result["items"].([]any)
		if len(items) != 3 {
			t.Fatalf("expected 3 items, got %d", len(items))
		}
	})

	t.Run("change dates fails when items not available", func(t *testing.T) {
		// Book all 3 Sibley for July 10-15 with another booking
		b, _ := json.Marshal(map[string]any{"start_date": "2026-07-10", "end_date": "2026-07-15"})
		resp, _ := manager.Post("/api/v0/bookings", bytes.NewReader(b))
		var otherBooking map[string]any
		json.NewDecoder(resp.Body).Decode(&otherBooking)
		resp.Body.Close()

		b, _ = json.Marshal(map[string]any{"commercial_name": "Sibley", "quantity": 3})
		resp, _ = manager.Post("/api/v0/bookings/"+otherBooking["id"].(string)+"/items", bytes.NewReader(b))
		resp.Body.Close()

		// Now try to move our booking to overlap with July 10-15
		b, _ = json.Marshal(map[string]any{"start_date": "2026-07-10", "end_date": "2026-07-15"})
		resp, err := leader.Put("/api/v0/bookings/"+bookingID, bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusConflict {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 409, got %d: %s", resp.StatusCode, body)
		}
	})
}

func TestBookingFlow_AccessControl(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
		r.Mount("/bookings", (&handler.BookingHandler{Q: env.Queries}).Routes())
	})

	leaderYgg := env.ClientAs("leader-yggdrasil")
	leaderOrn := env.ClientAs("leader-spindlarna")

	// Create a personal booking (no unit)
	b, _ := json.Marshal(map[string]any{"start_date": "2026-08-01", "end_date": "2026-08-05"})
	resp, _ := leaderYgg.Post("/api/v0/bookings", bytes.NewReader(b))
	var booking map[string]any
	json.NewDecoder(resp.Body).Decode(&booking)
	resp.Body.Close()
	bookingID := booking["id"].(string)

	t.Run("creator can update own booking", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"notes": "My booking"})
		resp, err := leaderYgg.Put("/api/v0/bookings/"+bookingID, bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}
	})

	t.Run("other leader cannot update booking", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"notes": "Hacked"})
		resp, err := leaderOrn.Put("/api/v0/bookings/"+bookingID, bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("equipment manager can update any booking", func(t *testing.T) {
		manager := env.ClientAs("equipment-manager")
		b, _ := json.Marshal(map[string]any{"notes": "Manager override"})
		resp, err := manager.Put("/api/v0/bookings/"+bookingID, bytes.NewReader(b))
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

func TestBookingFlow_CancelAndDeleteDraft(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
		r.Mount("/bookings", (&handler.BookingHandler{Q: env.Queries}).Routes())
	})

	leader := env.ClientAs("leader-yggdrasil")

	t.Run("delete draft booking", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"start_date": "2026-09-01", "end_date": "2026-09-03"})
		resp, _ := leader.Post("/api/v0/bookings", bytes.NewReader(b))
		var booking map[string]any
		json.NewDecoder(resp.Body).Decode(&booking)
		resp.Body.Close()

		resp, err := leader.Post("/api/v0/bookings/"+booking["id"].(string)+"/cancel", nil)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 204, got %d: %s", resp.StatusCode, body)
		}

		// Verify it's gone
		resp, _ = leader.Get("/api/v0/bookings/" + booking["id"].(string))
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404 after delete, got %d", resp.StatusCode)
		}
	})

	t.Run("cancel confirmed booking", func(t *testing.T) {
		manager := env.ClientAs("equipment-manager")

		// Create an article so we can submit
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
			"commercial_name": "CancelTest", "common_name": "CancelTest 1",
			"category_id": catID, "location_id": locID, "individually_tracked": true,
		})
		resp, _ = manager.Post("/api/v0/articles", bytes.NewReader(b))
		resp.Body.Close()

		// Create, add item, submit
		b, _ = json.Marshal(map[string]any{"start_date": "2026-09-10", "end_date": "2026-09-12"})
		resp, _ = leader.Post("/api/v0/bookings", bytes.NewReader(b))
		var booking map[string]any
		json.NewDecoder(resp.Body).Decode(&booking)
		resp.Body.Close()
		bookingID := booking["id"].(string)

		b, _ = json.Marshal(map[string]any{"commercial_name": "CancelTest", "quantity": 1})
		resp, _ = leader.Post("/api/v0/bookings/"+bookingID+"/items", bytes.NewReader(b))
		resp.Body.Close()

		resp, _ = leader.Post("/api/v0/bookings/"+bookingID+"/submit", nil)
		resp.Body.Close()

		// Cancel it
		resp, err := leader.Post("/api/v0/bookings/"+bookingID+"/cancel", nil)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}

		var cancelled map[string]any
		json.NewDecoder(resp.Body).Decode(&cancelled)
		if cancelled["status"] != "cancelled" {
			t.Errorf("expected cancelled, got %v", cancelled["status"])
		}
	})
}

func TestBookingFlow_IncrementalAddNoDuplicates(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
		r.Mount("/bookings", (&handler.BookingHandler{Q: env.Queries}).Routes())
	})

	manager := env.ClientAs("equipment-manager")
	leader := env.ClientAs("leader-yggdrasil")

	// Setup: create 5 items of same type
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

	for i := range 5 {
		b, _ := json.Marshal(map[string]any{
			"commercial_name":      "Handskar",
			"common_name":          "Handskar " + string(rune('1'+i)),
			"category_id":          catID,
			"location_id":          locID,
			"individually_tracked": true,
		})
		resp, _ := manager.Post("/api/v0/articles", bytes.NewReader(b))
		resp.Body.Close()
	}

	// Create booking
	b, _ := json.Marshal(map[string]any{"start_date": "2026-10-01", "end_date": "2026-10-03"})
	resp, _ = leader.Post("/api/v0/bookings", bytes.NewReader(b))
	var booking map[string]any
	json.NewDecoder(resp.Body).Decode(&booking)
	resp.Body.Close()
	bookingID := booking["id"].(string)

	// Add 2, then 2 more, then 1 more — should get 5 unique articles
	for _, qty := range []int{2, 2, 1} {
		b, _ := json.Marshal(map[string]any{"commercial_name": "Handskar", "quantity": qty})
		resp, err := leader.Post("/api/v0/bookings/"+bookingID+"/items", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
		}
		resp.Body.Close()
	}

	// Verify 5 unique items
	resp, _ = leader.Get("/api/v0/bookings/" + bookingID)
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()

	items := result["items"].([]any)
	if len(items) != 5 {
		t.Fatalf("expected 5 items, got %d", len(items))
	}

	// Verify all article IDs are unique
	seen := map[string]bool{}
	for _, item := range items {
		aid := item.(map[string]any)["article_id"].(string)
		if seen[aid] {
			t.Errorf("duplicate article_id: %s", aid)
		}
		seen[aid] = true
	}

	// Verify availability is now 0
	resp, _ = leader.Get("/api/v0/articles/availability?start_date=2026-10-01&end_date=2026-10-03")
	var avail []map[string]any
	json.NewDecoder(resp.Body).Decode(&avail)
	resp.Body.Close()

	for _, a := range avail {
		if a["commercial_name"] == "Handskar" {
			t.Errorf("expected Handskar to not appear in availability (0 left), but got %v", a["available_count"])
		}
	}

	// Adding 1 more should fail
	b, _ = json.Marshal(map[string]any{"commercial_name": "Handskar", "quantity": 1})
	resp, _ = leader.Post("/api/v0/bookings/"+bookingID+"/items", bytes.NewReader(b))
	if resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 409 when all booked, got %d: %s", resp.StatusCode, body)
	}
	resp.Body.Close()
}

func TestBookingFlow_LocationScopedAvailability(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
		r.Mount("/bookings", (&handler.BookingHandler{Q: env.Queries}).Routes())
	})

	manager := env.ClientAs("equipment-manager")
	leader := env.ClientAs("leader-yggdrasil")

	// Get seed location and category
	resp, _ := manager.Get("/api/v0/locations")
	var locations []map[string]any
	json.NewDecoder(resp.Body).Decode(&locations)
	resp.Body.Close()

	// Need two locations — seed has multiple
	if len(locations) < 2 {
		t.Skip("need at least 2 seed locations")
	}
	loc1ID := locations[0]["id"].(string)
	loc1Name := locations[0]["name"].(string)
	loc2ID := locations[1]["id"].(string)
	loc2Name := locations[1]["name"].(string)

	resp, _ = manager.Get("/api/v0/categories")
	var categories []map[string]any
	json.NewDecoder(resp.Body).Decode(&categories)
	resp.Body.Close()
	catID := categories[0]["id"].(string)

	// Create 2 "Flaska" in location 1, 3 "Flaska" in location 2
	for i := range 2 {
		b, _ := json.Marshal(map[string]any{
			"commercial_name": "Flaska", "common_name": "Flaska L1-" + string(rune('1'+i)),
			"category_id": catID, "location_id": loc1ID, "individually_tracked": true,
		})
		resp, _ := manager.Post("/api/v0/articles", bytes.NewReader(b))
		resp.Body.Close()
	}
	for i := range 3 {
		b, _ := json.Marshal(map[string]any{
			"commercial_name": "Flaska", "common_name": "Flaska L2-" + string(rune('1'+i)),
			"category_id": catID, "location_id": loc2ID, "individually_tracked": true,
		})
		resp, _ := manager.Post("/api/v0/articles", bytes.NewReader(b))
		resp.Body.Close()
	}

	t.Run("availability shows two groups for same commercial_name", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/articles/availability?start_date=2026-11-01&end_date=2026-11-03")
		defer resp.Body.Close()

		var avail []map[string]any
		json.NewDecoder(resp.Body).Decode(&avail)

		flaskaGroups := 0
		for _, a := range avail {
			if a["commercial_name"] == "Flaska" {
				flaskaGroups++
				locName := a["location_name"].(string)
				count := int(a["available_count"].(float64))
				if locName == loc1Name && count != 2 {
					t.Errorf("expected 2 Flaska in %s, got %d", loc1Name, count)
				}
				if locName == loc2Name && count != 3 {
					t.Errorf("expected 3 Flaska in %s, got %d", loc2Name, count)
				}
			}
		}
		if flaskaGroups != 2 {
			t.Errorf("expected 2 Flaska groups (one per location), got %d", flaskaGroups)
		}
	})

	t.Run("booking from location 1 does not affect location 2 availability", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"start_date": "2026-11-01", "end_date": "2026-11-03"})
		resp, _ := leader.Post("/api/v0/bookings", bytes.NewReader(b))
		var booking map[string]any
		json.NewDecoder(resp.Body).Decode(&booking)
		resp.Body.Close()
		bookingID := booking["id"].(string)

		// Book 2 Flaska from location 1
		b, _ = json.Marshal(map[string]any{
			"commercial_name": "Flaska", "location_name": loc1Name, "quantity": 2,
		})
		resp, err := leader.Post("/api/v0/bookings/"+bookingID+"/items", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()

		// Check availability — location 1 should have 0, location 2 still 3
		resp, _ = leader.Get("/api/v0/articles/availability?start_date=2026-11-01&end_date=2026-11-03")
		defer resp.Body.Close()

		var avail []map[string]any
		json.NewDecoder(resp.Body).Decode(&avail)

		for _, a := range avail {
			if a["commercial_name"] == "Flaska" {
				locName := a["location_name"].(string)
				count := int(a["available_count"].(float64))
				if locName == loc1Name {
					t.Errorf("expected Flaska in %s to not appear (0 available), but got %d", loc1Name, count)
				}
				if locName == loc2Name && count != 3 {
					t.Errorf("expected 3 Flaska in %s, got %d", loc2Name, count)
				}
			}
		}
	})
}

func TestBookingFlow_Copy(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
		r.Mount("/bookings", (&handler.BookingHandler{Q: env.Queries}).Routes())
	})

	manager := env.ClientAs("equipment-manager")
	leader := env.ClientAs("leader-yggdrasil")

	// Setup
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

	for i := range 3 {
		b, _ := json.Marshal(map[string]any{
			"commercial_name": "CopyTest", "common_name": "CopyTest " + string(rune('1'+i)),
			"category_id": catID, "location_id": locID, "individually_tracked": true,
		})
		resp, _ := manager.Post("/api/v0/articles", bytes.NewReader(b))
		resp.Body.Close()
	}

	// Create booking with 2 items, submit, then cancel
	b, _ := json.Marshal(map[string]any{"start_date": "2026-12-01", "end_date": "2026-12-03", "notes": "Original"})
	resp, _ = leader.Post("/api/v0/bookings", bytes.NewReader(b))
	var booking map[string]any
	json.NewDecoder(resp.Body).Decode(&booking)
	resp.Body.Close()
	bookingID := booking["id"].(string)

	b, _ = json.Marshal(map[string]any{"commercial_name": "CopyTest", "quantity": 2})
	resp, _ = leader.Post("/api/v0/bookings/"+bookingID+"/items", bytes.NewReader(b))
	resp.Body.Close()

	resp, _ = leader.Post("/api/v0/bookings/"+bookingID+"/submit", nil)
	resp.Body.Close()

	resp, _ = leader.Post("/api/v0/bookings/"+bookingID+"/cancel", nil)
	resp.Body.Close()

	t.Run("copy cancelled booking creates new draft with same items", func(t *testing.T) {
		resp, err := leader.Post("/api/v0/bookings/"+bookingID+"/copy", nil)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
		}

		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)

		copied := int(result["items_copied"].(float64))
		if copied != 2 {
			t.Errorf("expected 2 items copied, got %d", copied)
		}

		newBooking := result["booking"].(map[string]any)
		if newBooking["status"] != "draft" {
			t.Errorf("expected draft, got %v", newBooking["status"])
		}
		if newBooking["notes"] != "Original" {
			t.Errorf("expected notes preserved, got %v", newBooking["notes"])
		}

		// Verify the new booking has items
		newID := newBooking["id"].(string)
		resp2, _ := leader.Get("/api/v0/bookings/" + newID)
		var detail map[string]any
		json.NewDecoder(resp2.Body).Decode(&detail)
		resp2.Body.Close()

		items := detail["items"].([]any)
		if len(items) != 2 {
			t.Errorf("expected 2 items in copy, got %d", len(items))
		}
	})
}
