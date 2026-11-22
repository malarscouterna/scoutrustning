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

func setupBookingEditEnv(t *testing.T) (*testutil.TestClient, *testutil.TestClient) {
	env := testutil.SetupTestEnv(t)
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries, Perms: handler.NewPermissionCache(env.Queries)}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
		r.Mount("/bookings", (&handler.BookingHandler{Q: env.Queries, Pool: env.Pool, Perms: handler.NewPermissionCache(env.Queries)}).Routes())
		r.Mount("/teams", (&handler.TeamHandler{Q: env.Queries}).Routes())
	})

	manager := env.ClientAs("manager-equipment")
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

	// Two Stormkök stoves, approval level none - submit auto-confirms unless force_approval.
	for i := range 2 {
		body := map[string]any{
			"commercial_name":      "Stormkök",
			"common_name":          "Stormkök " + string(rune('1'+i)),
			"category_id":          catID,
			"location_id":          locID,
			"individually_tracked": true,
		}
		b, _ := json.Marshal(body)
		resp, _ := manager.Post("/api/v0/articles", bytes.NewReader(b))
		resp.Body.Close()
	}

	leader := env.ClientAs("leader-yggdrasil")
	return manager, leader
}

func createTeamBooking(t *testing.T, leader *testutil.TestClient, forceApproval bool, startDate, endDate, title string) (string, *map[string]any) {
	teamID := getTeamID(t, leader, "Yggdrasil")
	body := map[string]any{
		"start_date":      startDate,
		"end_date":        endDate,
		"title":           title,
		"used_by_team_id": teamID,
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
	id := booking["id"].(string)

	itemBody := map[string]any{"commercial_name": "Stormkök", "quantity": 1}
	ib, _ := json.Marshal(itemBody)
	resp, err = leader.Post("/api/v0/bookings/"+id+"/items", bytes.NewReader(ib))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201 adding item, got %d: %s", resp.StatusCode, body)
	}

	submitBody := map[string]any{"force_approval": forceApproval}
	sb, _ := json.Marshal(submitBody)
	resp, err = leader.Post("/api/v0/bookings/"+id+"/submit", bytes.NewReader(sb))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 submitting, got %d: %s", resp.StatusCode, body)
	}
	return id, &booking
}

