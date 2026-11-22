package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/malarscouterna/scoutrustning/api/internal/notifications"
	"github.com/malarscouterna/scoutrustning/api/internal/testutil"
)

func TestBookingItemsChangedNotification_OnResubmit(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	notifier := &notifications.CapturingNotifier{}
	mountNotifRoutes(env, notifier)

	// A dedicated manager gives a stable recipient for the first submission and
	// its follow-up diff.
	const managerEmail = "diffmanager@test.example"
	_, err := env.Pool.Exec(context.Background(), `
		INSERT INTO users (id, group_id, name, email, max_access_level)
		VALUES ('diff-mgr', '766', 'Diff Manager', $1, 'manager')
		ON CONFLICT (id, group_id) DO UPDATE SET email = EXCLUDED.email
	`, managerEmail)
	if err != nil {
		t.Fatalf("seed manager: %v", err)
	}

	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")
	createTestArticle(t, manager, "DiffTält")
	createTestArticle(t, manager, "DiffStormkök")

	teamID := getTeamID(t, leader, "Yggdrasil")
	createBody, _ := json.Marshal(map[string]any{
		"start_date":      "2026-11-01",
		"end_date":        "2026-11-03",
		"title":           "Diff notification",
		"used_by_team_id": teamID,
	})
	resp, err := leader.Post("/api/v0/bookings", bytes.NewReader(createBody))
	if err != nil {
		t.Fatal(err)
	}
	var booking map[string]any
	if err := json.Unmarshal(expectStatus(t, resp, http.StatusCreated), &booking); err != nil {
		t.Fatal(err)
	}
	bookingID := booking["id"].(string)

	addItem := func(commercialName string) string {
		t.Helper()
		b, _ := json.Marshal(map[string]any{"commercial_name": commercialName, "quantity": 1})
		resp, err := leader.Post("/api/v0/bookings/"+bookingID+"/items", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		var items []map[string]any
		if err := json.Unmarshal(expectStatus(t, resp, http.StatusCreated), &items); err != nil {
			t.Fatal(err)
		}
		return items[0]["id"].(string)
	}
	submit := func() {
		t.Helper()
		resp, err := leader.Post("/api/v0/bookings/"+bookingID+"/submit", bytes.NewReader([]byte(`{"force_approval":true}`)))
		if err != nil {
			t.Fatal(err)
		}
		expectStatus(t, resp, http.StatusOK)
	}

	firstItemID := addItem("DiffTält")
	submit()

	first := waitForMessage(t, notifier, func(m notifications.Message) bool {
		return m.To == managerEmail && m.MessageID != ""
	})
	if strings.Contains(first.Subject, "uppdaterad") || strings.Contains(first.Subject, "updated") {
		t.Fatalf("first submission must not send a change notification, got %q", first.Subject)
	}
	notifier.Reset()

	// Remove the first item and add a different one. Adding reopens the booking
	// to draft; the next submit is the one that should carry the diff.
	resp, err = leader.Delete("/api/v0/bookings/" + bookingID + "/items/" + firstItemID)
	if err != nil {
		t.Fatal(err)
	}
	expectStatus(t, resp, http.StatusNoContent)
	addItem("DiffStormkök")
	submit()

	diff := waitForMessage(t, notifier, func(m notifications.Message) bool {
		return m.To == managerEmail && strings.Contains(m.Subject, "uppdaterad")
	})
	if diff.InReplyTo != first.MessageID {
		t.Errorf("expected diff to reply to first submission %q, got %q", first.MessageID, diff.InReplyTo)
	}
	for _, want := range []string{"DiffTält", "DiffStormkök", "Tillagda", "Borttagna"} {
		if !strings.Contains(diff.Body, want) {
			t.Errorf("expected HTML diff to contain %q", want)
		}
		if !strings.Contains(diff.TextBody, want) {
			t.Errorf("expected text diff to contain %q", want)
		}
	}
}
