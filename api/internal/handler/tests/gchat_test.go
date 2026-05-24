package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/scoutrustning/api/internal/handler"
	"github.com/malarscouterna/scoutrustning/api/internal/notifications"
	"github.com/malarscouterna/scoutrustning/api/internal/testutil"
)

// stubSpaces is returned by the fake ListSpacesFn.
var stubSpaces = []notifications.GChatSpace{
	{Name: "spaces/AAA111", DisplayName: "Eagle Scouts"},
	{Name: "spaces/BBB222", DisplayName: "Lager"},
}

func mountGChatRoutes(env *testutil.TestEnv, gs *handler.GroupSettingsHandler) {
	env.V1(func(r chi.Router) {
		r.Mount("/group-settings", gs.Routes())
		r.Mount("/teams", (&handler.TeamHandler{
			Q: env.Queries,
			AddBotFn: func(_ context.Context, _ []byte, _, _, _ string) error {
				return nil
			},
		}).Routes())
	})
}

func newGChatHandler(env *testutil.TestEnv) *handler.GroupSettingsHandler {
	return &handler.GroupSettingsHandler{
		Q:     env.Queries,
		Pool:  env.Pool,
		Perms: handler.NewPermissionCache(env.Queries),
		ListSpacesFn: func(_ context.Context, _ []byte, _ string) ([]notifications.GChatSpace, error) {
			// Return fresh copies so the handler's BotIsMember mutations don't persist.
			out := make([]notifications.GChatSpace, len(stubSpaces))
			copy(out, stubSpaces)
			return out, nil
		},
	}
}

func uploadKey(t *testing.T, client *testutil.TestClient) *http.Response {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"key_json":    map[string]any{"type": "service_account", "project_id": "test"},
		"admin_email": "admin@example.org",
	})
	resp, err := client.Post("/api/v0/group-settings/gchat-key", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST gchat-key: %v", err)
	}
	return resp
}

func getManagerTeamID(t *testing.T, client *testutil.TestClient) string {
	t.Helper()
	resp, _ := client.Get("/api/v0/teams")
	defer resp.Body.Close()
	var teams []map[string]any
	json.NewDecoder(resp.Body).Decode(&teams)
	for _, team := range teams {
		if team["access_level"] == "manager" {
			return team["id"].(string)
		}
	}
	t.Fatal("no manager team found")
	return ""
}

// TestGChatKeyManagement_Auth covers permission and demo-mode guards.
// Uses one env; subtests are independent (none mutate shared state).
func TestGChatKeyManagement_Auth(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	gs := newGChatHandler(env)
	mountGChatRoutes(env, gs)

	leader := env.ClientAs("leader-yggdrasil")
	manager := env.ClientAs("manager-equipment")

	t.Run("upload key: non-manager is forbidden", func(t *testing.T) {
		resp := uploadKey(t, leader)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("upload key: invalid JSON body returns 400", func(t *testing.T) {
		resp, _ := manager.Post("/api/v0/group-settings/gchat-key", bytes.NewReader([]byte(`not json`)))
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("list spaces: returns 400 when gchat not configured", func(t *testing.T) {
		resp, _ := manager.Get("/api/v0/group-settings/gchat-spaces")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400 when gchat not configured, got %d", resp.StatusCode)
		}
	})

	t.Run("set team gchat space: requires gchat to be configured", func(t *testing.T) {
		teamID := getManagerTeamID(t, manager)
		body, _ := json.Marshal(map[string]any{"gchat_space_id": "spaces/AAA111"})
		resp, _ := manager.Put(fmt.Sprintf("/api/v0/teams/%s/gchat-space", teamID), bytes.NewReader(body))
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400 when gchat not configured, got %d", resp.StatusCode)
		}
	})
}

// TestGChatKeyManagement_DemoMode covers the demo-mode 403 guards.
func TestGChatKeyManagement_DemoMode(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	gs := newGChatHandler(env)
	gs.DemoMode = true
	mountGChatRoutes(env, gs)
	manager := env.ClientAs("manager-equipment")

	t.Run("upload key returns 403", func(t *testing.T) {
		resp := uploadKey(t, manager)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403 in demo mode, got %d", resp.StatusCode)
		}
	})

	t.Run("delete key returns 403", func(t *testing.T) {
		resp, _ := manager.Delete("/api/v0/group-settings/gchat-key")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403 in demo mode, got %d", resp.StatusCode)
		}
	})
}

