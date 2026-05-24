package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/scoutrustning/api/internal/auth"
	"github.com/malarscouterna/scoutrustning/api/internal/handler"
	"github.com/malarscouterna/scoutrustning/api/internal/testutil"
)

func mountAll(env *testutil.TestEnv) {
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries, Perms: handler.NewPermissionCache(env.Queries)}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
		r.Mount("/bookings", (&handler.BookingHandler{Q: env.Queries}).Routes())
		r.Mount("/teams", (&handler.TeamHandler{Q: env.Queries}).Routes())
		r.Mount("/issues", (&handler.IssueHandler{Q: env.Queries, Perms: handler.NewPermissionCache(env.Queries)}).Routes())
	})
}

// createArticle is a test helper that creates an article and returns its ID.
func createArticle(t *testing.T, client *testutil.TestClient, name, catID, locID string) string {
	t.Helper()
	b, _ := json.Marshal(map[string]any{
		"commercial_name": name, "common_name": name + " 1",
		"category_id": catID, "location_id": locID, "individually_tracked": true,
	})
	resp, err := client.Post("/api/v0/articles", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("create article: expected 201, got %d: %s", resp.StatusCode, body)
	}
	var article map[string]any
	json.NewDecoder(resp.Body).Decode(&article)
	return article["id"].(string)
}

// createBookingWithTeam creates a draft booking for a unit and returns the booking ID.
func createBookingWithTeam(t *testing.T, client *testutil.TestClient, teamID string) string {
	t.Helper()
	body := map[string]any{
		"start_date": "2026-06-01", "end_date": "2026-06-05",
		"used_by_team_id": teamID,
	}
	b, _ := json.Marshal(body)
	resp, err := client.Post("/api/v0/bookings", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("create booking: expected 201, got %d: %s", resp.StatusCode, body)
	}
	var booking map[string]any
	json.NewDecoder(resp.Body).Decode(&booking)
	return booking["id"].(string)
}

// getTeamID returns the team ID for a given team name.
func getTeamID(t *testing.T, client *testutil.TestClient, name string) string {
	t.Helper()
	resp, _ := client.Get("/api/v0/teams")
	defer resp.Body.Close()
	var units []map[string]any
	json.NewDecoder(resp.Body).Decode(&units)
	for _, u := range units {
		if u["name"] == name {
			return u["id"].(string)
		}
	}
	t.Fatalf("team %q not found", name)
	return ""
}

// seedIDs returns the first location and category IDs for the caller's group.
func seedIDs(t *testing.T, client *testutil.TestClient) (locID, catID string) {
	t.Helper()
	resp, _ := client.Get("/api/v0/locations")
	var locs []map[string]any
	json.NewDecoder(resp.Body).Decode(&locs)
	resp.Body.Close()
	resp, _ = client.Get("/api/v0/categories")
	var cats []map[string]any
	json.NewDecoder(resp.Body).Decode(&cats)
	resp.Body.Close()
	return locs[0]["id"].(string), cats[0]["id"].(string)
}

// TestAccess_TeamBookingVisibility verifies that team-scoped bookings are visible
// to unit members but not to leaders of other units.
func TestAccess_TeamBookingVisibility(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountAll(env)

	manager := env.ClientAs("manager-equipment")
	leaderYgg := env.ClientAs("leader-yggdrasil")
	leaderSpi := env.ClientAs("leader-flaskpost")

	// Teams are seeded by SetupTestEnv
	yggID := getTeamID(t, leaderYgg, "Yggdrasil")

	// Yggdrasil leader creates a unit booking
	bookingID := createBookingWithTeam(t, leaderYgg, yggID)

	t.Run("creator sees own team booking in list", func(t *testing.T) {
		resp, _ := leaderYgg.Get("/api/v0/bookings")
		defer resp.Body.Close()
		var bookings []map[string]any
		json.NewDecoder(resp.Body).Decode(&bookings)

		found := false
		for _, b := range bookings {
			if b["id"] == bookingID {
				found = true
			}
		}
		if !found {
			t.Error("creator should see own unit booking")
		}
	})

	t.Run("other team leader cannot see booking in list", func(t *testing.T) {
		resp, _ := leaderSpi.Get("/api/v0/bookings")
		defer resp.Body.Close()
		var bookings []map[string]any
		json.NewDecoder(resp.Body).Decode(&bookings)

		for _, b := range bookings {
			if b["id"] == bookingID {
				t.Error("Spindlarna leader should not see Yggdrasil booking")
			}
		}
	})

	t.Run("other team leader cannot modify booking", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"notes": "hacked"})
		resp, _ := leaderSpi.Put("/api/v0/bookings/"+bookingID, bytes.NewReader(b))
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("equipment manager sees all bookings", func(t *testing.T) {
		resp, _ := manager.Get("/api/v0/bookings")
		defer resp.Body.Close()
		var bookings []map[string]any
		json.NewDecoder(resp.Body).Decode(&bookings)

		found := false
		for _, b := range bookings {
			if b["id"] == bookingID {
				found = true
			}
		}
		if !found {
			t.Error("equipment manager should see all bookings")
		}
	})

	t.Run("equipment manager can modify any booking", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"notes": "manager note"})
		resp, _ := manager.Put("/api/v0/bookings/"+bookingID, bytes.NewReader(b))
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}
	})
}

