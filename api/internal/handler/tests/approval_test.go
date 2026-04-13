package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/ms-utrustning/api/internal/handler"
	"github.com/malarscouterna/ms-utrustning/api/internal/testutil"
)

func TestApprovalFlow(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
		r.Mount("/bookings", (&handler.BookingHandler{Q: env.Queries}).Routes())
		r.Mount("/teams", (&handler.TeamHandler{Q: env.Queries}).Routes())
	})

	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")
	projectLeader := env.ClientAs("project-leader")

	// Setup: get location and category
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

	// Create articles with different approval levels
	createArticle := func(name, approvalLevel string) {
		b, _ := json.Marshal(map[string]any{
			"commercial_name":      name,
			"common_name":          name + " 1",
			"category_id":          catID,
			"location_id":          locID,
			"individually_tracked": true,
			"approval_level":       approvalLevel,
		})
		resp, _ := manager.Post("/api/v0/articles", bytes.NewReader(b))
		resp.Body.Close()
	}

	createArticle("FreeGear", "none")
	// Create multiple LowGear and HighGear so each subtest has availability
	for i := range 10 {
		b, _ := json.Marshal(map[string]any{
			"commercial_name":      "LowGear",
			"common_name":          "LowGear " + string(rune('A'+i)),
			"category_id":          catID,
			"location_id":          locID,
			"individually_tracked": true,
			"approval_level":       "low",
		})
		resp, _ := manager.Post("/api/v0/articles", bytes.NewReader(b))
		resp.Body.Close()
	}
	for i := range 10 {
		b, _ := json.Marshal(map[string]any{
			"commercial_name":      "HighGear",
			"common_name":          "HighGear " + string(rune('A'+i)),
			"category_id":          catID,
			"location_id":          locID,
			"individually_tracked": true,
			"approval_level":       "high",
		})
		resp, _ := manager.Post("/api/v0/articles", bytes.NewReader(b))
		resp.Body.Close()
	}

	// Helper: create booking, add item, submit, return booking status
	dateCounter := 0
	bookAndSubmitWithOpts := func(client *testutil.TestClient, commercialName string, message string, forceApproval bool) (string, string) {
		dateCounter++
		startDate := fmt.Sprintf("2026-%02d-01", dateCounter%12+1)
		endDate := fmt.Sprintf("2026-%02d-05", dateCounter%12+1)
		b, _ := json.Marshal(map[string]any{"start_date": startDate, "end_date": endDate})
		resp, _ := client.Post("/api/v0/bookings", bytes.NewReader(b))
		var booking map[string]any
		json.NewDecoder(resp.Body).Decode(&booking)
		resp.Body.Close()
		bookingID := booking["id"].(string)

		b, _ = json.Marshal(map[string]any{"commercial_name": commercialName, "quantity": 1})
		resp, _ = client.Post("/api/v0/bookings/"+bookingID+"/items", bytes.NewReader(b))
		resp.Body.Close()

		submitBody := map[string]any{}
		if message != "" {
			submitBody["message"] = message
		}
		if forceApproval {
			submitBody["force_approval"] = true
		}
		b, _ = json.Marshal(submitBody)
		resp, _ = client.Post("/api/v0/bookings/"+bookingID+"/submit", bytes.NewReader(b))
		json.NewDecoder(resp.Body).Decode(&booking)
		resp.Body.Close()

		return bookingID, booking["status"].(string)
	}
	bookAndSubmit := func(client *testutil.TestClient, commercialName string) (string, string) {
		return bookAndSubmitWithOpts(client, commercialName, "", false)
	}

	t.Run("none approval auto-confirms for leader", func(t *testing.T) {
		_, status := bookAndSubmit(leader, "FreeGear")
		if status != "confirmed" {
			t.Errorf("expected confirmed, got %s", status)
		}
	})

	t.Run("low approval needs approval for leader", func(t *testing.T) {
		_, status := bookAndSubmit(leader, "LowGear")
		if status != "submitted" {
			t.Errorf("expected submitted, got %s", status)
		}
	})

	t.Run("low approval auto-confirms for project leader", func(t *testing.T) {
		_, status := bookAndSubmit(projectLeader, "LowGear")
		if status != "confirmed" {
			t.Errorf("expected confirmed, got %s", status)
		}
	})

	t.Run("high approval needs approval for project leader", func(t *testing.T) {
		_, status := bookAndSubmit(projectLeader, "HighGear")
		if status != "submitted" {
			t.Errorf("expected submitted, got %s", status)
		}
	})

	t.Run("high approval auto-confirms for manager", func(t *testing.T) {
		_, status := bookAndSubmit(manager, "HighGear")
		if status != "confirmed" {
			t.Errorf("expected confirmed, got %s", status)
		}
	})

	t.Run("manager approves with message", func(t *testing.T) {
		bookingID, _ := bookAndSubmit(leader, "LowGear")

		b, _ := json.Marshal(map[string]any{"message": "Godkänt, lycka till!"})
		resp, err := manager.Post("/api/v0/bookings/"+bookingID+"/approve", bytes.NewReader(b))
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
			t.Errorf("expected confirmed, got %v", booking["status"])
		}

		// Verify events
		resp2, _ := manager.Get("/api/v0/bookings/" + bookingID + "/events")
		defer resp2.Body.Close()
		var events []map[string]any
		json.NewDecoder(resp2.Body).Decode(&events)
		if len(events) < 2 {
			t.Fatalf("expected at least 2 events (submitted + approved), got %d", len(events))
		}
		last := events[len(events)-1]
		if last["event_type"] != "approved" {
			t.Errorf("expected last event to be approved, got %v", last["event_type"])
		}
		if last["message"] != "Godkänt, lycka till!" {
			t.Errorf("expected approval message, got %v", last["message"])
		}
		if last["actor_name"] == nil || last["actor_name"] == "" {
			t.Error("expected actor_name on event")
		}
	})

	t.Run("manager rejects with message reverts to draft", func(t *testing.T) {
		bookingID, _ := bookAndSubmit(leader, "LowGear")

		b, _ := json.Marshal(map[string]any{"message": "Boka färre, vi har inte tillräckligt"})
		resp, err := manager.Post("/api/v0/bookings/"+bookingID+"/reject", bytes.NewReader(b))
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
		if booking["status"] != "draft" {
			t.Errorf("expected draft, got %v", booking["status"])
		}

		// Verify rejection event
		resp2, _ := manager.Get("/api/v0/bookings/" + bookingID + "/events")
		defer resp2.Body.Close()
		var events []map[string]any
		json.NewDecoder(resp2.Body).Decode(&events)
		last := events[len(events)-1]
		if last["event_type"] != "rejected" {
			t.Errorf("expected last event to be rejected, got %v", last["event_type"])
		}
		if last["message"] != "Boka färre, vi har inte tillräckligt" {
			t.Errorf("expected rejection message, got %v", last["message"])
		}
	})

	t.Run("leader can resubmit after rejection", func(t *testing.T) {
		bookingID, _ := bookAndSubmit(leader, "LowGear")

		// Manager rejects
		b, _ := json.Marshal(map[string]any{"message": "Nej"})
		resp, _ := manager.Post("/api/v0/bookings/"+bookingID+"/reject", bytes.NewReader(b))
		resp.Body.Close()

		// Leader resubmits
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
		if booking["status"] != "submitted" {
			t.Errorf("expected submitted after resubmit, got %v", booking["status"])
		}
	})

	t.Run("leader cannot approve", func(t *testing.T) {
		bookingID, _ := bookAndSubmit(leader, "LowGear")

		b, _ := json.Marshal(map[string]any{"message": "I approve myself"})
		resp, err := leader.Post("/api/v0/bookings/"+bookingID+"/approve", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("leader cannot reject", func(t *testing.T) {
		bookingID, _ := bookAndSubmit(leader, "LowGear")

		b, _ := json.Marshal(map[string]any{"message": "nope"})
		resp, err := leader.Post("/api/v0/bookings/"+bookingID+"/reject", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("submit with message creates event", func(t *testing.T) {
		// Create booking with LowGear, submit with a message
		dateCounter++
		startDate := fmt.Sprintf("2027-%02d-01", dateCounter%12+1)
		endDate := fmt.Sprintf("2027-%02d-05", dateCounter%12+1)
		b, _ := json.Marshal(map[string]any{"start_date": startDate, "end_date": endDate})
		resp, _ := leader.Post("/api/v0/bookings", bytes.NewReader(b))
		var booking map[string]any
		json.NewDecoder(resp.Body).Decode(&booking)
		resp.Body.Close()
		bookingID := booking["id"].(string)

		b, _ = json.Marshal(map[string]any{"commercial_name": "LowGear", "quantity": 1})
		resp, _ = leader.Post("/api/v0/bookings/"+bookingID+"/items", bytes.NewReader(b))
		resp.Body.Close()

		b, _ = json.Marshal(map[string]any{"message": "Vi behöver detta för hajk, kort varsel"})
		resp, _ = leader.Post("/api/v0/bookings/"+bookingID+"/submit", bytes.NewReader(b))
		resp.Body.Close()

		resp, _ = manager.Get("/api/v0/bookings/" + bookingID + "/events")
		defer resp.Body.Close()
		var events []map[string]any
		json.NewDecoder(resp.Body).Decode(&events)
		if len(events) != 1 {
			t.Fatalf("expected 1 event, got %d", len(events))
		}
		if events[0]["event_type"] != "submitted" {
			t.Errorf("expected submitted event, got %v", events[0]["event_type"])
		}
		if events[0]["message"] != "Vi behöver detta för hajk, kort varsel" {
			t.Errorf("expected message, got %v", events[0]["message"])
		}
	})

	t.Run("force_approval sends none-items to submitted", func(t *testing.T) {
		_, status := bookAndSubmitWithOpts(leader, "FreeGear", "", true)
		if status != "submitted" {
			t.Errorf("expected submitted with force_approval, got %s", status)
		}
	})

	t.Run("manager can list pending approvals", func(t *testing.T) {
		resp, err := manager.Get("/api/v0/bookings?status=submitted")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}

		var bookings []map[string]any
		json.NewDecoder(resp.Body).Decode(&bookings)
		for _, b := range bookings {
			if b["status"] != "submitted" {
				t.Errorf("expected all bookings to be submitted, got %v", b["status"])
			}
		}
	})

	t.Run("anyone with access can add notes anytime", func(t *testing.T) {
		// Leader submits booking (goes to submitted)
		bookingID, _ := bookAndSubmit(leader, "LowGear")

		// Leader adds a note while waiting for approval
		b, _ := json.Marshal(map[string]any{"message": "Glömde säga — vi behöver hämta tidigt på morgonen"})
		resp, err := leader.Post("/api/v0/bookings/"+bookingID+"/events", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
		}

		// Manager approves
		b, _ = json.Marshal(map[string]any{"message": "OK"})
		resp2, _ := manager.Post("/api/v0/bookings/"+bookingID+"/approve", bytes.NewReader(b))
		resp2.Body.Close()

		// Leader adds another note after approval
		b, _ = json.Marshal(map[string]any{"message": "Tack! Kan vi lägga till en yxa också?"})
		resp3, err := leader.Post("/api/v0/bookings/"+bookingID+"/events", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		defer resp3.Body.Close()
		if resp3.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp3.Body)
			t.Fatalf("expected 201, got %d: %s", resp3.StatusCode, body)
		}

		// Verify events: submitted, note, approved, note
		resp4, _ := manager.Get("/api/v0/bookings/" + bookingID + "/events")
		defer resp4.Body.Close()
		var events []map[string]any
		json.NewDecoder(resp4.Body).Decode(&events)
		if len(events) != 4 {
			t.Fatalf("expected 4 events, got %d", len(events))
		}
		if events[1]["event_type"] != "note" {
			t.Errorf("expected event 1 to be note, got %v", events[1]["event_type"])
		}
		if events[3]["event_type"] != "note" {
			t.Errorf("expected event 3 to be note, got %v", events[3]["event_type"])
		}
	})

	t.Run("empty note rejected", func(t *testing.T) {
		bookingID, _ := bookAndSubmit(leader, "LowGear")
		b, _ := json.Marshal(map[string]any{"message": ""})
		resp, _ := leader.Post("/api/v0/bookings/"+bookingID+"/events", bytes.NewReader(b))
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 for empty note, got %d", resp.StatusCode)
		}
	})

	t.Run("full approval conversation thread", func(t *testing.T) {
		// Leader submits with message
		bookingID, _ := bookAndSubmitWithOpts(leader, "LowGear", "Behöver detta för hajk", false)

		// Manager rejects
		b, _ := json.Marshal(map[string]any{"message": "Boka färre"})
		resp, _ := manager.Post("/api/v0/bookings/"+bookingID+"/reject", bytes.NewReader(b))
		resp.Body.Close()

		// Leader resubmits with response
		b, _ = json.Marshal(map[string]any{"message": "Ändrat, tack för tipset!"})
		resp, _ = leader.Post("/api/v0/bookings/"+bookingID+"/submit", bytes.NewReader(b))
		resp.Body.Close()

		// Manager approves
		b, _ = json.Marshal(map[string]any{"message": "Godkänt!"})
		resp, _ = manager.Post("/api/v0/bookings/"+bookingID+"/approve", bytes.NewReader(b))
		resp.Body.Close()

		// Verify full event thread
		resp, err := manager.Get("/api/v0/bookings/" + bookingID + "/events")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		var events []map[string]any
		json.NewDecoder(resp.Body).Decode(&events)
		if len(events) != 4 {
			t.Fatalf("expected 4 events (submit, reject, resubmit, approve), got %d", len(events))
		}

		expected := []struct {
			eventType string
			message   string
		}{
			{"submitted", "Behöver detta för hajk"},
			{"rejected", "Boka färre"},
			{"submitted", "Ändrat, tack för tipset!"},
			{"approved", "Godkänt!"},
		}
		for i, exp := range expected {
			if events[i]["event_type"] != exp.eventType {
				t.Errorf("event %d: expected type %q, got %q", i, exp.eventType, events[i]["event_type"])
			}
			if events[i]["message"] != exp.message {
				t.Errorf("event %d: expected message %q, got %q", i, exp.message, events[i]["message"])
			}
			if events[i]["actor_name"] == nil || events[i]["actor_name"] == "" {
				t.Errorf("event %d: expected actor_name", i)
			}
		}
	})
}
