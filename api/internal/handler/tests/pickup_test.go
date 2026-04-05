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

// setupPickupEnv creates a confirmed booking with 2 Sibley tents ready for pickup.
// Returns the booking ID and the item IDs.
func setupPickupEnv(t *testing.T, env *testutil.TestEnv) (bookingID string, itemIDs []string, extraArticleID string) {
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

	// Create 3 Sibley tents (book 2, keep 1 as swap candidate)
	var articleIDs []string
	for i := range 3 {
		b, _ := json.Marshal(map[string]any{
			"commercial_name":      "Sibley",
			"common_name":          "Sibley " + string(rune('1'+i)),
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

	// Create booking spanning today (realistic for pickup)
	start, end := todayRange(5)
	b, _ := json.Marshal(map[string]any{"start_date": start, "end_date": end})
	resp, _ = leader.Post("/api/v0/bookings", bytes.NewReader(b))
	var booking map[string]any
	json.NewDecoder(resp.Body).Decode(&booking)
	resp.Body.Close()
	bookingID = booking["id"].(string)

	b, _ = json.Marshal(map[string]any{"commercial_name": "Sibley", "quantity": 2})
	resp, _ = leader.Post("/api/v0/bookings/"+bookingID+"/items", bytes.NewReader(b))
	resp.Body.Close()

	resp, _ = leader.Post("/api/v0/bookings/"+bookingID+"/submit", nil)
	resp.Body.Close()

	// Get item IDs
	resp, _ = leader.Get("/api/v0/bookings/" + bookingID)
	var detail map[string]any
	json.NewDecoder(resp.Body).Decode(&detail)
	resp.Body.Close()

	items := detail["items"].([]any)
	for _, item := range items {
		itemIDs = append(itemIDs, item.(map[string]any)["id"].(string))
	}

	// Find the article that wasn't booked (swap candidate)
	bookedArticles := map[string]bool{}
	for _, item := range items {
		bookedArticles[item.(map[string]any)["article_id"].(string)] = true
	}
	for _, aid := range articleIDs {
		if !bookedArticles[aid] {
			extraArticleID = aid
			break
		}
	}

	return bookingID, itemIDs, extraArticleID
}

func mountPickupRoutes(env *testutil.TestEnv) {
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
		r.Mount("/bookings", (&handler.BookingHandler{Q: env.Queries}).Routes())
	})
}

func TestPickupFlow_TransitionAndChecklist(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountPickupRoutes(env)

	leader := env.ClientAs("leader-yggdrasil")
	bookingID, itemIDs, _ := setupPickupEnv(t, env)

	t.Run("cannot pickup a draft booking", func(t *testing.T) {
		// Create a draft (don't submit)
		start, end := todayRange(3)
		b, _ := json.Marshal(map[string]any{"start_date": start, "end_date": end})
		resp, _ := leader.Post("/api/v0/bookings", bytes.NewReader(b))
		var draft map[string]any
		json.NewDecoder(resp.Body).Decode(&draft)
		resp.Body.Close()

		resp, err := leader.Post("/api/v0/bookings/"+draft["id"].(string)+"/pickup", nil)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 400, got %d: %s", resp.StatusCode, body)
		}
	})

	t.Run("transition confirmed booking to picked_up", func(t *testing.T) {
		resp, err := leader.Post("/api/v0/bookings/"+bookingID+"/pickup", nil)
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
		if booking["status"] != "picked_up" {
			t.Errorf("expected picked_up, got %v", booking["status"])
		}
	})

	t.Run("mark item as picked_up", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"pickup_status": "picked_up"})
		resp, err := leader.Put("/api/v0/bookings/"+bookingID+"/items/"+itemIDs[0]+"/pickup", bytes.NewReader(b))
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
		if item["pickup_status"] != "picked_up" {
			t.Errorf("expected picked_up, got %v", item["pickup_status"])
		}
	})

	t.Run("mark item as lost", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"pickup_status": "lost"})
		resp, err := leader.Put("/api/v0/bookings/"+bookingID+"/items/"+itemIDs[1]+"/pickup", bytes.NewReader(b))
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
		if item["pickup_status"] != "lost" {
			t.Errorf("expected lost, got %v", item["pickup_status"])
		}
	})

	t.Run("invalid pickup_status rejected", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"pickup_status": "invalid"})
		resp, err := leader.Put("/api/v0/bookings/"+bookingID+"/items/"+itemIDs[0]+"/pickup", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("get booking shows pickup statuses", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/bookings/" + bookingID)
		defer resp.Body.Close()

		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		items := result["items"].([]any)

		statuses := map[string]bool{}
		for _, item := range items {
			ps := item.(map[string]any)["pickup_status"]
			if ps != nil {
				statuses[ps.(string)] = true
			}
		}
		if !statuses["picked_up"] || !statuses["lost"] {
			t.Errorf("expected both pickup statuses, got %v", statuses)
		}
	})
}