// TestAccess_RoleEnforcementAllEndpoints verifies that leaders get 403 on all
// equipment manager endpoints.
func TestAccess_RoleEnforcementAllEndpoints(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountAll(env)

	leader := env.ClientAs("leader-yggdrasil")
	manager := env.ClientAs("manager-equipment")

	locID, catID := seedIDs(t, manager)

	// Create an article so we have an ID to test against
	articleID := createArticle(t, manager, "RoleTest", catID, locID)

	tests := []struct {
		name   string
		method string
		path   string
		body   map[string]any
	}{
		{"create article", "POST", "/api/v0/articles", map[string]any{
			"commercial_name": "X", "common_name": "X1",
			"category_id": catID, "location_id": locID,
		}},
		{"update article", "PUT", "/api/v0/articles/" + articleID, map[string]any{
			"commercial_name": "X", "common_name": "X1", "status": "ok",
			"category_id": catID, "location_id": locID,
		}},
		{"delete article", "DELETE", "/api/v0/articles/" + articleID, nil},
		{"create location", "POST", "/api/v0/locations", map[string]any{"name": "Test", "sort_order": 99}},
		{"create category", "POST", "/api/v0/categories", map[string]any{"name": "Test", "sort_order": 99}},
		{"create unit", "POST", "/api/v0/teams", map[string]any{"name": "TestUnit"}},
	}

	for _, tc := range tests {
		t.Run("leader gets 403 on "+tc.name, func(t *testing.T) {
			var body io.Reader
			if tc.body != nil {
				b, _ := json.Marshal(tc.body)
				body = bytes.NewReader(b)
			}
			resp, err := leader.Do(tc.method, tc.path, body)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				respBody, _ := io.ReadAll(resp.Body)
				t.Fatalf("expected 403, got %d: %s", resp.StatusCode, respBody)
			}
		})
	}

	// Verify leader CAN access read-only endpoints
	readEndpoints := []struct {
		name string
		path string
	}{
		{"list articles", "/api/v0/articles"},
		{"list locations", "/api/v0/locations"},
		{"list categories", "/api/v0/categories"},
		{"list units", "/api/v0/teams"},
		{"list bookings", "/api/v0/bookings"},
		{"check availability", "/api/v0/articles/availability?start_date=2026-01-01&end_date=2026-01-05"},
	}

	for _, tc := range readEndpoints {
		t.Run("leader can access "+tc.name, func(t *testing.T) {
			resp, err := leader.Get(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200, got %d", resp.StatusCode)
			}
		})
	}
}

// TestAccess_ArticleStatusRoles verifies that leaders can create issues but
// cannot set manager-only article statuses.
func TestAccess_ArticleStatusRoles(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountAll(env)

	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")

	locID, catID := seedIDs(t, manager)
	articleID := createArticle(t, manager, "StatusRoleTest", catID, locID)

	t.Run("leader can create issue", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{
			"article_id":  articleID,
			"severity":    "usable",
			"description": "Torn fabric",
		})
		resp, _ := leader.Post("/api/v0/issues", bytes.NewReader(b))
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
		}
	})

	t.Run("leader cannot set manager-only article status", func(t *testing.T) {
		for _, status := range []string{"ok", "under_repair", "archived"} {
			b, _ := json.Marshal(map[string]any{"status": status})
			resp, _ := leader.Put("/api/v0/articles/"+articleID+"/status", bytes.NewReader(b))
			resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("status %q: expected 403, got %d", status, resp.StatusCode)
			}
		}
	})

	t.Run("manager can set article status", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{"status": "under_repair", "comment": "Fixing it"})
		resp, _ := manager.Put("/api/v0/articles/"+articleID+"/status", bytes.NewReader(b))
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}
	})
}

