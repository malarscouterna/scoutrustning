package notifications

import (
	"context"
	"fmt"
	"html"
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
// teamID is the team context for three-tier pref resolution; pass "" for issue events.
// entityID and threadKey drive email threading; pass zero UUID / "" to skip threading.
func sendTo(ctx context.Context, q *db.Queries, n Notifier, groupID, teamID string, r recipient, event, channel string, entityID pgtype.UUID, threadKey string, msg func(lang string) Message) {
	if !IsEnabled(ctx, q, r.id, groupID, teamID, event, channel, r.maxAccessLevel == "manager") {
		return
	}
	m := msg(r.lang)
	m.GroupID = groupID

	// Email threading: look up prior Message-ID for this thread, or generate a new one.
	if channel == "email" && threadKey != "" && entityID.Valid {
		idSuffix := r.id
		if len(idSuffix) > 8 {
			idSuffix = idSuffix[:8]
		}
		newMsgID := fmt.Sprintf("%s-%s@notification", threadKey, idSuffix)
		prior, err := q.GetThreadMessageID(ctx, db.GetThreadMessageIDParams{
			ThreadKey: pgtype.Text{String: threadKey, Valid: true},
			UserID:    r.id,
			Channel:   channel,
		})
		if err == nil && prior.Valid {
			m.InReplyTo = prior.String
		} else {
			m.MessageID = newMsgID
		}

		status := "sent"
		errText := pgtype.Text{}
		if sendErr := n.Send(ctx, m); sendErr != nil {
			slog.Error("notification send failed", "event", event, "to", r.email, "error", sendErr)
			status = "failed"
			errText = pgtype.Text{String: sendErr.Error(), Valid: true}
		}
		_ = q.LogNotification(ctx, db.LogNotificationParams{
			GroupID:   groupID,
			UserID:    r.id,
			EventType: event,
			EntityID:  entityID,
			Channel:   channel,
			Status:    status,
			Error:     errText,
			ThreadKey: pgtype.Text{String: threadKey, Valid: true},
			MessageID: pgtype.Text{String: newMsgID, Valid: m.MessageID != ""},
		})
		return
	}

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

// sendBroadcastGChat sends one card to a team's mapped Google Chat Space if configured.
// Uses sentinel user_id "gchat:<teamID>" in notification_log.
func sendBroadcastGChat(ctx context.Context, q *db.Queries, gn Notifier, groupID string, teamID pgtype.UUID, event string, entityID pgtype.UUID, threadKey, subject, textBody string) {
	if !teamID.Valid {
		return
	}
	team, err := q.GetTeam(ctx, db.GetTeamParams{ID: teamID, GroupID: groupID})
	if err != nil || !team.GchatSpaceID.Valid || team.GchatSpaceID.String == "" {
		return
	}
	teamPrefs := parseSimplePrefs(team.NotificationPrefs)
	if enabled, ok := lookupSimplePrefs(teamPrefs, event, "gchat"); ok {
		if !enabled {
			return
		}
	} else {
		groupDefaultsRaw, _ := q.GetGroupNotificationDefaults(ctx, groupID)
		groupDefaults := ParseGroupDefaults(groupDefaultsRaw)
		if gv, ok := groupDefaults.Lookup(event, "gchat", true); ok {
			if !gv {
				return
			}
		} else if bsd := BroadcastSystemDefaults(); bsd[EventKey(event)] != nil {
			if !bsd[EventKey(event)]["gchat"] {
				return
			}
		}
	}

	msg := Message{
		GroupID:   groupID,
		To:        team.GchatSpaceID.String,
		Subject:   subject,
		TextBody:  textBody,
		ThreadKey: threadKey,
	}

	status := "sent"
	errText := pgtype.Text{}
	if sendErr := gn.Send(ctx, msg); sendErr != nil {
		slog.Error("gchat broadcast failed", "event", event, "space", team.GchatSpaceID.String, "error", sendErr)
		status = "failed"
		errText = pgtype.Text{String: sendErr.Error(), Valid: true}
	}
	_ = q.LogNotification(ctx, db.LogNotificationParams{
		GroupID:   groupID,
		UserID:    "gchat:" + teamIDStr(teamID),
		EventType: event,
		EntityID:  entityID,
		Channel:   "gchat",
		Status:    status,
		Error:     errText,
		ThreadKey: pgtype.Text{String: threadKey, Valid: threadKey != ""},
		MessageID: pgtype.Text{},
	})
}

// sendBroadcastEmail sends one email to a team's shared notification_email address if configured
// and enabled in the team's notification prefs. Uses sentinel user_id "broadcast:<teamID>" in
// notification_log so threading is independent from personal sends.
func sendBroadcastEmail(ctx context.Context, q *db.Queries, n Notifier, groupID string, teamID pgtype.UUID, event string, entityID pgtype.UUID, threadKey string, msg Message) {
	if !teamID.Valid {
		return
	}
	ts, err := q.GetTeamNotificationSettings(ctx, db.GetTeamNotificationSettingsParams{
		ID: teamID, GroupID: groupID,
	})
	if err != nil || !ts.NotificationEmail.Valid || ts.NotificationEmail.String == "" {
		return
	}
	teamPrefs := parseSimplePrefs(ts.NotificationPrefs)
	if enabled, ok := lookupSimplePrefs(teamPrefs, event, "email"); ok {
		if !enabled {
			return
		}
	} else {
		// No team pref — check group defaults, then broadcast system default.
		groupDefaultsRaw, _ := q.GetGroupNotificationDefaults(ctx, groupID)
		groupDefaults := ParseGroupDefaults(groupDefaultsRaw)
		if gv, ok := groupDefaults.Lookup(event, "email", true); ok {
			if !gv {
				return
			}
		} else if bsd := BroadcastSystemDefaults(); bsd[EventKey(event)] != nil {
			if !bsd[EventKey(event)]["email"] {
				return
			}
		}
	}

	sentinelUserID := "broadcast:" + teamIDStr(teamID)
	prior, _ := q.GetBroadcastThreadMessageID(ctx, db.GetBroadcastThreadMessageIDParams{
		ThreadKey: pgtype.Text{String: threadKey, Valid: true},
		Channel:   "email",
	})

	msg.To = ts.NotificationEmail.String
	msg.GroupID = groupID
	logMsgID := pgtype.Text{}
	if prior.Valid {
		msg.InReplyTo = prior.String
	} else {
		newMsgID := threadKey + "-broadcast@notification"
		msg.MessageID = newMsgID
		logMsgID = pgtype.Text{String: newMsgID, Valid: true}
	}

	status := "sent"
	errText := pgtype.Text{}
	if sendErr := n.Send(ctx, msg); sendErr != nil {
		slog.Error("broadcast notification failed", "event", event, "to", msg.To, "error", sendErr)
		status = "failed"
		errText = pgtype.Text{String: sendErr.Error(), Valid: true}
	}
	_ = q.LogNotification(ctx, db.LogNotificationParams{
		GroupID:   groupID,
		UserID:    sentinelUserID,
		EventType: event,
		EntityID:  entityID,
		Channel:   "email",
		Status:    status,
		Error:     errText,
		ThreadKey: pgtype.Text{String: threadKey, Valid: true},
		MessageID: logMsgID,
	})
}

// bookingMsg builds a Message for a booking event for a specific recipient.
func bookingMsg(ctx context.Context, q *db.Queries, b db.Booking, event, baseURL string, r recipient) Message {
	data := fetchBookingEmailData(ctx, q, b, event, r.lang, r.name, baseURL)
	htmlBody, textBody := renderBookingEmail(data)
	subject := i18n.T(r.lang, "email_subject_"+event)
	if data.TeamName != "" {
		subject = data.TeamName + ": " + subject
	}
	return Message{
		To:       r.email,
		Subject:  subject,
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
		Subject:  i18n.T(r.lang, "email_subject_"+event, map[string]string{"title": issue.Title}),
		Body:     htmlBody,
		TextBody: textBody,
	}
}

// bookingThreadKey returns the thread key for a booking entity.
func bookingThreadKey(b db.Booking) string {
	return "booking_" + teamIDStr(b.ID)
}

// issueThreadKey returns the thread key for an issue entity.
func issueThreadKey(issue db.IssueReport) string {
	return "issue_" + teamIDStr(issue.ID)
}

// --- Booking events ---

func SendBookingNeedsApproval(ctx context.Context, q *db.Queries, n Notifier, gn Notifier, b db.Booking, baseURL string) {
	tid := teamIDStr(b.UsedByTeamID)
	tk := bookingThreadKey(b)
	broadcastMsg := bookingMsg(ctx, q, b, EventBookingNeedsApproval, baseURL, recipient{lang: "sv"})
	sendBroadcastEmail(ctx, q, n, b.GroupID, b.UsedByTeamID, EventBookingNeedsApproval, b.ID, tk, broadcastMsg)
	sendBroadcastGChat(ctx, q, gn, b.GroupID, b.UsedByTeamID, EventBookingNeedsApproval, b.ID, tk, broadcastMsg.Subject, broadcastMsg.TextBody)
	for _, r := range groupManagers(ctx, q, b.GroupID) {
		r := r
		sendTo(ctx, q, n, b.GroupID, tid, r, EventBookingNeedsApproval, "email", b.ID, tk, func(lang string) Message {
			return bookingMsg(ctx, q, b, EventBookingNeedsApproval, baseURL, r)
		})
	}
}

func SendBookingSubmittedNoApproval(ctx context.Context, q *db.Queries, n Notifier, gn Notifier, b db.Booking, baseURL string) {
	tid := teamIDStr(b.UsedByTeamID)
	tk := bookingThreadKey(b)
	broadcastMsg := bookingMsg(ctx, q, b, EventBookingSubmittedNoApproval, baseURL, recipient{lang: "sv"})
	sendBroadcastEmail(ctx, q, n, b.GroupID, b.UsedByTeamID, EventBookingSubmittedNoApproval, b.ID, tk, broadcastMsg)
	sendBroadcastGChat(ctx, q, gn, b.GroupID, b.UsedByTeamID, EventBookingSubmittedNoApproval, b.ID, tk, broadcastMsg.Subject, broadcastMsg.TextBody)
	for _, r := range groupManagers(ctx, q, b.GroupID) {
		r := r
		sendTo(ctx, q, n, b.GroupID, tid, r, EventBookingSubmittedNoApproval, "email", b.ID, tk, func(lang string) Message {
			return bookingMsg(ctx, q, b, EventBookingSubmittedNoApproval, baseURL, r)
		})
	}
}

func SendBookingConfirmed(ctx context.Context, q *db.Queries, n Notifier, gn Notifier, b db.Booking, baseURL string) {
	tid := teamIDStr(b.UsedByTeamID)
	tk := bookingThreadKey(b)
	broadcastMsg := bookingMsg(ctx, q, b, EventBookingConfirmed, baseURL, recipient{lang: "sv"})
	sendBroadcastEmail(ctx, q, n, b.GroupID, b.UsedByTeamID, EventBookingConfirmed, b.ID, tk, broadcastMsg)
	sendBroadcastGChat(ctx, q, gn, b.GroupID, b.UsedByTeamID, EventBookingConfirmed, b.ID, tk, broadcastMsg.Subject, broadcastMsg.TextBody)
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
		sendTo(ctx, q, n, b.GroupID, tid, r, EventBookingConfirmed, "email", b.ID, tk, func(lang string) Message {
			return bookingMsg(ctx, q, b, EventBookingConfirmed, baseURL, r)
		})
	}
}

func SendBookingRejected(ctx context.Context, q *db.Queries, n Notifier, gn Notifier, b db.Booking, baseURL string) {
	r, ok := bookingCreator(ctx, q, b.GroupID, b.CreatedBy)
	if !ok {
		return
	}
	tid := teamIDStr(b.UsedByTeamID)
	tk := bookingThreadKey(b)
	broadcastMsg := bookingMsg(ctx, q, b, EventBookingRejected, baseURL, recipient{lang: "sv"})
	sendBroadcastEmail(ctx, q, n, b.GroupID, b.UsedByTeamID, EventBookingRejected, b.ID, tk, broadcastMsg)
	sendBroadcastGChat(ctx, q, gn, b.GroupID, b.UsedByTeamID, EventBookingRejected, b.ID, tk, broadcastMsg.Subject, broadcastMsg.TextBody)
	sendTo(ctx, q, n, b.GroupID, tid, r, EventBookingRejected, "email", b.ID, tk, func(lang string) Message {
		return bookingMsg(ctx, q, b, EventBookingRejected, baseURL, r)
	})
}

func SendBookingCancelled(ctx context.Context, q *db.Queries, n Notifier, gn Notifier, b db.Booking, baseURL string) {
	tid := teamIDStr(b.UsedByTeamID)
	tk := bookingThreadKey(b)
	broadcastMsg := bookingMsg(ctx, q, b, EventBookingCancelled, baseURL, recipient{lang: "sv"})
	sendBroadcastEmail(ctx, q, n, b.GroupID, b.UsedByTeamID, EventBookingCancelled, b.ID, tk, broadcastMsg)
	sendBroadcastGChat(ctx, q, gn, b.GroupID, b.UsedByTeamID, EventBookingCancelled, b.ID, tk, broadcastMsg.Subject, broadcastMsg.TextBody)
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
		sendTo(ctx, q, n, b.GroupID, tid, r, EventBookingCancelled, "email", b.ID, tk, func(lang string) Message {
			return bookingMsg(ctx, q, b, EventBookingCancelled, baseURL, r)
		})
	}
}

