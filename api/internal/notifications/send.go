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
	name           string
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
	return recipient{id: r.ID, name: r.Name, email: r.Email, lang: lang, maxAccessLevel: r.MaxAccessLevel, notifPrefs: r.NotificationPrefs}
}

func fromGetTeamMembersRow(r db.GetTeamMembersWithEmailsRow) recipient {
	lang := "sv"
	if r.Language.Valid {
		lang = r.Language.String
	}
	return recipient{id: r.ID, name: r.Name, email: r.Email, lang: lang, maxAccessLevel: r.MaxAccessLevel, notifPrefs: r.NotificationPrefs}
}

func fromGetUserRow(r db.User) recipient {
	lang := "sv"
	if r.Language.Valid {
		lang = r.Language.String
	}
	return recipient{id: r.ID, name: r.Name, email: r.Email, lang: lang, maxAccessLevel: r.MaxAccessLevel, notifPrefs: r.NotificationPrefs}
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

// bookingMsg builds a Message for a booking event for a specific recipient.
func bookingMsg(ctx context.Context, q *db.Queries, b db.Booking, event, baseURL string, r recipient) Message {
	data := fetchBookingEmailData(ctx, q, b, event, r.lang, r.name, baseURL)
	htmlBody, textBody := renderBookingEmail(data)
	return Message{
		To:       r.email,
		Subject:  i18n.T(r.lang, "email_subject_"+event),
		Body:     htmlBody,
		TextBody: textBody,
	}
}

// issueMsg builds a Message for an issue event for a specific recipient.
func issueMsg(ctx context.Context, q *db.Queries, issue db.IssueReport, event, baseURL string, r recipient) Message {
	data := fetchIssueEmailData(ctx, q, issue, event, r.lang, r.name, baseURL)
	htmlBody, textBody := renderIssueEmail(data)
	return Message{
		To:       r.email,
		Subject:  i18n.T(r.lang, "email_subject_"+event),
		Body:     htmlBody,
		TextBody: textBody,
	}
}

// --- Booking events ---

func SendBookingNeedsApproval(ctx context.Context, q *db.Queries, n Notifier, b db.Booking, baseURL string) {
	for _, r := range groupManagers(ctx, q, b.GroupID) {
		r := r
		sendTo(ctx, q, n, b.GroupID, r, EventBookingNeedsApproval, "email", func(lang string) Message {
			return bookingMsg(ctx, q, b, EventBookingNeedsApproval, baseURL, r)
		})
	}
}

func SendBookingSubmittedNoApproval(ctx context.Context, q *db.Queries, n Notifier, b db.Booking, baseURL string) {
	for _, r := range groupManagers(ctx, q, b.GroupID) {
		r := r
		sendTo(ctx, q, n, b.GroupID, r, EventBookingSubmittedNoApproval, "email", func(lang string) Message {
			return bookingMsg(ctx, q, b, EventBookingSubmittedNoApproval, baseURL, r)
		})
	}
}

func SendBookingConfirmed(ctx context.Context, q *db.Queries, n Notifier, b db.Booking, baseURL string) {
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
		r := r
		sendTo(ctx, q, n, b.GroupID, r, EventBookingConfirmed, "email", func(lang string) Message {
			return bookingMsg(ctx, q, b, EventBookingConfirmed, baseURL, r)
		})
	}
}

func SendBookingRejected(ctx context.Context, q *db.Queries, n Notifier, b db.Booking, baseURL string) {
	r, ok := bookingCreator(ctx, q, b.GroupID, b.CreatedBy)
	if !ok {
		return
	}
	sendTo(ctx, q, n, b.GroupID, r, EventBookingRejected, "email", func(lang string) Message {
		return bookingMsg(ctx, q, b, EventBookingRejected, baseURL, r)
	})
}

func SendBookingCancelled(ctx context.Context, q *db.Queries, n Notifier, b db.Booking, baseURL string) {
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
		r := r
		sendTo(ctx, q, n, b.GroupID, r, EventBookingCancelled, "email", func(lang string) Message {
			return bookingMsg(ctx, q, b, EventBookingCancelled, baseURL, r)
		})
	}
}

// --- Issue events ---

func SendIssueCreated(ctx context.Context, q *db.Queries, n Notifier, issue db.IssueReport, baseURL string) {
	for _, r := range groupManagers(ctx, q, issue.GroupID) {
		r := r
		sendTo(ctx, q, n, issue.GroupID, r, EventIssueCreated, "email", func(lang string) Message {
			return issueMsg(ctx, q, issue, EventIssueCreated, baseURL, r)
		})
	}
}

func SendIssueAssignedToMe(ctx context.Context, q *db.Queries, n Notifier, issue db.IssueReport, assigneeID, baseURL string) {
	u, err := q.GetUser(ctx, db.GetUserParams{ID: assigneeID, GroupID: issue.GroupID})
	if err != nil {
		return
	}
	r := fromGetUserRow(u)
	sendTo(ctx, q, n, issue.GroupID, r, EventIssueAssignedToMe, "email", func(lang string) Message {
		return issueMsg(ctx, q, issue, EventIssueAssignedToMe, baseURL, r)
	})
}

func SendIssueResolved(ctx context.Context, q *db.Queries, n Notifier, issue db.IssueReport, baseURL string) {
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
		r := r
		sendTo(ctx, q, n, issue.GroupID, r, EventIssueResolved, "email", func(lang string) Message {
			return issueMsg(ctx, q, issue, EventIssueResolved, baseURL, r)
		})
	}
}

func SendIssueCommented(ctx context.Context, q *db.Queries, n Notifier, issue db.IssueReport, baseURL string) {
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
		r := r
		sendTo(ctx, q, n, issue.GroupID, r, EventIssueCommented, "email", func(lang string) Message {
			return issueMsg(ctx, q, issue, EventIssueCommented, baseURL, r)
		})
	}
}

// sendTestEmail sends a test email directly to the given address using SMTPNotifier.
// Used by the test-email endpoint; not subject to preference checks.
func sendTestEmail(ctx context.Context, q *db.Queries, n Notifier, groupID, to, recipientName, lang, baseURL string) error {
	group, _ := q.GetGroup(ctx, groupID)
	subject := i18n.T(lang, "email_subject_test_email")
	body := fmt.Sprintf("<p>%s</p>", i18n.T(lang, "notif_test_email"))
	text := i18n.T(lang, "notif_test_email")
	_ = group // may be used in future template
	return n.Send(ctx, Message{GroupID: groupID, To: to, Subject: subject, Body: body, TextBody: text})
}