// TestMultiTenancy_GroupIsolation verifies that data in one group is invisible
// to users in another group.
func TestMultiTenancy_GroupIsolation(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountAll(env)

	// Group 766 (Mälarscouterna)
	manager766 := env.ClientAs("manager-equipment")
	leader766 := env.ClientAs("leader-yggdrasil")

	// Group 999 (Testkåren)
	leader999 := env.ClientAs("other-kar-leader")

	// Manager creates article in group 766
	locID, catID := seedIDs(t, manager766)
	createArticle(t, manager766, "IsolationTest", catID, locID)

	// Leader in group 766 creates a booking
	b, _ := json.Marshal(map[string]any{"start_date": "2026-06-01", "end_date": "2026-06-05"})
	resp, _ := leader766.Post("/api/v0/bookings", bytes.NewReader(b))
	var booking766 map[string]any
	json.NewDecoder(resp.Body).Decode(&booking766)
	resp.Body.Close()

	t.Run("other group cannot see articles", func(t *testing.T) {
		resp, _ := leader999.Get("/api/v0/articles")
		defer resp.Body.Close()
		var articles []map[string]any
		json.NewDecoder(resp.Body).Decode(&articles)

		for _, a := range articles {
			if a["commercial_name"] == "IsolationTest" {
				t.Error("group 999 should not see group 766 articles")
			}
		}
	})

	t.Run("other group cannot see bookings", func(t *testing.T) {
		resp, _ := leader999.Get("/api/v0/bookings")
		defer resp.Body.Close()
		var bookings []map[string]any
		json.NewDecoder(resp.Body).Decode(&bookings)

		for _, b := range bookings {
			if b["id"] == booking766["id"] {
				t.Error("group 999 should not see group 766 bookings")
			}
		}
	})

	t.Run("other group cannot see locations", func(t *testing.T) {
		resp, _ := leader999.Get("/api/v0/locations")
		defer resp.Body.Close()
		var locations []map[string]any
		json.NewDecoder(resp.Body).Decode(&locations)

		// Group 999 should only see "Förrådet"
		for _, l := range locations {
			if l["name"] == "Hajkförrådet" {
				t.Error("group 999 should not see group 766 locations")
			}
		}
	})

	t.Run("other group cannot access booking by ID", func(t *testing.T) {
		resp, _ := leader999.Get("/api/v0/bookings/" + booking766["id"].(string))
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}
	})
}