func getBookingStatus(t *testing.T, client *testutil.TestClient, id string) string {
	resp, err := client.Get("/api/v0/bookings/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	var out struct {
		Booking map[string]any `json:"booking"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	return out.Booking["status"].(string)
}

func expectStatus(t *testing.T, resp *http.Response, want int) []byte {
	t.Helper()
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != want {
		t.Fatalf("expected %d, got %d: %s", want, resp.StatusCode, body)
	}
	return body
}

// Editing a submitted/approved/confirmed booking returns it to draft: items stay
// reserved, the booking must be resubmitted, and the reopening is event-logged.
func TestBookingEdit_ReturnsToDraft(t *testing.T) {
	manager, leader := setupBookingEditEnv(t)

	// Submitted booking (forced approval).
	bookingID, _ := createTeamBooking(t, leader, true, "2026-07-01", "2026-07-05", "Sommarläger")
	if got := getBookingStatus(t, leader, bookingID); got != "submitted" {
		t.Fatalf("expected submitted, got %s", got)
	}

	// Reservation holds while the booking is being edited: the stove must not
	// show as available for an overlapping range.
	assertAvailability := func(want float64, msg string) {
		t.Helper()
		resp, _ := leader.Get("/api/v0/articles/availability?start_date=2026-07-01&end_date=2026-07-05")
		avail := expectStatus(t, resp, http.StatusOK)
		var groups []map[string]any
		json.NewDecoder(bytes.NewReader(avail)).Decode(&groups)
		var count float64 = -1
		for _, g := range groups {
			if g["commercial_name"] == "Stormkök" {
				count = g["available_count"].(float64)
			}
		}
		if count != want {
			t.Fatalf("%s: expected %v Stormkök available, got %v", msg, want, count)
		}
	}
	assertAvailability(1, "before edit")

	// Editing the details downgrades the booking to draft.
	teamID := getTeamID(t, leader, "Yggdrasil")
	updateBody := map[string]any{"title": "Sommarläger (ändrad)", "used_by_team_id": teamID}
	ub, _ := json.Marshal(updateBody)
	resp, err := leader.Put("/api/v0/bookings/"+bookingID, bytes.NewReader(ub))
	if err != nil {
		t.Fatal(err)
	}
	expectStatus(t, resp, http.StatusOK)

	if got := getBookingStatus(t, leader, bookingID); got != "draft" {
		t.Fatalf("expected draft after edit, got %s", got)
	}
	assertAvailability(1, "after edit (draft still reserves)")

	// The reopened event records the status it came from.
	resp, _ = leader.Get("/api/v0/bookings/" + bookingID + "/events")
	events := expectStatus(t, resp, http.StatusOK)
	var eventList []map[string]any
	json.NewDecoder(bytes.NewReader(events)).Decode(&eventList)
	var found bool
	for _, e := range eventList {
		if e["event_type"] == "reopened" {
			found = true
			meta, _ := e["metadata"].(map[string]any)
			if meta["from"] != "submitted" {
				t.Errorf("expected metadata.from=submitted, got %v", meta["from"])
			}
		}
	}
	if !found {
		t.Error("expected a reopened event in the booking history")
	}

	// The draft must be resubmitted to return to the flow.
	sb, _ := json.Marshal(map[string]any{"force_approval": true})
	resp, err = leader.Post("/api/v0/bookings/"+bookingID+"/submit", bytes.NewReader(sb))
	if err != nil {
		t.Fatal(err)
	}
	expectStatus(t, resp, http.StatusOK)
	if got := getBookingStatus(t, leader, bookingID); got != "submitted" {
		t.Fatalf("expected submitted after resubmit, got %s", got)
	}

	// Manager can edit it too; same downgrade.
	resp, err = manager.Post("/api/v0/bookings/"+bookingID+"/reopen", bytes.NewReader([]byte("{}")))
	if err != nil {
		t.Fatal(err)
	}
	expectStatus(t, resp, http.StatusOK)
	if got := getBookingStatus(t, leader, bookingID); got != "draft" {
		t.Fatalf("expected draft after manager reopen, got %s", got)
	}
}

func TestBookingEdit_ReopenEndpoint(t *testing.T) {
	_, leader := setupBookingEditEnv(t)

	// Auto-confirmed booking: approval level none + team booking.
	bookingID, _ := createTeamBooking(t, leader, false, "2026-08-01", "2026-08-03", "Hajk")
	if got := getBookingStatus(t, leader, bookingID); got != "confirmed" {
		t.Fatalf("expected confirmed, got %s", got)
	}

	// Reopen takes a confirmed booking back to draft.
	resp, err := leader.Post("/api/v0/bookings/"+bookingID+"/reopen", bytes.NewReader([]byte("{}")))
	if err != nil {
		t.Fatal(err)
	}
	expectStatus(t, resp, http.StatusOK)
	if got := getBookingStatus(t, leader, bookingID); got != "draft" {
		t.Fatalf("expected draft after reopen, got %s", got)
	}

	// Reopening a draft is a no-op and logs no duplicate event.
	resp, err = leader.Post("/api/v0/bookings/"+bookingID+"/reopen", bytes.NewReader([]byte("{}")))
	if err != nil {
		t.Fatal(err)
	}
	expectStatus(t, resp, http.StatusOK)

	resp, _ = leader.Get("/api/v0/bookings/" + bookingID + "/events")
	events := expectStatus(t, resp, http.StatusOK)
	var eventList []map[string]any
	json.NewDecoder(bytes.NewReader(events)).Decode(&eventList)
	reopened := 0
	for _, e := range eventList {
		if e["event_type"] == "reopened" {
			reopened++
		}
	}
	if reopened != 1 {
		t.Errorf("expected exactly 1 reopened event, got %d", reopened)
	}

	// Picked up bookings are past editing - reopen must be rejected.
	sb, _ := json.Marshal(map[string]any{})
	resp, err = leader.Post("/api/v0/bookings/"+bookingID+"/submit", bytes.NewReader(sb))
	if err != nil {
		t.Fatal(err)
	}
	expectStatus(t, resp, http.StatusOK)
	resp, err = leader.Post("/api/v0/bookings/"+bookingID+"/pickup", bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatal(err)
	}
	expectStatus(t, resp, http.StatusOK)

	resp, err = leader.Post("/api/v0/bookings/"+bookingID+"/reopen", bytes.NewReader([]byte("{}")))
	if err != nil {
		t.Fatal(err)
	}
	expectStatus(t, resp, http.StatusBadRequest)
	if got := getBookingStatus(t, leader, bookingID); got != "picked_up" {
		t.Fatalf("expected picked_up to be untouched, got %s", got)
	}
}
