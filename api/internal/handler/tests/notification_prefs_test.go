package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/scoutrustning/api/internal/handler"
	"github.com/malarscouterna/scoutrustning/api/internal/testutil"
)

func mountNotifPrefsRoutes(env *testutil.TestEnv) {
	notifPrefs := &handler.NotificationPrefsHandler{Q: env.Queries}
	me := &handler.MeHandler{Q: env.Queries, Perms: handler.NewPermissionCache(env.Queries), NotifPrefs: notifPrefs}
	env.V1(func(r chi.Router) {
		r.Mount("/me", me.Routes())
		r.Mount("/group-settings/notification-defaults", notifPrefs.GroupRoutes())
	})
}

// getPrefs returns the resolved prefs map: event → {policy, source, default_policy}.
func getPrefs(t *testing.T, client *testutil.TestClient) map[string]map[string]any {
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
		Prefs map[string]map[string]any `json:"prefs"`
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

		// booking_confirmed defaults to if_no_broadcast (system default)
		if v := prefs["booking_confirmed"]["policy"]; v != "if_no_broadcast" {
			t.Errorf("booking_confirmed policy should default to if_no_broadcast, got %v", v)
		}
		if src := prefs["booking_confirmed"]["source"]; src != "system_default" {
			t.Errorf("expected source system_default, got %v", src)
		}
	})

	t.Run("booking_needs_approval and issue_created have system defaults", func(t *testing.T) {
		mgrPrefs := getPrefs(t, manager)
		ldrPrefs := getPrefs(t, leader)

		// Both get if_no_broadcast from system defaults (isManager param is not yet wired up)
		for _, key := range []string{"booking_needs_approval", "issue_created"} {
			if mgrPrefs[key]["policy"] != "if_no_broadcast" {
				t.Errorf("manager %s policy should be if_no_broadcast, got %v", key, mgrPrefs[key]["policy"])
			}
			if ldrPrefs[key]["policy"] != "if_no_broadcast" {
				t.Errorf("leader %s policy should be if_no_broadcast, got %v", key, ldrPrefs[key]["policy"])
			}
		}
	})

	t.Run("PUT overrides a pref; GET shows source user", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"booking_confirmed": map[string]string{"personal_email_policy": "never"},
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
		if prefs["booking_confirmed"]["policy"] != "never" {
			t.Error("expected booking_confirmed to be overridden to never")
		}
		if prefs["booking_confirmed"]["source"] != "user" {
			t.Errorf("expected source user, got %v", prefs["booking_confirmed"]["source"])
		}
		// unrelated pref unchanged
		if prefs["booking_reminder"]["source"] != "system_default" {
			t.Errorf("unrelated pref should still be system_default, got %v", prefs["booking_reminder"]["source"])
		}
	})

	t.Run("DELETE reverts to system defaults", func(t *testing.T) {
		// First set a pref
		body, _ := json.Marshal(map[string]any{
			"booking_confirmed": map[string]string{"personal_email_policy": "never"},
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
		if prefs["booking_confirmed"]["source"] != "system_default" {
			t.Errorf("after DELETE, source should be system_default, got %v", prefs["booking_confirmed"]["source"])
		}
		if prefs["booking_confirmed"]["policy"] != "if_no_broadcast" {
			t.Errorf("after DELETE, booking_confirmed should revert to if_no_broadcast, got %v", prefs["booking_confirmed"]["policy"])
		}
	})

	t.Run("group defaults override system defaults for users with no user-level rows", func(t *testing.T) {
		// Manager sets group default: booking_reminder personal_email_policy = never
		body, _ := json.Marshal(map[string]any{
			"defaults": map[string]any{
				"booking_reminder": map[string]string{"personal_email_policy": "never"},
			},
			"default_gruppkanal_channels": []string{},
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
		if prefs["booking_reminder"]["policy"] != "never" {
			t.Errorf("group default should override system default (never), got %v", prefs["booking_reminder"]["policy"])
		}
		if prefs["booking_reminder"]["source"] != "group_default" {
			t.Errorf("expected source group_default, got %v", prefs["booking_reminder"]["source"])
		}
	})

	t.Run("user-level pref overrides group default", func(t *testing.T) {
		// Group default for booking_reminder is already never from previous subtest.
		// Leader sets their own pref to always.
		body, _ := json.Marshal(map[string]any{
			"booking_reminder": map[string]string{"personal_email_policy": "always"},
		})
		resp, _ := leader.Put("/api/v0/me/notification-prefs", bytes.NewReader(body))
		resp.Body.Close()

		prefs := getPrefs(t, leader)
		if prefs["booking_reminder"]["policy"] != "always" {
			t.Errorf("user pref should override group default, got %v", prefs["booking_reminder"]["policy"])
		}
		if prefs["booking_reminder"]["source"] != "user" {
			t.Errorf("expected source user, got %v", prefs["booking_reminder"]["source"])
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
			Defaults map[string]map[string]any `json:"defaults"`
		}
		json.NewDecoder(resp.Body).Decode(&body)
		// booking_reminder was set to never in previous subtest
		if v := body.Defaults["booking_reminder"]["personal_email_policy"]; v != "never" {
			t.Errorf("expected booking_reminder personal_email_policy=never in group defaults, got %v", v)
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
