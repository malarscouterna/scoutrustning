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
	notifEmail     string // override for personal delivery; empty = use email
	lang           string
	maxAccessLevel string
	notifPrefs     []byte
}

// deliveryEmail returns the address to use for personal notifications.
func (r recipient) deliveryEmail() string {
	if r.notifEmail != "" {
		return r.notifEmail
	}
	return r.email
}

func langOrDefault(v pgtype.Text) string {
	if v.Valid {
		return v.String
	}
	return "sv"
}

func fromGetGroupManagersRow(r db.GetGroupManagersRow) recipient {
	return recipient{id: r.ID, name: r.Name, email: r.Email, notifEmail: r.NotificationEmail.String, lang: langOrDefault(r.Language), maxAccessLevel: r.MaxAccessLevel, notifPrefs: r.NotificationPrefs}
}

func fromGetTeamMembersRow(r db.GetTeamMembersWithEmailsRow) recipient {
	return recipient{id: r.ID, name: r.Name, email: r.Email, notifEmail: r.NotificationEmail.String, lang: langOrDefault(r.Language), maxAccessLevel: r.MaxAccessLevel, notifPrefs: r.NotificationPrefs}
}

func fromGetUserRow(r db.User) recipient {
	return recipient{id: r.ID, name: r.Name, email: r.Email, notifEmail: r.NotificationEmail.String, lang: langOrDefault(r.Language), maxAccessLevel: r.MaxAccessLevel, notifPrefs: r.NotificationPrefs}
}

