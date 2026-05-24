package tests

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/malarscouterna/scoutrustning/api/internal/notifications"
	"github.com/malarscouterna/scoutrustning/api/internal/testutil"
)

// seedSchedulerUsers inserts users with the team_ids and notification_prefs
// needed for scheduler tests. Returns (creatorID, teamMemberID, managerID).
//
// creator     — leader in Yggdrasil
// teamMember  — another Yggdrasil member
// manager     — equipment manager (max_access_level = "manager")
func seedSchedulerUsers(t *testing.T, env *testutil.TestEnv) (creatorID, teamMemberID, managerID string) {
	t.Helper()
	ctx := context.Background()

	// Resolve Yggdrasil team UUID from the seed data
	teams, err := env.Pool.Query(ctx, `SELECT id FROM teams WHERE group_id = '766' AND name = 'Yggdrasil'`)
	if err != nil {
		t.Fatalf("query yggdrasil team: %v", err)
	}
	defer teams.Close()
	var yggdrasilID pgtype.UUID
	if teams.Next() {
		teams.Scan(&yggdrasilID)
	}
	teams.Close()

	// Resolve manager team UUID
	mgr, err := env.Pool.Query(ctx, `SELECT id FROM teams WHERE group_id = '766' AND name = 'Utrustningsgruppen'`)
	if err != nil {
		t.Fatalf("query manager team: %v", err)
	}
	defer mgr.Close()
	var managerTeamID pgtype.UUID
	if mgr.Next() {
		mgr.Scan(&managerTeamID)
	}
	mgr.Close()

	creatorID = "sched-creator-1"
	teamMemberID = "sched-member-1"
	managerID = "sched-manager-1"

	_, err = env.Pool.Exec(ctx, `
		INSERT INTO users (id, group_id, name, email, max_access_level, team_ids, notification_prefs)
		VALUES
			($1, '766', 'Scheduler Creator', 'sched-creator@test.example', 'book', $4::uuid[], '{}'),
			($2, '766', 'Scheduler Member',  'sched-member@test.example',  'book', $4::uuid[], '{}'),
			($3, '766', 'Scheduler Manager', 'sched-manager@test.example', 'manager', $5::uuid[], '{}')
	`,
		creatorID, teamMemberID, managerID,
		[]pgtype.UUID{yggdrasilID},
		[]pgtype.UUID{managerTeamID},
	)
	if err != nil {
		t.Fatalf("seed scheduler users: %v", err)
	}
	return
}

// seedBooking inserts a booking and returns its UUID.
func seedBooking(t *testing.T, env *testutil.TestEnv, creatorID, status string, startDate, endDate time.Time, teamID pgtype.UUID) pgtype.UUID {
	t.Helper()
	ctx := context.Background()

	var id pgtype.UUID
	err := env.Pool.QueryRow(ctx, `
		INSERT INTO bookings (group_id, created_by, used_by_team_id, status, start_date, end_date, notes)
		VALUES ('766', $1, $2, $3, $4, $5, '')
		RETURNING id
	`, creatorID, teamID, status,
		pgtype.Date{Time: startDate, Valid: true},
		pgtype.Date{Time: endDate, Valid: true},
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed booking: %v", err)
	}
	return id
}