// --- Issue events ---

func SendIssueCreated(ctx context.Context, q *db.Queries, n Notifier, issue db.IssueReport, baseURL string) {
	tk := issueThreadKey(issue)
	for _, r := range groupManagers(ctx, q, issue.GroupID) {
		r := r
		sendTo(ctx, q, n, issue.GroupID, "", r, EventIssueCreated, "email", issue.ID, tk, func(lang string) Message {
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
	tk := issueThreadKey(issue)
	sendTo(ctx, q, n, issue.GroupID, "", r, EventIssueAssignedToMe, "email", issue.ID, tk, func(lang string) Message {
		return issueMsg(ctx, q, issue, EventIssueAssignedToMe, baseURL, r)
	})
}

func SendIssueResolved(ctx context.Context, q *db.Queries, n Notifier, issue db.IssueReport, baseURL string) {
	reporter, err := q.GetUser(ctx, db.GetUserParams{ID: issue.ReporterID, GroupID: issue.GroupID})
	if err != nil {
		return
	}
	assignees, _ := q.ListIssueAssignees(ctx, db.ListIssueAssigneesParams{IssueID: issue.ID, GroupID: issue.GroupID})
	tk := issueThreadKey(issue)

	recipients := []recipient{fromGetUserRow(reporter)}
	for _, a := range assignees {
		if u, err := q.GetUser(ctx, db.GetUserParams{ID: a.UserID, GroupID: issue.GroupID}); err == nil {
			recipients = append(recipients, fromGetUserRow(u))
		}
	}
	for _, r := range dedup(recipients) {
		r := r
		sendTo(ctx, q, n, issue.GroupID, "", r, EventIssueResolved, "email", issue.ID, tk, func(lang string) Message {
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
	tk := issueThreadKey(issue)

	recipients := []recipient{fromGetUserRow(reporter)}
	for _, a := range assignees {
		if u, err := q.GetUser(ctx, db.GetUserParams{ID: a.UserID, GroupID: issue.GroupID}); err == nil {
			recipients = append(recipients, fromGetUserRow(u))
		}
	}
	for _, r := range dedup(recipients) {
		r := r
		sendTo(ctx, q, n, issue.GroupID, "", r, EventIssueCommented, "email", issue.ID, tk, func(lang string) Message {
			return issueMsg(ctx, q, issue, EventIssueCommented, baseURL, r)
		})
	}
}

// sendTestEmail sends a test email directly to the given address using SMTPNotifier.
// Used by the test-email endpoint; not subject to preference checks.
func sendTestEmail(ctx context.Context, q *db.Queries, n Notifier, groupID, to, recipientName, lang, baseURL string) error {
	group, _ := q.GetGroup(ctx, groupID)
	subject := i18n.T(lang, "email_subject_test_email")
	bodyText := i18n.T(lang, "notif_test_email")
	logoURL := groupLogoURL(ctx, q, groupID, baseURL)
	logoHdr := logoHeader(logoURL, group.Name)
	unsubURL := html.EscapeString(baseURL + "/profile")
	body := fmt.Sprintf(`<div style="font-family:sans-serif;max-width:600px;margin:0 auto">
<div style="background:#1e3a5f;padding:20px 24px;border-radius:6px 6px 0 0">%s</div>
<div style="background:#ffffff;padding:24px;border:1px solid #e5e7eb;border-top:none;border-radius:0 0 6px 6px">
<p style="margin:0 0 16px">%s</p>
<p style="margin:0;font-size:12px;color:#6b7280">%s &mdash; <a href="%s">%s</a></p>
</div></div>`,
		logoHdr,
		html.EscapeString(bodyText),
		html.EscapeString(group.Name),
		unsubURL,
		html.EscapeString(i18n.T(lang, "email_footer_unsubscribe")),
	)
	return n.Send(ctx, Message{GroupID: groupID, To: to, Subject: subject, Body: body, TextBody: bodyText})
}