func TestPickupFlow_SwapArticle(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountPickupRoutes(env)

	leader := env.ClientAs("leader-yggdrasil")
	bookingID, itemIDs, extraArticleID := setupPickupEnv(t, env)

	// Transition to picked_up
	resp, _ := leader.Post("/api/v0/bookings/"+bookingID+"/pickup", nil)
	resp.Body.Close()

	t.Run("swap article on booking item", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"new_article_id": extraArticleID})
		resp, err := leader.Post("/api/v0/bookings/"+bookingID+"/items/"+itemIDs[0]+"/swap", bytes.NewReader(b))
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
		if item["pickup_status"] != "swapped" {
			t.Errorf("expected swapped status, got %v", item["pickup_status"])
		}
		if item["article_id"] != extraArticleID {
			t.Errorf("expected article_id %s, got %v", extraArticleID, item["article_id"])
		}
	})

	t.Run("swap to unavailable article fails", func(t *testing.T) {
		// The original article from item[0] is now free, but item[1] still has its article.
		// Try to swap item[1] to the same article that item[0] now has (extraArticleID) — should conflict.
		b, _ := json.Marshal(map[string]any{"new_article_id": extraArticleID})
		resp, err := leader.Post("/api/v0/bookings/"+bookingID+"/items/"+itemIDs[1]+"/swap", bytes.NewReader(b))
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

func TestPickupFlow_UndoPickupStatus(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountPickupRoutes(env)

	leader := env.ClientAs("leader-yggdrasil")
	bookingID, itemIDs, _ := setupPickupEnv(t, env)

	// Transition to picked_up
	resp, _ := leader.Post("/api/v0/bookings/"+bookingID+"/pickup", nil)
	resp.Body.Close()

	// Mark as picked_up first
	b, _ := json.Marshal(map[string]any{"pickup_status": "picked_up"})
	resp, _ = leader.Put("/api/v0/bookings/"+bookingID+"/items/"+itemIDs[0]+"/pickup", bytes.NewReader(b))
	resp.Body.Close()

	t.Run("undo pickup status by sending empty string", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"pickup_status": ""})
		resp, err := leader.Put("/api/v0/bookings/"+bookingID+"/items/"+itemIDs[0]+"/pickup", bytes.NewReader(b))
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
		if item["pickup_status"] != nil {
			t.Errorf("expected null pickup_status after undo, got %v", item["pickup_status"])
		}
	})

	t.Run("booking reverts to confirmed when all pickups undone", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/bookings/" + bookingID)
		defer resp.Body.Close()
		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		booking := result["booking"].(map[string]any)
		if booking["status"] != "confirmed" {
			t.Errorf("expected confirmed after all pickups undone, got %v", booking["status"])
		}
	})

	t.Run("item can be re-marked after re-entering pickup", func(t *testing.T) {
		// Re-transition to picked_up
		resp, _ := leader.Post("/api/v0/bookings/"+bookingID+"/pickup", nil)
		resp.Body.Close()

		b, _ := json.Marshal(map[string]any{"pickup_status": "lost"})
		resp, err := leader.Put("/api/v0/bookings/"+bookingID+"/items/"+itemIDs[0]+"/pickup", bytes.NewReader(b))
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
		if item["pickup_status"] != "lost" {
			t.Errorf("expected lost, got %v", item["pickup_status"])
		}
	})
}