// TestGChatKeyManagement_Lifecycle exercises the full connect → use → disconnect flow
// in a single sequential test to avoid cross-test DB races.
func TestGChatKeyManagement_Lifecycle(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountGChatRoutes(env, newGChatHandler(env))
	manager := env.ClientAs("manager-equipment")

	// --- Connect ---
	t.Run("upload key stores credentials and adds gchat to enabled_channels", func(t *testing.T) {
		resp := uploadKey(t, manager)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var body map[string]any
		json.NewDecoder(resp.Body).Decode(&body)
		if body["gchat_configured"] != true {
			t.Errorf("expected gchat_configured:true, got %v", body)
		}
		if body["gchat_admin_email"] != "admin@example.org" {
			t.Errorf("unexpected admin email: %v", body["gchat_admin_email"])
		}
		spaces, ok := body["spaces"].([]any)
		if !ok || len(spaces) != 2 {
			t.Errorf("expected 2 spaces in response, got %v", body["spaces"])
		}

		gsResp, _ := manager.Get("/api/v0/group-settings")
		defer gsResp.Body.Close()
		var gs map[string]any
		json.NewDecoder(gsResp.Body).Decode(&gs)
		if gs["gchat_configured"] != true {
			t.Errorf("group-settings: expected gchat_configured:true, got %v", gs["gchat_configured"])
		}
		channels, _ := gs["notification_channels"].([]any)
		hasGChat := false
		for _, ch := range channels {
			if ch == "gchat" {
				hasGChat = true
			}
		}
		if !hasGChat {
			t.Errorf("expected gchat in notification_channels, got %v", channels)
		}
	})

	// --- List spaces ---
	t.Run("list spaces returns configured spaces", func(t *testing.T) {
		resp, _ := manager.Get("/api/v0/group-settings/gchat-spaces")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var spaces []map[string]any
		json.NewDecoder(resp.Body).Decode(&spaces)
		if len(spaces) != 2 {
			t.Errorf("expected 2 spaces, got %d", len(spaces))
		}
	})

	// --- Link team to space ---
	teamID := getManagerTeamID(t, manager)

	t.Run("link team space marks bot_is_member", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{"gchat_space_id": "spaces/AAA111"})
		resp, _ := manager.Put(fmt.Sprintf("/api/v0/teams/%s/gchat-space", teamID), bytes.NewReader(body))
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", resp.StatusCode)
		}

		listResp, _ := manager.Get("/api/v0/group-settings/gchat-spaces")
		defer listResp.Body.Close()
		var spaces []map[string]any
		json.NewDecoder(listResp.Body).Decode(&spaces)
		for _, s := range spaces {
			if s["name"] == "spaces/AAA111" && s["bot_is_member"] != true {
				t.Errorf("expected bot_is_member:true for linked space, got %v", s)
			}
		}
	})

	// --- Unlink team from space ---
	t.Run("clear team space removes bot_is_member", func(t *testing.T) {
		resp, _ := manager.Delete(fmt.Sprintf("/api/v0/teams/%s/gchat-space", teamID))
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", resp.StatusCode)
		}

		listResp, _ := manager.Get("/api/v0/group-settings/gchat-spaces")
		defer listResp.Body.Close()
		var spaces []map[string]any
		json.NewDecoder(listResp.Body).Decode(&spaces)
		for _, s := range spaces {
			if s["name"] == "spaces/AAA111" && s["bot_is_member"] == true {
				t.Errorf("expected bot_is_member:false after unlink, got %v", s)
			}
		}
	})

	// --- Re-link to test key-delete clears mappings ---
	t.Run("delete key clears team space mappings and removes gchat from channels", func(t *testing.T) {
		linkBody, _ := json.Marshal(map[string]any{"gchat_space_id": "spaces/AAA111"})
		manager.Put(fmt.Sprintf("/api/v0/teams/%s/gchat-space", teamID), bytes.NewReader(linkBody))

		delResp, _ := manager.Delete("/api/v0/group-settings/gchat-key")
		defer delResp.Body.Close()
		if delResp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", delResp.StatusCode)
		}

		gsResp, _ := manager.Get("/api/v0/group-settings")
		defer gsResp.Body.Close()
		var gs map[string]any
		json.NewDecoder(gsResp.Body).Decode(&gs)
		if gs["gchat_configured"] == true {
			t.Error("expected gchat_configured:false after delete")
		}
		channels, _ := gs["notification_channels"].([]any)
		for _, ch := range channels {
			if ch == "gchat" {
				t.Error("gchat should have been removed from notification_channels after key delete")
			}
		}

		teamResp, _ := manager.Get(fmt.Sprintf("/api/v0/teams/%s/notification-settings", teamID))
		defer teamResp.Body.Close()
		var team map[string]any
		json.NewDecoder(teamResp.Body).Decode(&team)
		if team["gchat_space_id"] != "" && team["gchat_space_id"] != nil {
			t.Errorf("expected gchat_space_id cleared after key delete, got %v", team["gchat_space_id"])
		}
	})
}
