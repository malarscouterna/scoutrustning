package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/malarscouterna/scoutrustning/api/internal/handler"
	"github.com/malarscouterna/scoutrustning/api/internal/testutil"
)

func mountUserRoutes(env *testutil.TestEnv) {
	env.V1(func(r chi.Router) {
		r.Mount("/users", (&handler.UserHandler{Q: env.Queries, Perms: handler.NewPermissionCache(env.Queries)}).Routes())
	})
}

func mountUserRoutesDemo(env *testutil.TestEnv, personaIDs map[string]bool) {
	env.V1(func(r chi.Router) {
		r.Mount("/users", (&handler.UserHandler{Q: env.Queries, Perms: handler.NewPermissionCache(env.Queries), DemoMode: true, PersonaIDs: personaIDs}).Routes())
	})
}

func TestUsers_GroupMembers(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountUserRoutes(env)

	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")

	// Seed users directly — simulates users who have previously logged in.
	ctx := t.Context()
	_, err := env.Pool.Exec(ctx, `
		INSERT INTO users (id, group_id, name, email, max_access_level) VALUES
			('u-manager',  '766', 'Gillis Utrustning',    'gillis@example.com',   'manager'),
			('u-trusted',  '766', 'Julia Valborg',        'julia@example.com',    'trusted'),
			('u-book',     '766', 'Hanna Yggdrasil',      'hanna@example.com',    'book'),
			('u-view',     '766', 'Vera Visa',            'vera@example.com',     'view'),
			('u-other',    '999', 'Linn Annan-Kår',       'linn@other.example.com','book')
	`)
	if err != nil {
		t.Fatalf("seed users: %v", err)
	}

	t.Run("leader gets 403", func(t *testing.T) {
		resp, err := leader.Get("/api/v0/users")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("manager gets all group users with access_level", func(t *testing.T) {
		resp, err := manager.Get("/api/v0/users")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var users []map[string]any
		json.NewDecoder(resp.Body).Decode(&users)

		if len(users) == 0 {
			t.Fatal("expected users, got none")
		}
		for _, u := range users {
			for _, field := range []string{"id", "name", "email", "access_level"} {
				if _, ok := u[field]; !ok {
					t.Errorf("user missing field %q", field)
				}
			}
		}
	})

	t.Run("access_levels filter returns only matching users", func(t *testing.T) {
		resp, err := manager.Get("/api/v0/users?access_levels=trusted,manager")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var users []map[string]any
		json.NewDecoder(resp.Body).Decode(&users)

		for _, u := range users {
			level := u["access_level"].(string)
			if level != "trusted" && level != "manager" {
				t.Errorf("unexpected access_level %q in filtered result", level)
			}
		}
		// view-only and book-level users must not appear
		for _, u := range users {
			if u["email"] == "vera@example.com" {
				t.Error("view-only user should not appear in trusted,manager filter")
			}
		}
	})

	t.Run("demo mode: only persona IDs are returned, real users excluded", func(t *testing.T) {
		demoEnv := testutil.SetupTestEnv(t)
		// Seed one persona ID and one "real" user ID into the same group.
		ctx2 := t.Context()
		_, err2 := demoEnv.Pool.Exec(ctx2, `
			INSERT INTO users (id, group_id, name, email, max_access_level) VALUES
				('persona-1',  '766', 'Persona One',  'persona@example.com', 'manager'),
				('real-user',  '766', 'Real Person',  'real@example.com',    'book')
		`)
		if err2 != nil {
			t.Fatalf("seed demo users: %v", err2)
		}
		personaIDs := map[string]bool{"persona-1": true}
		mountUserRoutesDemo(demoEnv, personaIDs)
		demoManager := demoEnv.ClientAs("manager-equipment")

		resp, err := demoManager.Get("/api/v0/users")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var users []map[string]any
		json.NewDecoder(resp.Body).Decode(&users)

		// Every returned user must be in the persona ID set.
		for _, u := range users {
			id, _ := u["id"].(string)
			if !personaIDs[id] {
				t.Errorf("demo mode: non-persona user %q (%v) leaked into response", id, u["email"])
			}
		}
		// The seeded persona must be present.
		found := false
		for _, u := range users {
			if u["id"] == "persona-1" {
				found = true
			}
		}
		if !found {
			t.Error("demo mode: persona should appear in response")
		}
	})

	t.Run("group isolation: group 999 users not visible to group 766 manager", func(t *testing.T) {
		resp, err := manager.Get("/api/v0/users")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		var users []map[string]any
		json.NewDecoder(resp.Body).Decode(&users)

		for _, u := range users {
			if u["email"] == "linn@other.example.com" {
				t.Error("user from group 999 should not appear in group 766 results")
			}
		}
	})
}

func TestUsers_GetUserInfo(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountUserRoutes(env)

	ctx := context.Background()
	leaderID := "3000005" // leader-yggdrasil persona

	// Upsert the leader by making an authenticated request so team_ids get populated.
	leader := env.ClientAs("leader-yggdrasil")
	resp, err := leader.Get("/api/v0/users")
	if err == nil {
		resp.Body.Close()
	}

	var yggdrasilID pgtype.UUID
	env.Pool.QueryRow(ctx, `SELECT id FROM teams WHERE group_id = '766' AND name = 'Yggdrasil'`).Scan(&yggdrasilID)

	today := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	draftID := seedBooking(t, env, leaderID, "draft", today, today.AddDate(0, 0, 2), yggdrasilID)
	submittedID := seedBooking(t, env, leaderID, "submitted", today.AddDate(0, 0, 5), today.AddDate(0, 0, 7), yggdrasilID)

	manager := env.ClientAs("manager-equipment")
	otherLeader := env.ClientAs("leader-flaskpost")
	if resp, err := otherLeader.Get("/api/v0/users/" + leaderID); err == nil {
		resp.Body.Close()
	}

	// A booking owned by otherLeader that leader-yggdrasil participated in via pickup,
	// without owning it - should still show up in leader's open bookings.
	var articleID pgtype.UUID
	env.Pool.QueryRow(ctx, `
		INSERT INTO articles (group_id, commercial_name, common_name, category_id, location_id)
		SELECT '766', 'Test Tent', 'Test Tent 1', c.id, l.id
		FROM categories c, locations l
		WHERE c.group_id = '766' AND l.group_id = '766'
		LIMIT 1
		RETURNING id
	`).Scan(&articleID)
	participatedID := seedBooking(t, env, "3000006", "picked_up", today.AddDate(0, 0, 10), today.AddDate(0, 0, 12), yggdrasilID)
	metadata, _ := json.Marshal(map[string]string{"booking_id": formatPgUUID(participatedID)})
	_, err = env.Pool.Exec(ctx, `
		INSERT INTO article_events (group_id, article_id, actor_id, event_type, description, metadata)
		VALUES ('766', $1, $2, 'picked_up', 'Picked up', $3)
	`, articleID, leaderID, metadata)
	if err != nil {
		t.Fatalf("seed article event: %v", err)
	}

	var issueID pgtype.UUID
	err = env.Pool.QueryRow(ctx, `
		INSERT INTO issue_reports (group_id, title, description, severity, status, reporter_id)
		VALUES ('766', 'Broken zipper', 'Zipper stuck', 'usable', 'open', $1)
		RETURNING id
	`, leaderID).Scan(&issueID)
	if err != nil {
		t.Fatalf("seed issue: %v", err)
	}

	t.Run("manager sees draft and submitted bookings plus team affiliation", func(t *testing.T) {
		resp, err := manager.Get("/api/v0/users/" + leaderID)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var info map[string]any
		json.NewDecoder(resp.Body).Decode(&info)

		teams, _ := info["teams"].([]any)
		foundYggdrasil := false
		for _, tm := range teams {
			team := tm.(map[string]any)
			if team["name"] == "Yggdrasil" {
				foundYggdrasil = true
			}
		}
		if !foundYggdrasil {
			t.Error("expected Yggdrasil team affiliation")
		}

		bookings, _ := info["open_bookings"].([]any)
		ids := map[string]bool{}
		for _, b := range bookings {
			ids[b.(map[string]any)["id"].(string)] = true
		}
		if !ids[formatPgUUID(draftID)] {
			t.Error("manager should see draft booking")
		}
		if !ids[formatPgUUID(submittedID)] {
			t.Error("manager should see submitted booking")
		}
		if !ids[formatPgUUID(participatedID)] {
			t.Error("manager should see booking leader participated in via pickup, though owned by someone else")
		}

		issues, _ := info["issues"].([]any)
		foundIssue := false
		for _, i := range issues {
			if i.(map[string]any)["id"] == formatPgUUID(issueID) {
				foundIssue = true
			}
		}
		if !foundIssue {
			t.Error("manager (has issue_resolve permission) should see leader's reported issue")
		}
	})

	t.Run("non-manager does not see draft booking", func(t *testing.T) {
		resp, err := otherLeader.Get("/api/v0/users/" + leaderID)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var info map[string]any
		json.NewDecoder(resp.Body).Decode(&info)

		bookings, _ := info["open_bookings"].([]any)
		ids := map[string]bool{}
		for _, b := range bookings {
			ids[b.(map[string]any)["id"].(string)] = true
		}
		if ids[formatPgUUID(draftID)] {
			t.Error("non-manager should not see draft booking")
		}
		if !ids[formatPgUUID(submittedID)] {
			t.Error("non-manager should see submitted booking")
		}
		if !ids[formatPgUUID(participatedID)] {
			t.Error("non-manager should see picked_up booking leader participated in")
		}

		issues, _ := info["issues"].([]any)
		if len(issues) != 0 {
			t.Error("non-manager (lacks issue_resolve permission) should see an empty issues list")
		}
	})
}

func formatPgUUID(u pgtype.UUID) string {
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
