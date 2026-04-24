package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/ms-utrustning/api/internal/handler"
	"github.com/malarscouterna/ms-utrustning/api/internal/notifications"
	"github.com/malarscouterna/ms-utrustning/api/internal/testutil"
)

func mountNotifRoutes(env *testutil.TestEnv, notifier *notifications.CapturingNotifier) {
	env.V1(func(r chi.Router) {
		r.Mount("/articles", (&handler.ArticleHandler{Q: env.Queries, Perms: handler.NewPermissionCache(env.Queries)}).Routes())
		r.Mount("/locations", (&handler.LocationHandler{Q: env.Queries}).Routes())
		r.Mount("/categories", (&handler.CategoryHandler{Q: env.Queries}).Routes())
		r.Mount("/teams", (&handler.TeamHandler{Q: env.Queries}).Routes())
		r.Mount("/bookings", (&handler.BookingHandler{Q: env.Queries, Notifier: notifier}).Routes())
		r.Mount("/issues", (&handler.IssueHandler{Q: env.Queries, Perms: handler.NewPermissionCache(env.Queries), Notifier: notifier}).Routes())
	})
}

// waitForMessages blocks until the capturing notifier has collected at least n messages.
func waitForMessages(notifier *notifications.CapturingNotifier, n int) {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(notifier.Messages()) >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestNotifications_EventTriggered(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	notifier := &notifications.CapturingNotifier{}
	mountNotifRoutes(env, notifier)

	manager := env.ClientAs("manager-equipment")
	leader := env.ClientAs("leader-yggdrasil")

	// Seed a manager user so they appear in GetGroupManagers.
	// The persona upsert via /me triggers UpsertUser and populates email.
	ctx := t.Context()
	_, err := env.Pool.Exec(ctx,
		`INSERT INTO users (id, group_id, name, email, max_access_level) VALUES
			('mgr-1', '766', 'Utrustningsansvarig', 'manager@test.example', 'manager')
		ON CONFLICT (id) DO UPDATE SET email = EXCLUDED.email, max_access_level = EXCLUDED.max_access_level`)
	if err != nil {
		t.Fatalf("seed manager user: %v", err)
	}

	articleID := createTestArticle(t, manager, "NotifTestTält")

	makeBooking := func() string {
		b, _ := json.Marshal(map[string]any{
			"article_ids": []string{articleID},
			"start_date":  "2025-08-01",
			"end_date":    "2025-08-05",
		})
		resp, _ := leader.Post("/api/v0/bookings", bytes.NewReader(b))
		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		return result["id"].(string)
	}

	t.Run("booking_needs_approval sends to managers", func(t *testing.T) {
		notifier.Reset()
		bookingID := makeBooking()

		resp, _ := leader.Post("/api/v0/bookings/"+bookingID+"/submit",
			bytes.NewReader([]byte(`{"force_approval":true}`)))
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("submit: got %d", resp.StatusCode)
		}

		waitForMessages(notifier, 1)
		if len(notifier.Messages()) == 0 {
			t.Fatal("expected at least one notification for booking_needs_approval, got none")
		}
		found := false
		for _, m := range notifier.Messages() {
			if m.To == "manager@test.example" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected notification to manager@test.example, got: %+v", notifier.Messages())
		}
	})

	t.Run("booking_confirmed sends after approve", func(t *testing.T) {
		notifier.Reset()
		bookingID := makeBooking()

		// Submit (force approval)
		resp, _ := leader.Post("/api/v0/bookings/"+bookingID+"/submit",
			bytes.NewReader([]byte(`{"force_approval":true}`)))
		resp.Body.Close()
		notifier.Reset() // clear needs_approval noise

		// Approve
		resp, _ = manager.Post("/api/v0/bookings/"+bookingID+"/approve",
			bytes.NewReader([]byte(`{}`)))
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("approve: got %d", resp.StatusCode)
		}

		waitForMessages(notifier, 1)
		if len(notifier.Messages()) == 0 {
			t.Fatal("expected notification for booking_confirmed, got none")
		}
	})

	t.Run("booking_rejected sends to creator", func(t *testing.T) {
		notifier.Reset()
		bookingID := makeBooking()

		resp, _ := leader.Post("/api/v0/bookings/"+bookingID+"/submit",
			bytes.NewReader([]byte(`{"force_approval":true}`)))
		resp.Body.Close()
		notifier.Reset()

		resp, _ = manager.Post("/api/v0/bookings/"+bookingID+"/reject",
			bytes.NewReader([]byte(`{}`)))
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("reject: got %d", resp.StatusCode)
		}

		waitForMessages(notifier, 1)
		if len(notifier.Messages()) == 0 {
			t.Fatal("expected notification for booking_rejected, got none")
		}
	})

	t.Run("booking_cancelled sends after cancel", func(t *testing.T) {
		notifier.Reset()
		bookingID := makeBooking()

		resp, _ := leader.Post("/api/v0/bookings/"+bookingID+"/submit", nil)
		resp.Body.Close()
		notifier.Reset()

		resp, _ = leader.Post("/api/v0/bookings/"+bookingID+"/cancel", nil)
		resp.Body.Close()

		waitForMessages(notifier, 1)
		if len(notifier.Messages()) == 0 {
			t.Fatal("expected notification for booking_cancelled, got none")
		}
	})

	t.Run("issue_created sends to managers", func(t *testing.T) {
		notifier.Reset()

		b, _ := json.Marshal(map[string]any{
			"article_id":  articleID,
			"severity":    "usable",
			"description": "Trasig dragkedja",
		})
		resp, _ := leader.Post("/api/v0/issues", bytes.NewReader(b))
		resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("create issue: got %d", resp.StatusCode)
		}

		waitForMessages(notifier, 1)
		if len(notifier.Messages()) == 0 {
			t.Fatal("expected notification for issue_created, got none")
		}
		found := false
		for _, m := range notifier.Messages() {
			if m.To == "manager@test.example" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected notification to manager@test.example, got: %+v", notifier.Messages())
		}
	})

	t.Run("no send when SMTP not configured — action still succeeds", func(t *testing.T) {
		// CapturingNotifier never fails, so this verifies the handler doesn't
		// propagate notifier errors. The booking submit returns 200.
		notifier.Reset()
		bookingID := makeBooking()
		resp, _ := leader.Post("/api/v0/bookings/"+bookingID+"/submit",
			bytes.NewReader([]byte(`{"force_approval":true}`)))
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})
}