func TestNotifications_Scheduled(t *testing.T) {
	today := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	tomorrow := today.AddDate(0, 0, 1)
	yesterday := today.AddDate(0, 0, -1)
	todayPG := pgtype.Date{Time: today, Valid: true}

	// Resolve Yggdrasil team for bookings
	resolveYggdrasil := func(env *testutil.TestEnv) pgtype.UUID {
		var id pgtype.UUID
		env.Pool.QueryRow(context.Background(), `SELECT id FROM teams WHERE group_id = '766' AND name = 'Yggdrasil'`).Scan(&id)
		return id
	}

	t.Run("reminder sends to creator and team members", func(t *testing.T) {
		env := testutil.SetupTestEnv(t)
		creatorID, memberID, _ := seedSchedulerUsers(t, env)
		yggdrasil := resolveYggdrasil(env)
		seedBooking(t, env, creatorID, "confirmed", tomorrow, tomorrow.AddDate(0, 0, 3), yggdrasil)

		notifier := &notifications.CapturingNotifier{}
		notifications.SendReminders(context.Background(), env.Queries, notifier, todayPG, "http://test.example")

		msgs := notifier.Messages()
		if len(msgs) < 2 {
			t.Fatalf("expected ≥2 reminder messages (creator + member), got %d", len(msgs))
		}
		emails := map[string]bool{}
		for _, m := range msgs {
			emails[m.To] = true
		}
		if !emails["sched-creator@test.example"] {
			t.Error("expected reminder to creator, not found")
		}
		if !emails["sched-member@test.example"] {
			t.Error("expected reminder to team member, not found")
		}
		// Body should contain the booking URL
		for _, m := range msgs {
			if m.To == "sched-creator@test.example" && m.Body == "" {
				t.Error("expected non-empty email body")
			}
		}
		_ = memberID
	})

	t.Run("reminder only fires for bookings starting tomorrow", func(t *testing.T) {
		env := testutil.SetupTestEnv(t)
		creatorID, _, _ := seedSchedulerUsers(t, env)
		yggdrasil := resolveYggdrasil(env)
		// Booking starts today — should NOT get a reminder
		seedBooking(t, env, creatorID, "confirmed", today, today.AddDate(0, 0, 3), yggdrasil)

		notifier := &notifications.CapturingNotifier{}
		notifications.SendReminders(context.Background(), env.Queries, notifier, todayPG, "http://test.example")

		if len(notifier.Messages()) != 0 {
			t.Errorf("expected 0 reminders for today-starting booking, got %d", len(notifier.Messages()))
		}
	})

	t.Run("reminder respects booking_reminder pref off", func(t *testing.T) {
		env := testutil.SetupTestEnv(t)
		creatorID, _, _ := seedSchedulerUsers(t, env)
		// Disable booking_reminder for creator
		_, err := env.Pool.Exec(context.Background(),
			`UPDATE users SET notification_prefs = '{"booking_reminder":{"personal_email_policy":"never"}}' WHERE id = $1`, creatorID)
		if err != nil {
			t.Fatalf("update pref: %v", err)
		}
		// Personal booking (no team) — only creator is recipient
		seedBooking(t, env, creatorID, "confirmed", tomorrow, tomorrow.AddDate(0, 0, 1), pgtype.UUID{})

		notifier := &notifications.CapturingNotifier{}
		notifications.SendReminders(context.Background(), env.Queries, notifier, todayPG, "http://test.example")

		for _, m := range notifier.Messages() {
			if m.To == "sched-creator@test.example" {
				t.Errorf("expected no reminder to creator with pref off, got one")
			}
		}
	})

	t.Run("overdue sends to creator and team members", func(t *testing.T) {
		env := testutil.SetupTestEnv(t)
		creatorID, memberID, _ := seedSchedulerUsers(t, env)
		yggdrasil := resolveYggdrasil(env)
		seedBooking(t, env, creatorID, "picked_up", yesterday.AddDate(0, 0, -5), yesterday, yggdrasil)

		notifier := &notifications.CapturingNotifier{}
		notifications.SendOverdueAlerts(context.Background(), env.Queries, notifier, todayPG, "http://test.example")

		emails := map[string]bool{}
		for _, m := range notifier.Messages() {
			emails[m.To] = true
		}
		if !emails["sched-creator@test.example"] {
			t.Error("expected overdue alert to creator")
		}
		if !emails["sched-member@test.example"] {
			t.Error("expected overdue alert to team member")
		}
		_ = memberID
	})

	t.Run("overdue deduplicates: second run sends nothing", func(t *testing.T) {
		env := testutil.SetupTestEnv(t)
		creatorID, _, _ := seedSchedulerUsers(t, env)
		yggdrasil := resolveYggdrasil(env)
		seedBooking(t, env, creatorID, "picked_up", yesterday.AddDate(0, 0, -5), yesterday, yggdrasil)

		notifier := &notifications.CapturingNotifier{}
		notifications.SendOverdueAlerts(context.Background(), env.Queries, notifier, todayPG, "http://test.example")
		firstCount := len(notifier.Messages())
		if firstCount == 0 {
			t.Fatal("expected overdue messages on first run, got none")
		}

		notifier.Reset()
		notifications.SendOverdueAlerts(context.Background(), env.Queries, notifier, todayPG, "http://test.example")
		if len(notifier.Messages()) != 0 {
			t.Errorf("expected 0 messages on second run (dedup), got %d", len(notifier.Messages()))
		}
	})

	t.Run("not-yet-overdue booking excluded", func(t *testing.T) {
		env := testutil.SetupTestEnv(t)
		creatorID, _, _ := seedSchedulerUsers(t, env)
		yggdrasil := resolveYggdrasil(env)
		// end_date = today — not overdue (overdue means end_date < today)
		seedBooking(t, env, creatorID, "picked_up", today.AddDate(0, 0, -3), today, yggdrasil)

		notifier := &notifications.CapturingNotifier{}
		notifications.SendOverdueAlerts(context.Background(), env.Queries, notifier, todayPG, "http://test.example")

		if len(notifier.Messages()) != 0 {
			t.Errorf("expected 0 overdue alerts for end_date=today, got %d", len(notifier.Messages()))
		}
	})

	t.Run("manager receives overdue when opted in", func(t *testing.T) {
		env := testutil.SetupTestEnv(t)
		creatorID, _, managerID := seedSchedulerUsers(t, env)
		yggdrasil := resolveYggdrasil(env)
		// Opt manager in to booking_overdue
		_, err := env.Pool.Exec(context.Background(),
			`UPDATE users SET notification_prefs = '{"booking_overdue":{"email":true}}' WHERE id = $1`, managerID)
		if err != nil {
			t.Fatalf("update manager pref: %v", err)
		}
		seedBooking(t, env, creatorID, "picked_up", yesterday.AddDate(0, 0, -5), yesterday, yggdrasil)

		notifier := &notifications.CapturingNotifier{}
		notifications.SendOverdueAlerts(context.Background(), env.Queries, notifier, todayPG, "http://test.example")

		found := false
		for _, m := range notifier.Messages() {
			if m.To == "sched-manager@test.example" {
				found = true
			}
		}
		if !found {
			t.Error("expected overdue alert to opted-in manager")
		}
		_ = creatorID
	})
}
