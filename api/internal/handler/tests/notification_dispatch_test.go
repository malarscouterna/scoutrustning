package tests

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/malarscouterna/scoutrustning/api/internal/db"
	"github.com/malarscouterna/scoutrustning/api/internal/notifications"
	"github.com/malarscouterna/scoutrustning/api/internal/testutil"
)

// dispatchEnv holds the minimal entities needed to exercise notification dispatch.
type dispatchEnv struct {
	env        *testutil.TestEnv
	q          *db.Queries
	groupID    string
	mgrTeamID  pgtype.UUID
	unitTeamID pgtype.UUID
	managerID  string
	reporterID string
}

// setupDispatchEnv creates a fresh test env with one manager team and one unit team,
// two users (a manager and a regular user), and wires team membership.
func setupDispatchEnv(t *testing.T) *dispatchEnv {
	t.Helper()
	env := testutil.SetupTestEnv(t)
	ctx := context.Background()
	q := env.Queries

	const groupID = "766"

	// Fetch the pre-seeded team IDs.
	rows, err := env.Pool.Query(ctx,
		`SELECT id, name FROM teams WHERE group_id = $1 AND access_level IN ('manager','book')`, groupID)
	if err != nil {
		t.Fatalf("query teams: %v", err)
	}
	defer rows.Close()

	var mgrTeamID, unitTeamID pgtype.UUID
	for rows.Next() {
		var id pgtype.UUID
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			t.Fatal(err)
		}
		if name == "Utrustningsgruppen" {
			mgrTeamID = id
		}
		if name == "Yggdrasil" {
			unitTeamID = id
		}
	}
	if !mgrTeamID.Valid || !unitTeamID.Valid {
		t.Fatal("could not find required teams in seed data")
	}

	// Insert a manager user and a regular user.
	managerID := "dispatch-mgr-1"
	reporterID := "dispatch-usr-1"
	_, err = env.Pool.Exec(ctx, `
		INSERT INTO users (id, group_id, name, email, max_access_level, team_ids)
		VALUES
			($1, $3, 'Test Manager', 'manager@test.example', 'manager', ARRAY[$4::uuid]),
			($2, $3, 'Test User',    'user@test.example',    'book',    ARRAY[$5::uuid])
	`, managerID, reporterID, groupID, mgrTeamID, unitTeamID)
	if err != nil {
		t.Fatalf("insert users: %v", err)
	}

	return &dispatchEnv{
		env:        env,
		q:          q,
		groupID:    groupID,
		mgrTeamID:  mgrTeamID,
		unitTeamID: unitTeamID,
		managerID:  managerID,
		reporterID: reporterID,
	}
}

// setTeamNotifSettings sets notification_email, gchat_space_id, and gruppkanal_channels for a team.
func (d *dispatchEnv) setTeamNotifSettings(t *testing.T, teamID pgtype.UUID, notifEmail, gchatSpace string, channels []string) {
	t.Helper()
	ctx := context.Background()
	channelLiteral := "NULL"
	if channels != nil {
		channelLiteral = "ARRAY["
		for i, ch := range channels {
			if i > 0 {
				channelLiteral += ","
			}
			channelLiteral += fmt.Sprintf("'%s'", ch)
		}
		channelLiteral += "]::text[]"
	}
	emailVal := "NULL"
	if notifEmail != "" {
		emailVal = fmt.Sprintf("'%s'", notifEmail)
	}
	gchatVal := "NULL"
	if gchatSpace != "" {
		gchatVal = fmt.Sprintf("'%s'", gchatSpace)
	}
	_, err := d.env.Pool.Exec(ctx, fmt.Sprintf(
		`UPDATE teams SET notification_email = %s, gchat_space_id = %s, gruppkanal_channels = %s WHERE id = $1`,
		emailVal, gchatVal, channelLiteral,
	), teamID)
	if err != nil {
		t.Fatalf("setTeamNotifSettings: %v", err)
	}
}

func (d *dispatchEnv) setGroupDefaultChannels(t *testing.T, channels []string) {
	t.Helper()
	ctx := context.Background()
	literal := "{}"
	if len(channels) > 0 {
		literal = "{"
		for i, ch := range channels {
			if i > 0 {
				literal += ","
			}
			literal += ch
		}
		literal += "}"
	}
	_, err := d.env.Pool.Exec(ctx,
		`UPDATE group_settings SET default_gruppkanal_channels = $1 WHERE group_id = $2`,
		literal, d.groupID)
	if err != nil {
		t.Fatalf("setGroupDefaultChannels: %v", err)
	}
}

func (d *dispatchEnv) newIssue(t *testing.T) db.IssueReport {
	t.Helper()
	ctx := context.Background()
	var issue db.IssueReport
	err := d.env.Pool.QueryRow(ctx, `
		INSERT INTO issue_reports (group_id, title, description, severity, status, reporter_id)
		VALUES ($1, 'Test issue', 'Description', 'usable', 'open', $2)
		RETURNING id, group_id, title, description, severity, status, reporter_id, booking_id, created_at, updated_at
	`, d.groupID, d.reporterID).Scan(
		&issue.ID, &issue.GroupID, &issue.Title, &issue.Description,
		&issue.Severity, &issue.Status, &issue.ReporterID, &issue.BookingID,
		&issue.CreatedAt, &issue.UpdatedAt,
	)
	if err != nil {
		t.Fatalf("newIssue: %v", err)
	}
	return issue
}

