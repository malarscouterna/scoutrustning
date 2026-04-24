package notifications

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/malarscouterna/ms-utrustning/api/internal/db"
	"github.com/malarscouterna/ms-utrustning/api/internal/i18n"
)

// recipient is a minimal view of a user needed to send a notification.
type recipient struct {
	id             string
	email          string
	lang           string
	maxAccessLevel string
	notifPrefs     []byte
}

func fromGetGroupManagersRow(r db.GetGroupManagersRow) recipient {
	lang := "sv"
	if r.Language.Valid {
		lang = r.Language.String
	}
	return recipient{id: r.ID, email: r.Email, lang: lang, maxAccessLevel: r.MaxAccessLevel, notifPrefs: r.NotificationPrefs}
}

func fromGetTeamMembersRow(r db.GetTeamMembersWithEmailsRow) recipient {
	lang := "sv"
	if r.Language.Valid {
		lang = r.Language.String
	}
	return recipient{id: r.ID, email: r.Email, lang: lang, maxAccessLevel: r.MaxAccessLevel, notifPrefs: r.NotificationPrefs}
}

func fromGetUserRow(r db.User) recipient {
	lang := "sv"
	if r.Language.Valid {
		lang = r.Language.String
	}
	return recipient{id: r.ID, email: r.Email, lang: lang, maxAccessLevel: r.MaxAccessLevel, notifPrefs: r.NotificationPrefs}
}

// sendTo sends msg to r if their effective preference for event+channel is enabled.
func sendTo(ctx context.Context, q *db.Queries, n Notifier, groupID string, r recipient, event, channel string, msg func(lang string) Message) {
	if !IsEnabled(ctx, q, r.id, groupID, event, channel, r.maxAccessLevel == "manager") {
		return
	}
	m := msg(r.lang)
	m.GroupID = groupID
	if err := n.Send(ctx, m); err != nil {
		slog.Error("notification send failed", "event", event, "to", r.email, "error", err)
	}
}

// bookingCreator fetches the booking creator as a recipient.
func bookingCreator(ctx context.Context, q *db.Queries, groupID, createdBy string) (recipient, bool) {
	u, err := q.GetUser(ctx, db.GetUserParams{ID: createdBy, GroupID: groupID})
	if err != nil {
		return recipient{}, false
	}
	return fromGetUserRow(u), true
}

// teamMembers fetches all users whose team_ids include the given team.
func teamMembers(ctx context.Context, q *db.Queries, groupID string, teamID pgtype.UUID) []recipient {
	if !teamID.Valid {
		return nil
	}
	rows, err := q.GetTeamMembersWithEmails(ctx, db.GetTeamMembersWithEmailsParams{GroupID: groupID, TeamID: teamID})
	if err != nil {
		return nil
	}
	out := make([]recipient, len(rows))
	for i, r := range rows {
		out[i] = fromGetTeamMembersRow(r)
	}
	return out
}

// groupManagers fetches all manager users in the group.
func groupManagers(ctx context.Context, q *db.Queries, groupID string) []recipient {
	rows, err := q.GetGroupManagers(ctx, groupID)
	if err != nil {
		return nil
	}
	out := make([]recipient, len(rows))
	for i, r := range rows {
		out[i] = fromGetGroupManagersRow(r)
	}
	return out
}

// dedup removes duplicate user IDs from a recipient list.
func dedup(recipients []recipient) []recipient {
	seen := make(map[string]bool, len(recipients))
	out := recipients[:0]
	for _, r := range recipients {
		if !seen[r.id] {
			seen[r.id] = true
			out = append(out, r)
		}
	}
	return out
}

// simpleBody returns a minimal HTML body for stub emails.
func simpleBody(lang, bodyText string) string {
	return fmt.Sprintf(`<!DOCTYPE html><html><body><p>%s</p></body></html>`, bodyText)
}

// --- Booking events ---

func SendBookingNeedsApproval(ctx context.Context, q *db.Queries, n Notifier, b db.Booking) {
	for _, r := range groupManagers(ctx, q, b.GroupID) {
		sendTo(ctx, q, n, b.GroupID, r, EventBookingNeedsApproval, "email", func(lang string) Message {
			return Message{
				To:      r.email,
				Subject: i18n.T(lang, "email_subject_booking_needs_approval"),
				Body:    simpleBody(lang, i18n.T(lang, "notif_booking_needs_approval")),
			}
		})
	}
}

func SendBookingSubmittedNoApproval(ctx context.Context, q *db.Queries, n Notifier, b db.Booking) {
	for _, r := range groupManagers(ctx, q, b.GroupID) {
		sendTo(ctx, q, n, b.GroupID, r, EventBookingSubmittedNoApproval, "email", func(lang string) Message {
			return Message{
				To:      r.email,
				Subject: i18n.T(lang, "email_subject_booking_submitted_no_approval"),
				Body:    simpleBody(lang, i18n.T(lang, "notif_booking_submitted_no_approval")),
			}
		})
	}
}

func SendBookingConfirmed(ctx context.Context, q *db.Queries, n Notifier, b db.Booking) {
	recipients := dedup(append(
		func() []recipient {
			if r, ok := bookingCreator(ctx, q, b.GroupID, b.CreatedBy); ok {
				return []recipient{r}
			}
			return nil
		}(),
		teamMembers(ctx, q, b.GroupID, b.UsedByTeamID)...,
	))
	for _, r := range recipients {
		sendTo(ctx, q, n, b.GroupID, r, EventBookingConfirmed, "email", func(lang string) Message {
			return Message{
				To:      r.email,
				Subject: i18n.T(lang, "email_subject_booking_confirmed"),
				Body:    simpleBody(lang, i18n.T(lang, "notif_booking_confirmed")),
			}
		})
	}
}

