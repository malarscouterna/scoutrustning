package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/ms-utrustning/api/internal/handler"
	"github.com/malarscouterna/ms-utrustning/api/internal/testutil"
)

func mountUserRoutes(env *testutil.TestEnv) {
	env.V1(func(r chi.Router) {
		r.Mount("/users", (&handler.UserHandler{Q: env.Queries}).Routes())
	})
}

func mountUserRoutesDemo(env *testutil.TestEnv, personaIDs map[string]bool) {
	env.V1(func(r chi.Router) {
		r.Mount("/users", (&handler.UserHandler{Q: env.Queries, DemoMode: true, PersonaIDs: personaIDs}).Routes())
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