// sendTo sends msg to r based on their personal email policy for the event.
// For personal events (teamID="") PolicyAlways/PolicyNever applies directly.
// entityID and threadKey drive email threading; pass zero UUID / "" to skip threading.
func sendTo(ctx context.Context, q *db.Queries, n Notifier, ds dispatchSettings, groupID string, r recipient, event, channel string, entityID pgtype.UUID, threadKey string, msg func(lang string) Message) {
	policy := resolvePersonalEmailPolicy(ctx, q, ds, r.id, groupID, event)
	switch policy {
	case PolicyNever:
		return
	case PolicyIfNoBroadcast:
		if HasConfiguredBroadcast(ds.effective, ds.team) {
			return
		}
	}
	// PolicyAlways or non-empty Gruppkanal suppression not triggered — send.
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
			slog.Error("notification send failed", "event", event, "to", r.deliveryEmail(), "error", sendErr)
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
		slog.Error("notification send failed", "event", event, "to", r.deliveryEmail(), "error", err)
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

// bookingRecipients returns the creator and all team members for a booking, deduped.
func bookingRecipients(ctx context.Context, q *db.Queries, groupID, createdBy string, teamID pgtype.UUID) []recipient {
	var out []recipient
	if creator, ok := bookingCreator(ctx, q, groupID, createdBy); ok {
		out = append(out, creator)
	}
	out = append(out, teamMembers(ctx, q, groupID, teamID)...)
	return dedup(out)
}

// containsChannel reports whether ch appears in the channels slice.
func containsChannel(channels []string, ch string) bool {
	for _, c := range channels {
		if c == ch {
			return true
		}
	}
	return false
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

// managerTeamID returns the UUID of the manager-level team for a group, or an invalid UUID if none.
func managerTeamID(ctx context.Context, q *db.Queries, groupID string) pgtype.UUID {
	id, err := q.GetManagerTeam(ctx, groupID)
	if err != nil {
		return pgtype.UUID{}
	}
	return id
}

// sendBroadcastGChat sends two threaded messages to a team's mapped Google Chat Space.
// Fires only if "gchat" is in the effective Gruppkanal and the event is enabled.
// Uses sentinel user_id "gchat:<teamID>" in notification_log.
func sendBroadcastGChat(ctx context.Context, q *db.Queries, gn Notifier, groupID string, teamID pgtype.UUID, ds dispatchSettings, event string, entityID pgtype.UUID, entityThreadKey, openerText, detailText string) {
	if !teamID.Valid || ds.team.GchatSpaceID == "" {
		return
	}
	if !containsChannel(ds.effective, "gchat") || !isGruppkanalEnabled(ds, event) {
		return
	}

	gchatThreadKey := entityThreadKey

	labeledOpener := openerText
	if gcn, ok := gn.(*GChatNotifier); ok && gcn.LabelTeam {
		// Dev mode: fetch team name to distinguish messages from different teams in the same space.
		if team, err := q.GetTeam(ctx, db.GetTeamParams{ID: teamID, GroupID: groupID}); err == nil {
			labeledOpener = "*" + team.Name + "*  " + openerText
		}
	}

	space := ds.team.GchatSpaceID
	sentinelUserID := "gchat:" + formatUUID(teamID)

	logGChat := func(status, errStr string) {
		_ = q.LogNotification(ctx, db.LogNotificationParams{
			GroupID:   groupID,
			UserID:    sentinelUserID,
			EventType: event,
			EntityID:  entityID,
			Channel:   "gchat",
			Status:    status,
			Error:     pgtype.Text{String: errStr, Valid: errStr != ""},
			ThreadKey: pgtype.Text{String: gchatThreadKey, Valid: gchatThreadKey != ""},
			MessageID: pgtype.Text{},
		})
	}

	if pn, ok := gn.(PairedNotifier); ok {
		openerErr, detailErr := pn.SendPaired(ctx, groupID, space, labeledOpener, detailText, gchatThreadKey)
		if openerErr != nil {
			slog.Error("gchat broadcast opener failed", "event", event, "space", space, "error", openerErr)
			logGChat("failed", openerErr.Error())
			return
		}
		logGChat("sent", "")
		if detailText != "" {
			errStr := ""
			if detailErr != nil {
				slog.Error("gchat broadcast detail failed", "event", event, "space", space, "error", detailErr)
				errStr = detailErr.Error()
			}
			logGChat("sent", errStr)
		}
		return
	}

	// Fallback for notifiers that don't support paired sends (e.g. CapturingNotifier in tests).
	for _, text := range []string{labeledOpener, detailText} {
		if text == "" {
			continue
		}
		msg := Message{GroupID: groupID, To: space, Subject: text, ThreadKey: gchatThreadKey}
		status := "sent"
		errStr := ""
		if sendErr := gn.Send(ctx, msg); sendErr != nil {
			slog.Error("gchat broadcast failed", "event", event, "space", space, "error", sendErr)
			status = "failed"
			errStr = sendErr.Error()
		}
		logGChat(status, errStr)
	}
}

// sendBroadcastEmail sends one email to a team's shared notification_email address if configured.
// Fires only if "email" is in the effective Gruppkanal and the event is enabled.
// Uses sentinel user_id "broadcast:<teamID>" in notification_log so threading is independent
// from personal sends.
func sendBroadcastEmail(ctx context.Context, q *db.Queries, n Notifier, groupID string, teamID pgtype.UUID, ds dispatchSettings, event string, entityID pgtype.UUID, threadKey string, msg Message) {
	if !teamID.Valid || ds.team.NotificationEmail == "" {
		return
	}
	if !containsChannel(ds.effective, "email") || !isGruppkanalEnabled(ds, event) {
		return
	}

	sentinelUserID := "broadcast:" + formatUUID(teamID)
	prior, _ := q.GetBroadcastThreadMessageID(ctx, db.GetBroadcastThreadMessageIDParams{
		ThreadKey: pgtype.Text{String: threadKey, Valid: true},
		Channel:   "email",
	})

	msg.To = ds.team.NotificationEmail
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
		To:       r.deliveryEmail(),
		Subject:  subject,
		Body:     htmlBody,
		TextBody: textBody,
	}
}

// bookingBroadcastTexts returns the GChat opener and detail text for a booking broadcast.
func bookingBroadcastTexts(ctx context.Context, q *db.Queries, b db.Booking, event, baseURL string) (opener, detail string) {
	data := fetchBookingEmailData(ctx, q, b, event, "sv", "", baseURL)
	return BookingOpenerText(data), BookingDetailText(data)
}

// issueMsg builds a Message for an issue event for a specific recipient.
func issueMsg(ctx context.Context, q *db.Queries, issue db.IssueReport, event, baseURL string, r recipient) Message {
	data := fetchIssueEmailData(ctx, q, issue, event, r.lang, r.name, baseURL)
	htmlBody, textBody := renderIssueEmail(data)
	return Message{
		To:       r.deliveryEmail(),
		Subject:  i18n.T(r.lang, "email_subject_"+event, map[string]string{"title": issue.Title}),
		Body:     htmlBody,
		TextBody: textBody,
	}
}

// issueBroadcastData returns the broadcast email Message and GChat texts for an issue event,
// fetching template data exactly once.
func issueBroadcastData(ctx context.Context, q *db.Queries, issue db.IssueReport, event, baseURL string) (msg Message, opener, detail string) {
	data := fetchIssueEmailData(ctx, q, issue, event, "sv", "", baseURL)
	htmlBody, textBody := renderIssueEmail(data)
	msg = Message{
		Subject:  i18n.T("sv", "email_subject_"+event, map[string]string{"title": issue.Title}),
		Body:     htmlBody,
		TextBody: textBody,
	}
	opener = IssueOpenerText(data)
	detail = IssueDetailText(data)
	return
}

// bookingThreadKey returns the thread key for a booking entity.
func bookingThreadKey(b db.Booking) string {
	return "booking_" + formatUUID(b.ID)
}

// issueThreadKey returns the thread key for an issue entity.
func issueThreadKey(issue db.IssueReport) string {
	return "issue_" + formatUUID(issue.ID)
}

// --- Booking events ---

func SendBookingNeedsApproval(ctx context.Context, q *db.Queries, n Notifier, gn Notifier, b db.Booking, baseURL string) {
	sendBookingToManagers(ctx, q, n, gn, b, EventBookingNeedsApproval, baseURL)
}

func SendBookingSubmittedNoApproval(ctx context.Context, q *db.Queries, n Notifier, gn Notifier, b db.Booking, baseURL string) {
	sendBookingToManagers(ctx, q, n, gn, b, EventBookingSubmittedNoApproval, baseURL)
}

// sendBookingToManagers broadcasts to the manager team and sends personal email to each manager.
func sendBookingToManagers(ctx context.Context, q *db.Queries, n Notifier, gn Notifier, b db.Booking, event, baseURL string) {
	mgrTeam := managerTeamID(ctx, q, b.GroupID)
	ds := loadDispatchSettings(ctx, q, b.GroupID, formatUUID(mgrTeam))
	dsBooking := loadDispatchSettings(ctx, q, b.GroupID, formatUUID(b.UsedByTeamID))
	tk := bookingThreadKey(b)
	broadcastMsg := bookingMsg(ctx, q, b, event, baseURL, recipient{lang: "sv"})
	opener, detail := bookingBroadcastTexts(ctx, q, b, event, baseURL)
	sendBroadcastEmail(ctx, q, n, b.GroupID, mgrTeam, ds, event, b.ID, tk, broadcastMsg)
	sendBroadcastGChat(ctx, q, gn, b.GroupID, mgrTeam, ds, event, b.ID, tk, opener, detail)
	for _, r := range groupManagers(ctx, q, b.GroupID) {
		r := r
		sendTo(ctx, q, n, dsBooking, b.GroupID, r, event, "email", b.ID, tk, func(lang string) Message {
			return bookingMsg(ctx, q, b, event, baseURL, r)
		})
	}
}

// sendBookingToTeam broadcasts to the booking team and sends personal email to the creator and team members.
func sendBookingToTeam(ctx context.Context, q *db.Queries, n Notifier, gn Notifier, b db.Booking, event, baseURL string) {
	ds := loadDispatchSettings(ctx, q, b.GroupID, formatUUID(b.UsedByTeamID))
	tk := bookingThreadKey(b)
	broadcastMsg := bookingMsg(ctx, q, b, event, baseURL, recipient{lang: "sv"})
	opener, detail := bookingBroadcastTexts(ctx, q, b, event, baseURL)
	sendBroadcastEmail(ctx, q, n, b.GroupID, b.UsedByTeamID, ds, event, b.ID, tk, broadcastMsg)
	sendBroadcastGChat(ctx, q, gn, b.GroupID, b.UsedByTeamID, ds, event, b.ID, tk, opener, detail)
	for _, r := range bookingRecipients(ctx, q, b.GroupID, b.CreatedBy, b.UsedByTeamID) {
		r := r
		sendTo(ctx, q, n, ds, b.GroupID, r, event, "email", b.ID, tk, func(lang string) Message {
			return bookingMsg(ctx, q, b, event, baseURL, r)
		})
	}
}

func SendBookingConfirmed(ctx context.Context, q *db.Queries, n Notifier, gn Notifier, b db.Booking, baseURL string) {
	sendBookingToTeam(ctx, q, n, gn, b, EventBookingConfirmed, baseURL)
}

func SendBookingRejected(ctx context.Context, q *db.Queries, n Notifier, gn Notifier, b db.Booking, baseURL string) {
	r, ok := bookingCreator(ctx, q, b.GroupID, b.CreatedBy)
	if !ok {
		return
	}
	ds := loadDispatchSettings(ctx, q, b.GroupID, formatUUID(b.UsedByTeamID))
	tk := bookingThreadKey(b)
	broadcastMsg := bookingMsg(ctx, q, b, EventBookingRejected, baseURL, recipient{lang: "sv"})
	opener, detail := bookingBroadcastTexts(ctx, q, b, EventBookingRejected, baseURL)
	sendBroadcastEmail(ctx, q, n, b.GroupID, b.UsedByTeamID, ds, EventBookingRejected, b.ID, tk, broadcastMsg)
	sendBroadcastGChat(ctx, q, gn, b.GroupID, b.UsedByTeamID, ds, EventBookingRejected, b.ID, tk, opener, detail)
	sendTo(ctx, q, n, ds, b.GroupID, r, EventBookingRejected, "email", b.ID, tk, func(lang string) Message {
		return bookingMsg(ctx, q, b, EventBookingRejected, baseURL, r)
	})
}

func SendBookingCancelled(ctx context.Context, q *db.Queries, n Notifier, gn Notifier, b db.Booking, baseURL string) {
	sendBookingToTeam(ctx, q, n, gn, b, EventBookingCancelled, baseURL)
}

// --- Issue events ---

func SendIssueCreated(ctx context.Context, q *db.Queries, n Notifier, gn Notifier, issue db.IssueReport, baseURL string) {
	mgrTeam := managerTeamID(ctx, q, issue.GroupID)
	ds := loadDispatchSettings(ctx, q, issue.GroupID, formatUUID(mgrTeam))
	tk := issueThreadKey(issue)
	broadcastMsg, opener, detail := issueBroadcastData(ctx, q, issue, EventIssueCreated, baseURL)
	sendBroadcastEmail(ctx, q, n, issue.GroupID, mgrTeam, ds, EventIssueCreated, issue.ID, tk, broadcastMsg)
	sendBroadcastGChat(ctx, q, gn, issue.GroupID, mgrTeam, ds, EventIssueCreated, issue.ID, tk, opener, detail)
	for _, r := range groupManagers(ctx, q, issue.GroupID) {
		r := r
		sendTo(ctx, q, n, ds, issue.GroupID, r, EventIssueCreated, "email", issue.ID, tk, func(lang string) Message {
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
	ds := loadDispatchSettings(ctx, q, issue.GroupID, "")
	sendTo(ctx, q, n, ds, issue.GroupID, r, EventIssueAssignedToMe, "email", issue.ID, tk, func(lang string) Message {
		return issueMsg(ctx, q, issue, EventIssueAssignedToMe, baseURL, r)
	})
}

func SendIssueResolved(ctx context.Context, q *db.Queries, n Notifier, gn Notifier, issue db.IssueReport, baseURL string) {
	sendIssueBroadcastAndPersonal(ctx, q, n, gn, issue, EventIssueResolved, baseURL)
}

func SendIssueCommented(ctx context.Context, q *db.Queries, n Notifier, gn Notifier, issue db.IssueReport, baseURL string) {
	sendIssueBroadcastAndPersonal(ctx, q, n, gn, issue, EventIssueCommented, baseURL)
}

func sendIssueBroadcastAndPersonal(ctx context.Context, q *db.Queries, n Notifier, gn Notifier, issue db.IssueReport, event, baseURL string) {
	reporter, err := q.GetUser(ctx, db.GetUserParams{ID: issue.ReporterID, GroupID: issue.GroupID})
	if err != nil {
		return
	}
	assignees, _ := q.ListIssueAssignees(ctx, db.ListIssueAssigneesParams{IssueID: issue.ID, GroupID: issue.GroupID})
	mgrTeam := managerTeamID(ctx, q, issue.GroupID)
	ds := loadDispatchSettings(ctx, q, issue.GroupID, formatUUID(mgrTeam))
	tk := issueThreadKey(issue)
	broadcastMsg, opener, detail := issueBroadcastData(ctx, q, issue, event, baseURL)
	sendBroadcastEmail(ctx, q, n, issue.GroupID, mgrTeam, ds, event, issue.ID, tk, broadcastMsg)
	sendBroadcastGChat(ctx, q, gn, issue.GroupID, mgrTeam, ds, event, issue.ID, tk, opener, detail)

	recipients := []recipient{fromGetUserRow(reporter)}
	for _, a := range assignees {
		if u, err := q.GetUser(ctx, db.GetUserParams{ID: a.UserID, GroupID: issue.GroupID}); err == nil {
			recipients = append(recipients, fromGetUserRow(u))
		}
	}
	for _, r := range dedup(recipients) {
		r := r
		sendTo(ctx, q, n, ds, issue.GroupID, r, event, "email", issue.ID, tk, func(lang string) Message {
			return issueMsg(ctx, q, issue, event, baseURL, r)
		})
	}
}

