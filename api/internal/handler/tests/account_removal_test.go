package tests

import (
	"net/http"
	"testing"

	"github.com/malarscouterna/scoutrustning/api/internal/db"
	"github.com/malarscouterna/scoutrustning/api/internal/testutil"
)

func TestAccountRemoval(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountActiveGroupRoutes(env)

	t.Run("scrubs personal info but keeps the row linked", func(t *testing.T) {
		client := env.ClientAs("leader-flaskpost")

		before := getMe(t, client)
		if before["name"] == "[Borttagen användare]" {
			t.Fatal("persona should start with a real name")
		}

		resp, err := client.Do("DELETE", "/api/v0/me", nil)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("DELETE /me: status %d", resp.StatusCode)
		}

		u, err := env.Queries.GetUser(t.Context(), db.GetUserParams{ID: "3000006", GroupID: "766"})
		if err != nil {
			t.Fatal(err)
		}
		if u.Name != "[Borttagen användare]" {
			t.Errorf("name = %q, want placeholder", u.Name)
		}
		if u.Email != "borttagen@scoutrustning.invalid" {
			t.Errorf("email = %q, want placeholder", u.Email)
		}
		if u.MaxAccessLevel != "view" {
			t.Errorf("max_access_level = %q, want view", u.MaxAccessLevel)
		}
		if len(u.TeamIds) != 0 {
			t.Errorf("team_ids = %v, want empty", u.TeamIds)
		}
	})

	t.Run("re-login restores the full profile", func(t *testing.T) {
		client := env.ClientAs("leader-flaskpost")
		resp, err := client.Do("DELETE", "/api/v0/me", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()

		// Any subsequent request re-runs UpsertUserMiddleware, which
		// overwrites the scrubbed fields from the persona's claims.
		me := getMe(t, client)
		if me["name"] != "Fredrik Flaskpost" {
			t.Errorf("name after re-login = %v, want restored", me["name"])
		}
	})

	t.Run("multi-group member: removal is global, scrubs every group's row in one call", func(t *testing.T) {
		julia766 := env.ClientAs("project-unit-leader")
		julia999 := julia766.WithActiveGroup("999")

		// Seed her group-999 row first (only created on first request resolving
		// to that group) so there's something to assert got scrubbed too.
		getMe(t, julia999)

		resp, err := julia766.Do("DELETE", "/api/v0/me", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()

		u999, err := env.Queries.GetUser(t.Context(), db.GetUserParams{ID: "3000003", GroupID: "999"})
		if err != nil {
			t.Fatal(err)
		}
		if u999.Name != "[Borttagen användare]" {
			t.Errorf("group 999 profile name = %q, want scrubbed too", u999.Name)
		}

		u766, err := env.Queries.GetUser(t.Context(), db.GetUserParams{ID: "3000003", GroupID: "766"})
		if err != nil {
			t.Fatal(err)
		}
		if u766.Name != "[Borttagen användare]" {
			t.Errorf("group 766 profile name = %q, want scrubbed", u766.Name)
		}

		// Switching to the other group still works post-removal - the row
		// wasn't deleted, just scrubbed, so no "recreate to enter" gap.
		me999 := getMe(t, julia999)
		if me999["group_id"] != "999" {
			t.Errorf("group_id = %v, want 999", me999["group_id"])
		}
	})

	t.Run("removed manager no longer appears in GetGroupManagers", func(t *testing.T) {
		manager := env.ClientAs("manager-equipment")
		getMe(t, manager) // seed the row via UpsertUserMiddleware

		before, err := env.Queries.GetGroupManagers(t.Context(), "766")
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, m := range before {
			if m.ID == "3000002" {
				found = true
			}
		}
		if !found {
			t.Fatal("manager-equipment should be in the manager list before removal")
		}

		resp, err := manager.Do("DELETE", "/api/v0/me", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()

		after, err := env.Queries.GetGroupManagers(t.Context(), "766")
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range after {
			if m.ID == "3000002" {
				t.Error("removed manager should no longer appear in GetGroupManagers")
			}
		}
	})
}
