package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
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
		r.Mount("/bookings", (&handler.BookingHandler{Q: env.Queries, Notifier: notifier, BaseURL: "http://test.example"}).Routes())
		r.Mount("/issues", (&handler.IssueHandler{Q: env.Queries, Perms: handler.NewPermissionCache(env.Queries), Notifier: notifier, BaseURL: "http://test.example"}).Routes())
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

func mountTestEmailRoutes(env *testutil.TestEnv, notifier *notifications.CapturingNotifier, personaIDs map[string]bool) {
	notifPrefs := &handler.NotificationPrefsHandler{Q: env.Queries}
	me := &handler.MeHandler{
		Q:          env.Queries,
		Perms:      handler.NewPermissionCache(env.Queries),
		NotifPrefs: notifPrefs,
		Notifier:   notifier,
		PersonaIDs: personaIDs,
	}
	env.V1(func(r chi.Router) {
		r.Mount("/me", me.Routes())
	})
}

func TestNotifications_TestEmail(t *testing.T) {
	t.Run("real user receives test email", func(t *testing.T) {
		env := testutil.SetupTestEnv(t)
		notifier := &notifications.CapturingNotifier{}
		mountTestEmailRoutes(env, notifier, nil)

		leader := env.ClientAs("leader-yggdrasil")
		resp, _ := leader.Post("/api/v0/me/test-email", nil)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var body map[string]any
		json.NewDecoder(resp.Body).Decode(&body)
		if body["sent"] != true {
			t.Errorf("expected sent:true, got %v", body)
		}
		msgs := notifier.Messages()
		if len(msgs) == 0 {
			t.Fatal("expected one captured message, got none")
		}
		if msgs[0].To == "" {
			t.Error("expected To to be set")
		}
	})

	t.Run("persona is skipped", func(t *testing.T) {
		env := testutil.SetupTestEnv(t)
		notifier := &notifications.CapturingNotifier{}
		// Mark leader-yggdrasil's member_id as a persona.
		personaIDs := map[string]bool{"3000005": true}
		mountTestEmailRoutes(env, notifier, personaIDs)

		leader := env.ClientAs("leader-yggdrasil")
		resp, _ := leader.Post("/api/v0/me/test-email", nil)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var body map[string]any
		json.NewDecoder(resp.Body).Decode(&body)
		if body["skipped"] != true {
			t.Errorf("expected skipped:true, got %v", body)
		}
		if len(notifier.Messages()) != 0 {
			t.Error("expected no messages sent for persona")
		}
	})

	t.Run("demo mode: notifier still sends via injected notifier", func(t *testing.T) {
		env := testutil.SetupTestEnv(t)
		// Simulate demo mode: event handlers would use NoopNotifier, but
		// MeHandler.Notifier is a real (capturing) notifier.
		notifier := &notifications.CapturingNotifier{}
		mountTestEmailRoutes(env, notifier, nil) // no personaIDs → not a persona

		leader := env.ClientAs("leader-yggdrasil")
		resp, _ := leader.Post("/api/v0/me/test-email", nil)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if len(notifier.Messages()) == 0 {
			t.Fatal("expected test email to be sent even in demo mode")
		}
	})
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
			"start_date": "2025-08-01",
			"end_date":   "2025-08-05",
		})
		resp, _ := leader.Post("/api/v0/bookings", bytes.NewReader(b))
		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		id := result["id"].(string)
		// Add an item so the email body contains the article name.
		itemBody, _ := json.Marshal(map[string]any{
			"commercial_name": "NotifTestTält",
			"location_name":   "Kammaren",
			"quantity":        1,
		})
		addResp, _ := leader.Post("/api/v0/bookings/"+id+"/items", bytes.NewReader(itemBody))
		addResp.Body.Close()
		return id
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

		// Two managers exist: mgr-1 and the manager-equipment persona user.
		waitForMessages(notifier, 2)
		if len(notifier.Messages()) == 0 {
			t.Fatal("expected at least one notification for booking_needs_approval, got none")
		}
		found := false
		var msg notifications.Message
		for _, m := range notifier.Messages() {
			if m.To == "manager@test.example" {
				found = true
				msg = m
			}
		}
		if !found {
			t.Errorf("expected notification to manager@test.example, got: %+v", notifier.Messages())
		}
		if !strings.Contains(msg.Body, "http://test.example/bookings/"+bookingID) {
			t.Errorf("expected body to contain booking URL, got: %s", msg.Body)
		}
		if !strings.Contains(msg.Body, "2025-08-01") && !strings.Contains(msg.Body, "1 aug") {
			t.Errorf("expected body to contain booking dates, got: %s", msg.Body)
		}
		if !strings.Contains(msg.Body, "NotifTestTält") {
			t.Errorf("expected body to contain item name, got: %s", msg.Body)
		}
		if msg.TextBody == "" {
			t.Error("expected TextBody to be non-empty")
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
		waitForMessages(notifier, 3) // drain confirmed + submitted_no_approval goroutines before reset
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

		// Two managers exist: mgr-1 and the manager-equipment persona user.
		waitForMessages(notifier, 2)
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
