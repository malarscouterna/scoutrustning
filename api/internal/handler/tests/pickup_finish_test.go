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

func setupFinishPickupEnv(t *testing.T) (*testutil.TestEnv, *testutil.TestClient) {
	t.Helper()
	env := testutil.SetupTestEnv(t)
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries, Perms: handler.NewPermissionCache(env.Queries)}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
		r.Mount("/bookings", (&handler.BookingHandler{Q: env.Queries, Pool: env.Pool, Perms: handler.NewPermissionCache(env.Queries)}).Routes())
		r.Mount("/teams", (&handler.TeamHandler{Q: env.Queries}).Routes())
	})

	manager := env.ClientAs("manager-equipment")
	locID, catID := seedIDs(t, manager)
	for i := range 2 {
		body, _ := json.Marshal(map[string]any{
			"commercial_name":      "Stormkök",
			"common_name":          "Stormkök " + string(rune('1'+i)),
			"category_id":          catID,
			"location_id":          locID,
			"individually_tracked": true,
		})
		resp, _ := manager.Post("/api/v0/articles", bytes.NewReader(body))
		resp.Body.Close()
	}
	return env, env.ClientAs("leader-yggdrasil")
}

func getBookingItems(t *testing.T, client *testutil.TestClient, bookingID string) []map[string]any {
	t.Helper()
	resp, err := client.Get("/api/v0/bookings/" + bookingID)
	if err != nil {
		t.Fatal(err)
	}
	body := expectStatus(t, resp, http.StatusOK)
	var out struct {
		Items []map[string]any `json:"items"`
	}
	json.NewDecoder(bytes.NewReader(body)).Decode(&out)
	return out.Items
}

func setItemPickup(t *testing.T, client *testutil.TestClient, bookingID, itemID, status string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"pickup_status": status})
	resp, err := client.Put("/api/v0/bookings/"+bookingID+"/items/"+itemID+"/pickup", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	expectStatus(t, resp, http.StatusOK)
}

func finishPickup(t *testing.T, client *testutil.TestClient, bookingID string, wantStatus int) map[string]any {
	t.Helper()
	resp, err := client.Post("/api/v0/bookings/"+bookingID+"/finish-pickup", bytes.NewReader([]byte("{}")))
	if err != nil {
		t.Fatal(err)
	}
	body := expectStatus(t, resp, wantStatus)
	var out map[string]any
	json.NewDecoder(bytes.NewReader(body)).Decode(&out)
	return out
}

func TestFinishPickup_ReleasesUnpickedItems(t *testing.T) {
	_, leader := setupFinishPickupEnv(t)
	bookingID, _ := createTeamBooking(t, leader, false, "2026-09-01", "2026-09-05", "Partial pickup")

	// A second stove, so finishing can release one while one stays on loan.
	// Adding it reopens the confirmed booking as a draft, so resubmit first.
	body, _ := json.Marshal(map[string]any{"commercial_name": "Stormkök", "quantity": 1})
	resp, err := leader.Post("/api/v0/bookings/"+bookingID+"/items", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	expectStatus(t, resp, http.StatusCreated)
	resp, err = leader.Post("/api/v0/bookings/"+bookingID+"/submit", bytes.NewReader([]byte(`{"force_approval":false}`)))
	if err != nil {
		t.Fatal(err)
	}
	expectStatus(t, resp, http.StatusOK)

	resp, err = leader.Post("/api/v0/bookings/"+bookingID+"/pickup", bytes.NewReader([]byte("{}")))
	if err != nil {
		t.Fatal(err)
	}
	expectStatus(t, resp, http.StatusOK)

	items := getBookingItems(t, leader, bookingID)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	pickedID := items[0]["id"].(string)
	unpickedID := items[1]["id"].(string)
	setItemPickup(t, leader, bookingID, pickedID, "picked_up")

	result := finishPickup(t, leader, bookingID, http.StatusOK)
	if result["released"] != float64(1) || result["picked"] != float64(1) {
		t.Fatalf("unexpected finish result: %+v", result)
	}

	statuses := map[string]any{}
	for _, item := range getBookingItems(t, leader, bookingID) {
		statuses[item["id"].(string)] = item["pickup_status"]
	}
	if statuses[pickedID] != "picked_up" {
		t.Errorf("picked item changed status: %v", statuses[pickedID])
	}
	if statuses[unpickedID] != "not_picked_up" {
		t.Errorf("expected not_picked_up, got %v", statuses[unpickedID])
	}

	// The released stove is bookable again; the collected one stays reserved.
	resp, _ = leader.Get("/api/v0/articles/availability?start_date=2026-09-01&end_date=2026-09-05")
	avail := expectStatus(t, resp, http.StatusOK)
	var groups []map[string]any
	json.NewDecoder(bytes.NewReader(avail)).Decode(&groups)
	for _, g := range groups {
		if g["commercial_name"] == "Stormkök" && g["available_count"] != float64(1) {
			t.Errorf("expected 1 available Stormkök after release, got %v", g["available_count"])
		}
	}

	// Returning only the collected item completes the booking.
	body, _ = json.Marshal(map[string]string{"return_status": "returned_ok"})
	resp, err = leader.Put("/api/v0/bookings/"+bookingID+"/items/"+pickedID+"/return", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	expectStatus(t, resp, http.StatusOK)
	resp, err = leader.Post("/api/v0/bookings/"+bookingID+"/return", bytes.NewReader([]byte("{}")))
	if err != nil {
		t.Fatal(err)
	}
	expectStatus(t, resp, http.StatusOK)

	resp, _ = leader.Get("/api/v0/bookings/" + bookingID + "/events")
	events := expectStatus(t, resp, http.StatusOK)
	var eventList []map[string]any
	json.NewDecoder(bytes.NewReader(events)).Decode(&eventList)
	found := false
	for _, e := range eventList {
		if e["event_type"] == "pickup_finished" {
			found = true
		}
	}
	if !found {
		t.Error("expected a pickup_finished event")
	}
}

func TestFinishPickup_RequiresPickedItemAndAccess(t *testing.T) {
	env, leader := setupFinishPickupEnv(t)
	bookingID, _ := createTeamBooking(t, leader, false, "2026-10-01", "2026-10-05", "No pickup yet")
	resp, err := leader.Post("/api/v0/bookings/"+bookingID+"/pickup", bytes.NewReader([]byte("{}")))
	if err != nil {
		t.Fatal(err)
	}
	expectStatus(t, resp, http.StatusOK)

	finishPickup(t, leader, bookingID, http.StatusBadRequest)

	// A leader from an unrelated team cannot finish somebody else's pickup.
	other := env.ClientAs("leader-flaskpost")
	finishPickup(t, other, bookingID, http.StatusForbidden)

	// Status must still be picked_up after the rejected calls.
	resp, _ = leader.Get("/api/v0/bookings/" + bookingID)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var detail struct {
		Booking map[string]any `json:"booking"`
	}
	json.NewDecoder(bytes.NewReader(body)).Decode(&detail)
	if detail.Booking["status"] != "picked_up" {
		t.Errorf("expected picked_up to be untouched, got %v", detail.Booking["status"])
	}
}