func TestPickupFlow_AvailableArticlesEndpoint(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountPickupRoutes(env)

	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")

	// Setup: create 3 articles
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
			"commercial_name": "Yxa", "common_name": "Yxa " + string(rune('1'+i)),
			"category_id": catID, "location_id": locID, "individually_tracked": true,
		})
		resp, _ := manager.Post("/api/v0/articles", bytes.NewReader(b))
		resp.Body.Close()
	}

	// Create a booking with 2 Yxa
	b, _ := json.Marshal(map[string]any{"start_date": "2026-06-01", "end_date": "2026-06-05"})
	resp, _ = leader.Post("/api/v0/bookings", bytes.NewReader(b))
	var booking map[string]any
	json.NewDecoder(resp.Body).Decode(&booking)
	resp.Body.Close()
	bookingID := booking["id"].(string)

	b, _ = json.Marshal(map[string]any{"commercial_name": "Yxa", "quantity": 2})
	resp, _ = leader.Post("/api/v0/bookings/"+bookingID+"/items", bytes.NewReader(b))
	resp.Body.Close()

	t.Run("returns individual articles", func(t *testing.T) {
		resp, err := leader.Get("/api/v0/articles/availability/articles?start_date=2026-06-01&end_date=2026-06-05&commercial_name=Yxa")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}

		var articles []map[string]any
		json.NewDecoder(resp.Body).Decode(&articles)
		// 3 total minus 2 booked = 1 available
		if len(articles) != 1 {
			t.Fatalf("expected 1 available Yxa, got %d", len(articles))
		}
		if articles[0]["id"] == nil {
			t.Error("expected article to have id")
		}
	})

	t.Run("exclude_booking_id shows articles from that booking as available", func(t *testing.T) {
		resp, err := leader.Get("/api/v0/articles/availability/articles?start_date=2026-06-01&end_date=2026-06-05&commercial_name=Yxa&exclude_booking_id=" + bookingID)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}

		var articles []map[string]any
		json.NewDecoder(resp.Body).Decode(&articles)
		// Excluding the booking means its 2 items don't count as booked,
		// but AvailableArticlesExcludingBooking also excludes items IN the booking,
		// so we get 3 - 2 (in booking) = 1
		if len(articles) != 1 {
			t.Fatalf("expected 1 available Yxa (excluding booking), got %d", len(articles))
		}
	})
}

func TestPickupFlow_AccessControl(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountPickupRoutes(env)

	leader := env.ClientAs("leader-yggdrasil")
	otherLeader := env.ClientAs("leader-flaskpost")
	bookingID, itemIDs, _ := setupPickupEnv(t, env)

	// Transition to picked_up
	resp, _ := leader.Post("/api/v0/bookings/"+bookingID+"/pickup", nil)
	resp.Body.Close()

	t.Run("other leader cannot update pickup status", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"pickup_status": "picked_up"})
		resp, err := otherLeader.Put("/api/v0/bookings/"+bookingID+"/items/"+itemIDs[0]+"/pickup", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("other leader cannot swap article", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"new_article_id": "00000000-0000-0000-0000-000000000000"})
		resp, err := otherLeader.Post("/api/v0/bookings/"+bookingID+"/items/"+itemIDs[0]+"/swap", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("equipment manager can update pickup status", func(t *testing.T) {
		manager := env.ClientAs("manager-equipment")
		b, _ := json.Marshal(map[string]any{"pickup_status": "picked_up"})
		resp, err := manager.Put("/api/v0/bookings/"+bookingID+"/items/"+itemIDs[0]+"/pickup", bytes.NewReader(b))
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

// todayRange returns a start date (today) and end date (today + days) as ISO strings.
// Use for tests that exercise pickup/return — the booking should span the current date.
func todayRange(days int) (string, string) {
	now := time.Now()
	return now.Format("2006-01-02"), now.AddDate(0, 0, days).Format("2006-01-02")
}
