package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/ms-utrustning/api/internal/handler"
	"github.com/malarscouterna/ms-utrustning/api/internal/testutil"
)

func mountNotifPrefsRoutes(env *testutil.TestEnv) {
	notifPrefs := &handler.NotificationPrefsHandler{Q: env.Queries}
	me := &handler.MeHandler{Q: env.Queries, Perms: handler.NewPermissionCache(env.Queries), NotifPrefs: notifPrefs}
	env.V1(func(r chi.Router) {
		r.Mount("/me", me.Routes())
		r.Mount("/group-settings/notification-defaults", notifPrefs.GroupRoutes())
	})
}

func getPrefs(t *testing.T, client *testutil.TestClient) map[string]map[string]map[string]any {
	t.Helper()
	resp, err := client.Get("/api/v0/me/notification-prefs")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body struct {
		Prefs map[string]map[string]map[string]any `json:"prefs"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	return body.Prefs
}

func TestNotificationPrefs(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountNotifPrefsRoutes(env)

	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")

	t.Run("GET returns system defaults when no rows exist", func(t *testing.T) {
		prefs := getPrefs(t, leader)

		// booking_confirmed should default true for non-managers
		if v, ok := prefs["booking_confirmed"]["email"]; !ok || v["enabled"] != true {
			t.Errorf("booking_confirmed email should default to true for non-manager, got %v", v)
		}
		if src := prefs["booking_confirmed"]["email"]["source"]; src != "system_default" {
			t.Errorf("expected source system_default, got %v", src)
		}
	})

	t.Run("manager defaults differ for booking_needs_approval and issue_created", func(t *testing.T) {
		mgrPrefs := getPrefs(t, manager)
		ldrPrefs := getPrefs(t, leader)

		if mgrPrefs["booking_needs_approval"]["email"]["enabled"] != true {
			t.Error("manager should default booking_needs_approval email to true")
		}
		if ldrPrefs["booking_needs_approval"]["email"]["enabled"] != false {
			t.Error("non-manager should default booking_needs_approval email to false")
		}
		if mgrPrefs["issue_created"]["email"]["enabled"] != true {
			t.Error("manager should default issue_created email to true")
		}
		if ldrPrefs["issue_created"]["email"]["enabled"] != false {
			t.Error("non-manager should default issue_created email to false")
		}
	})

	t.Run("PUT overrides a pref; GET shows source user", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"booking_confirmed": map[string]bool{"email": false},
		})
		resp, err := leader.Put("/api/v0/me/notification-prefs", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", resp.StatusCode)
		}

		prefs := getPrefs(t, leader)
		if prefs["booking_confirmed"]["email"]["enabled"] != false {
			t.Error("expected booking_confirmed to be overridden to false")
		}
		if prefs["booking_confirmed"]["email"]["source"] != "user" {
			t.Errorf("expected source user, got %v", prefs["booking_confirmed"]["email"]["source"])
		}
		// unrelated pref unchanged
		if prefs["booking_reminder"]["email"]["source"] != "system_default" {
			t.Errorf("unrelated pref should still be system_default, got %v", prefs["booking_reminder"]["email"]["source"])
		}
	})

	t.Run("DELETE reverts to system defaults", func(t *testing.T) {
		// First set a pref
		body, _ := json.Marshal(map[string]any{
			"booking_confirmed": map[string]bool{"email": false},
		})
		resp, _ := leader.Put("/api/v0/me/notification-prefs", bytes.NewReader(body))
		resp.Body.Close()

		// Then delete
		resp, err := leader.Delete("/api/v0/me/notification-prefs")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", resp.StatusCode)
		}

		prefs := getPrefs(t, leader)
		if prefs["booking_confirmed"]["email"]["source"] != "system_default" {
			t.Errorf("after DELETE, source should be system_default, got %v", prefs["booking_confirmed"]["email"]["source"])
		}
		if prefs["booking_confirmed"]["email"]["enabled"] != true {
			t.Error("after DELETE, booking_confirmed should revert to true (system default)")
		}
	})

	t.Run("group defaults override system defaults for users with no user-level rows", func(t *testing.T) {
		// Manager sets group default for non-manager users: booking_reminder email = false
		body, _ := json.Marshal(map[string]any{
			"user": map[string]any{"booking_reminder": map[string]bool{"email": false}},
		})
		resp, err := manager.Put("/api/v0/group-settings/notification-defaults", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", resp.StatusCode)
		}

		// Leader (no user-level pref) should now see group_default
		prefs := getPrefs(t, leader)
		if prefs["booking_reminder"]["email"]["enabled"] != false {
			t.Error("group default should override system default (false)")
		}
		if prefs["booking_reminder"]["email"]["source"] != "group_default" {
			t.Errorf("expected source group_default, got %v", prefs["booking_reminder"]["email"]["source"])
		}
	})

	t.Run("user-level pref overrides group default", func(t *testing.T) {
		// Group default for booking_reminder is already false from previous subtest.
		// Leader sets their own pref to true.
		body, _ := json.Marshal(map[string]any{
			"booking_reminder": map[string]bool{"email": true},
		})
		resp, _ := leader.Put("/api/v0/me/notification-prefs", bytes.NewReader(body))
		resp.Body.Close()

		prefs := getPrefs(t, leader)
		if prefs["booking_reminder"]["email"]["enabled"] != true {
			t.Error("user pref should override group default")
		}
		if prefs["booking_reminder"]["email"]["source"] != "user" {
			t.Errorf("expected source user, got %v", prefs["booking_reminder"]["email"]["source"])
		}

		// Clean up
		resp, _ = leader.Delete("/api/v0/me/notification-prefs")
		resp.Body.Close()
	})

	t.Run("group defaults GET returns current defaults", func(t *testing.T) {
		resp, err := manager.Get("/api/v0/group-settings/notification-defaults")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var body struct {
			User map[string]map[string]bool `json:"user"`
		}
		json.NewDecoder(resp.Body).Decode(&body)
		// booking_reminder was set to false for user role in previous subtest
		if v, ok := body.User["booking_reminder"]["email"]; !ok || v != false {
			t.Errorf("expected booking_reminder email=false in group defaults user, got %v", body.User)
		}
	})

	t.Run("group defaults endpoint: leader gets 403", func(t *testing.T) {
		resp, err := leader.Get("/api/v0/group-settings/notification-defaults")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}

		body, _ := json.Marshal(map[string]any{})
		resp, err = leader.Put("/api/v0/group-settings/notification-defaults", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})
}