func SendBookingRejected(ctx context.Context, q *db.Queries, n Notifier, b db.Booking) {
	r, ok := bookingCreator(ctx, q, b.GroupID, b.CreatedBy)
	if !ok {
		return
	}
	sendTo(ctx, q, n, b.GroupID, r, EventBookingRejected, "email", func(lang string) Message {
		return Message{
			To:      r.email,
			Subject: i18n.T(lang, "email_subject_booking_rejected"),
			Body:    simpleBody(lang, i18n.T(lang, "notif_booking_rejected")),
		}
	})
}

func SendBookingCancelled(ctx context.Context, q *db.Queries, n Notifier, b db.Booking) {
	recipients := dedup(append(
		func() []recipient {
			if r, ok := bookingCreator(ctx, q, b.GroupID, b.CreatedBy); ok {
				return []recipient{r}
			}
			return nil
		}(),
		teamMembers(ctx, q, b.GroupID, b.UsedByTeamID)...,
	))
	for _, r := range recipients {
		sendTo(ctx, q, n, b.GroupID, r, EventBookingCancelled, "email", func(lang string) Message {
			return Message{
				To:      r.email,
				Subject: i18n.T(lang, "email_subject_booking_cancelled"),
				Body:    simpleBody(lang, i18n.T(lang, "notif_booking_cancelled")),
			}
		})
	}
}

// --- Issue events ---

func SendIssueCreated(ctx context.Context, q *db.Queries, n Notifier, issue db.IssueReport) {
	for _, r := range groupManagers(ctx, q, issue.GroupID) {
		sendTo(ctx, q, n, issue.GroupID, r, EventIssueCreated, "email", func(lang string) Message {
			return Message{
				To:      r.email,
				Subject: i18n.T(lang, "email_subject_issue_created", map[string]string{"title": issue.Title}),
				Body:    simpleBody(lang, i18n.T(lang, "notif_issue_created")),
			}
		})
	}
}

func SendIssueAssignedToMe(ctx context.Context, q *db.Queries, n Notifier, issue db.IssueReport, assigneeID string) {
	u, err := q.GetUser(ctx, db.GetUserParams{ID: assigneeID, GroupID: issue.GroupID})
	if err != nil {
		return
	}
	r := fromGetUserRow(u)
	sendTo(ctx, q, n, issue.GroupID, r, EventIssueAssignedToMe, "email", func(lang string) Message {
		return Message{
			To:      r.email,
			Subject: i18n.T(lang, "email_subject_issue_assigned_to_me", map[string]string{"title": issue.Title}),
			Body:    simpleBody(lang, i18n.T(lang, "notif_issue_assigned_to_me")),
		}
	})
}

func SendIssueResolved(ctx context.Context, q *db.Queries, n Notifier, issue db.IssueReport) {
	// Notify reporter + all assignees.
	reporter, err := q.GetUser(ctx, db.GetUserParams{ID: issue.ReporterID, GroupID: issue.GroupID})
	if err != nil {
		return
	}
	assignees, _ := q.ListIssueAssignees(ctx, db.ListIssueAssigneesParams{IssueID: issue.ID, GroupID: issue.GroupID})

	recipients := []recipient{fromGetUserRow(reporter)}
	for _, a := range assignees {
		if u, err := q.GetUser(ctx, db.GetUserParams{ID: a.UserID, GroupID: issue.GroupID}); err == nil {
			recipients = append(recipients, fromGetUserRow(u))
		}
	}
	for _, r := range dedup(recipients) {
		sendTo(ctx, q, n, issue.GroupID, r, EventIssueResolved, "email", func(lang string) Message {
			return Message{
				To:      r.email,
				Subject: i18n.T(lang, "email_subject_issue_resolved", map[string]string{"title": issue.Title}),
				Body:    simpleBody(lang, i18n.T(lang, "notif_issue_resolved")),
			}
		})
	}
}

func SendIssueCommented(ctx context.Context, q *db.Queries, n Notifier, issue db.IssueReport) {
	reporter, err := q.GetUser(ctx, db.GetUserParams{ID: issue.ReporterID, GroupID: issue.GroupID})
	if err != nil {
		return
	}
	assignees, _ := q.ListIssueAssignees(ctx, db.ListIssueAssigneesParams{IssueID: issue.ID, GroupID: issue.GroupID})

	recipients := []recipient{fromGetUserRow(reporter)}
	for _, a := range assignees {
		if u, err := q.GetUser(ctx, db.GetUserParams{ID: a.UserID, GroupID: issue.GroupID}); err == nil {
			recipients = append(recipients, fromGetUserRow(u))
		}
	}
	for _, r := range dedup(recipients) {
		sendTo(ctx, q, n, issue.GroupID, r, EventIssueCommented, "email", func(lang string) Message {
			return Message{
				To:      r.email,
				Subject: i18n.T(lang, "email_subject_issue_commented", map[string]string{"title": issue.Title}),
				Body:    simpleBody(lang, i18n.T(lang, "notif_issue_commented")),
			}
		})
	}
}