// recipientEmails extracts the To addresses from captured messages.
func recipientEmails(msgs []notifications.Message) []string {
	seen := make(map[string]bool)
	var out []string
	for _, m := range msgs {
		if !seen[m.To] {
			seen[m.To] = true
			out = append(out, m.To)
		}
	}
	return out
}

func containsEmail(msgs []notifications.Message, addr string) bool {
	for _, m := range msgs {
		if m.To == addr {
			return true
		}
	}
	return false
}

// TestPersonalEmailPolicy_NoConfiguredBroadcast verifies that when a team inherits
// ["email"] from the group default but has no notification_email set, personal emails
// still reach team members (nothing to broadcast to, so suppression must not trigger).
func TestPersonalEmailPolicy_NoConfiguredBroadcast(t *testing.T) {
	d := setupDispatchEnv(t)

	// Group default says email is in Gruppkanal, but manager team has no notification_email.
	d.setGroupDefaultChannels(t, []string{"email"})
	// Manager team: no notification_email, no explicit gruppkanal_channels (inherits group default)

	issue := d.newIssue(t)
	n := &notifications.CapturingNotifier{}
	notifications.SendIssueCreated(context.Background(), d.q, n, &notifications.NoopNotifier{}, issue, "http://localhost")

	if !containsEmail(n.Messages(), "manager@test.example") {
		t.Errorf("expected personal email to manager@test.example but got: %v", recipientEmails(n.Messages()))
	}
}

// TestPersonalEmailPolicy_BroadcastEmailConfigured verifies that when a team has a
// notification_email and email is in its effective Gruppkanal, personal emails for
// PolicyIfNoBroadcast events are suppressed in favour of the broadcast.
func TestPersonalEmailPolicy_BroadcastEmailConfigured(t *testing.T) {
	d := setupDispatchEnv(t)

	d.setGroupDefaultChannels(t, []string{"email"})
	d.setTeamNotifSettings(t, d.mgrTeamID,
		"utrustning@test.example", // notification_email configured
		"",                        // no GChat
		[]string{"email"},         // explicit Gruppkanal
	)

	issue := d.newIssue(t)
	n := &notifications.CapturingNotifier{}
	notifications.SendIssueCreated(context.Background(), d.q, n, &notifications.NoopNotifier{}, issue, "http://localhost")

	// Broadcast should fire to the team address.
	if !containsEmail(n.Messages(), "utrustning@test.example") {
		t.Errorf("expected broadcast email to utrustning@test.example, got: %v", recipientEmails(n.Messages()))
	}
	// Personal email to the manager should be suppressed (policy=if_no_broadcast + broadcast configured).
	if containsEmail(n.Messages(), "manager@test.example") {
		t.Errorf("expected personal email to manager@test.example to be suppressed, but it was sent")
	}
}

// TestPersonalEmailPolicy_ExplicitEmptyGruppkanal verifies that a team with an
// explicitly empty Gruppkanal (opted out of all channels) still receives personal emails.
func TestPersonalEmailPolicy_ExplicitEmptyGruppkanal(t *testing.T) {
	d := setupDispatchEnv(t)

	d.setGroupDefaultChannels(t, []string{"email"})
	d.setTeamNotifSettings(t, d.mgrTeamID,
		"",             // no notification_email
		"",             // no GChat
		[]string{},     // explicit empty Gruppkanal (overrides group default)
	)

	issue := d.newIssue(t)
	n := &notifications.CapturingNotifier{}
	notifications.SendIssueCreated(context.Background(), d.q, n, &notifications.NoopNotifier{}, issue, "http://localhost")

	if !containsEmail(n.Messages(), "manager@test.example") {
		t.Errorf("expected personal email to manager@test.example but got: %v", recipientEmails(n.Messages()))
	}
}

// TestPersonalEmailPolicy_GchatSpaceConfigured verifies that a team with a linked
// GChat space (and gchat in effective Gruppkanal) suppresses personal email in favour
// of the GChat broadcast.
func TestPersonalEmailPolicy_GchatSpaceConfigured(t *testing.T) {
	d := setupDispatchEnv(t)

	// Group has gchat in enabled_channels so the team can opt in.
	_, err := d.env.Pool.Exec(context.Background(),
		`UPDATE group_settings SET enabled_channels = '{email,gchat}' WHERE group_id = $1`, d.groupID)
	if err != nil {
		t.Fatalf("set enabled_channels: %v", err)
	}
	d.setTeamNotifSettings(t, d.mgrTeamID,
		"",                    // no broadcast email
		"spaces/TESTSPACE",    // GChat space linked
		[]string{"gchat"},     // explicit Gruppkanal: gchat only
	)

	issue := d.newIssue(t)
	n := &notifications.CapturingNotifier{}
	gn := &notifications.CapturingNotifier{}
	notifications.SendIssueCreated(context.Background(), d.q, n, gn, issue, "http://localhost")

	// Personal email should be suppressed because GChat space is configured and in Gruppkanal.
	if containsEmail(n.Messages(), "manager@test.example") {
		t.Errorf("expected personal email to manager@test.example to be suppressed (gchat broadcast configured)")
	}
}