// TestAccess_TeamMembershipOnBooking verifies that leaders can only book for
// their own units, project leaders for their projects, and managers for anything.
func TestAccess_TeamMembershipOnBooking(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountAll(env)

	manager := env.ClientAs("manager-equipment")
	leaderYgg := env.ClientAs("leader-yggdrasil")
	leaderSpi := env.ClientAs("leader-flaskpost")
	projectLeader := env.ClientAs("project-leader")

	// Teams are seeded by SetupTestEnv
	yggID := getTeamID(t, leaderYgg, "Yggdrasil")
	spiID := getTeamID(t, leaderSpi, "Spindlarna")
	valborgID := getTeamID(t, projectLeader, "Valborgskommittén")

	t.Run("leader can book for own team", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{
			"start_date": "2026-06-01", "end_date": "2026-06-05",
			"used_by_team_id": yggID,
		})
		resp, _ := leaderYgg.Post("/api/v0/bookings", bytes.NewReader(b))
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
		}
	})

	t.Run("leader cannot book for other team", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{
			"start_date": "2026-06-01", "end_date": "2026-06-05",
			"used_by_team_id": spiID,
		})
		resp, _ := leaderYgg.Post("/api/v0/bookings", bytes.NewReader(b))
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("trusted user can book for own role team", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{
			"start_date": "2026-06-01", "end_date": "2026-06-05",
			"used_by_team_id": valborgID,
		})
		resp, _ := projectLeader.Post("/api/v0/bookings", bytes.NewReader(b))
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
		}
	})

	t.Run("trusted user cannot book for a troop", func(t *testing.T) {
		b, _ := json.Marshal(map[string]any{
			"start_date": "2026-06-01", "end_date": "2026-06-05",
			"used_by_team_id": yggID,
		})
		resp, _ := projectLeader.Post("/api/v0/bookings", bytes.NewReader(b))
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("manager can book for any team", func(t *testing.T) {
		for _, id := range []string{yggID, spiID, valborgID} {
			b, _ := json.Marshal(map[string]any{
				"start_date": "2026-07-01", "end_date": "2026-07-05",
				"used_by_team_id": id,
			})
			resp, _ := manager.Post("/api/v0/bookings", bytes.NewReader(b))
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusCreated {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("expected 201 for unit %s, got %d: %s", id, resp.StatusCode, body)
			}
		}
	})
}

// TestAccess_PickupEventLogging verifies that pickup and return actions
// are logged with the acting user in article_events.
func TestAccess_PickupEventLogging(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries, Perms: handler.NewPermissionCache(env.Queries)}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
		r.Mount("/bookings", (&handler.BookingHandler{Q: env.Queries}).Routes())
		r.Mount("/teams", (&handler.TeamHandler{Q: env.Queries}).Routes())
		r.Get("/me", func(w http.ResponseWriter, r *http.Request) {
			claims, _ := auth.ClaimsFromContext(r.Context())
			handler.WriteJSON(w, http.StatusOK, claims)
		})
	})

	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")

	locID, catID := seedIDs(t, manager)
	articleID := createArticle(t, manager, "EventLogTest", catID, locID)

	// Create booking, add item, submit, pickup
	b, _ := json.Marshal(map[string]any{"start_date": "2026-08-01", "end_date": "2026-08-05"})
	resp, _ := leader.Post("/api/v0/bookings", bytes.NewReader(b))
	var booking map[string]any
	json.NewDecoder(resp.Body).Decode(&booking)
	resp.Body.Close()
	bookingID := booking["id"].(string)

	b, _ = json.Marshal(map[string]any{"commercial_name": "EventLogTest", "quantity": 1})
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
	items := detail["items"].([]any)
	itemID := items[0].(map[string]any)["id"].(string)

	// Tick as picked up
	b, _ = json.Marshal(map[string]any{"pickup_status": "picked_up"})
	resp, _ = leader.Put("/api/v0/bookings/"+bookingID+"/items/"+itemID+"/pickup", bytes.NewReader(b))
	resp.Body.Close()

	t.Run("pickup logged with actor", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/articles/" + articleID + "/events")
		defer resp.Body.Close()
		var result struct{ Events []map[string]any }
		json.NewDecoder(resp.Body).Decode(&result)
		events := result.Events

		found := false
		for _, e := range events {
			if e["event_type"] == "picked_up" {
				found = true
				if e["actor_name"] != "Hanna Yggdrasil" {
					t.Errorf("expected actor Hanna Yggdrasil, got %v", e["actor_name"])
				}
			}
		}
		if !found {
			t.Error("expected picked_up event in article history")
		}
	})

	// Return as OK
	b, _ = json.Marshal(map[string]any{"return_status": "returned_ok"})
	resp, _ = leader.Put("/api/v0/bookings/"+bookingID+"/items/"+itemID+"/return", bytes.NewReader(b))
	resp.Body.Close()

	t.Run("return logged with actor", func(t *testing.T) {
		resp, _ := leader.Get("/api/v0/articles/" + articleID + "/events")
		defer resp.Body.Close()
		var result struct{ Events []map[string]any }
		json.NewDecoder(resp.Body).Decode(&result)
		events := result.Events

		found := false
		for _, e := range events {
			if e["event_type"] == "returned" {
				found = true
				if e["actor_name"] != "Hanna Yggdrasil" {
					t.Errorf("expected actor Hanna Yggdrasil, got %v", e["actor_name"])
				}
			}
		}
		if !found {
			t.Error("expected returned event in article history")
		}
	})
}
