package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/scoutrustning/api/internal/handler"
	"github.com/malarscouterna/scoutrustning/api/internal/testutil"
)

func mountActiveGroupRoutes(env *testutil.TestEnv) {
	notifPrefs := &handler.NotificationPrefsHandler{Q: env.Queries}
	me := &handler.MeHandler{Q: env.Queries, Perms: handler.NewPermissionCache(env.Queries), NotifPrefs: notifPrefs}
	env.V1(func(r chi.Router) {
		r.Mount("/me", me.Routes())
	})
}

func getMe(t *testing.T, client *testutil.TestClient) map[string]any {
	t.Helper()
	resp, err := client.Get("/api/v0/me")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /me: status %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body
}

// project-unit-leader (Julia) belongs to two registered groups (766, 999) -
// see dev-personas.json. This exercises the cookie-hint -> pickActiveGroup
// path added for group switching (docs/implementation/scout-group-signup.md §3).
func TestActiveGroup_MultiGroupPersonaSwitch(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountActiveGroupRoutes(env)

	multiGroupUser := env.ClientAs("project-unit-leader")

	t.Run("no hint defaults to primary/first group (766)", func(t *testing.T) {
		me := getMe(t, multiGroupUser)
		if me["group_id"] != "766" {
			t.Errorf("group_id = %v, want 766", me["group_id"])
		}
		groups, ok := me["groups"].([]any)
		if !ok || len(groups) != 2 {
			t.Fatalf("groups = %v, want 2 entries", me["groups"])
		}
	})

	t.Run("hint switches to the other registered group", func(t *testing.T) {
		me := getMe(t, multiGroupUser.WithActiveGroup("999"))
		if me["group_id"] != "999" {
			t.Errorf("group_id = %v, want 999", me["group_id"])
		}
	})

	t.Run("hint for an unregistered group falls back", func(t *testing.T) {
		me := getMe(t, multiGroupUser.WithActiveGroup("123456"))
		if me["group_id"] != "766" {
			t.Errorf("group_id = %v, want fallback to 766", me["group_id"])
		}
	})

	t.Run("single-group persona is unaffected by the hint machinery", func(t *testing.T) {
		single := env.ClientAs("leader-yggdrasil")
		me := getMe(t, single)
		if me["group_id"] != "766" {
			t.Errorf("group_id = %v, want 766", me["group_id"])
		}
		groups, ok := me["groups"].([]any)
		if !ok || len(groups) != 1 {
			t.Fatalf("groups = %v, want 1 entry", me["groups"])
		}
	})
}
