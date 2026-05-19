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

func fromGetGroupManagersRow(r db.GetGroupManagersRow) recipient {
	lang := "sv"
	if r.Language.Valid {
		lang = r.Language.String
	}
	return recipient{id: r.ID, name: r.Name, email: r.Email, notifEmail: r.NotificationEmail.String, lang: lang, maxAccessLevel: r.MaxAccessLevel, notifPrefs: r.NotificationPrefs}
}

func fromGetTeamMembersRow(r db.GetTeamMembersWithEmailsRow) recipient {
	lang := "sv"
	if r.Language.Valid {
		lang = r.Language.String
	}
	return recipient{id: r.ID, name: r.Name, email: r.Email, notifEmail: r.NotificationEmail.String, lang: lang, maxAccessLevel: r.MaxAccessLevel, notifPrefs: r.NotificationPrefs}
}

func fromGetUserRow(r db.User) recipient {
	lang := "sv"
	if r.Language.Valid {
		lang = r.Language.String
	}
	return recipient{id: r.ID, name: r.Name, email: r.Email, notifEmail: r.NotificationEmail.String, lang: lang, maxAccessLevel: r.MaxAccessLevel, notifPrefs: r.NotificationPrefs}
}

// sendTo sends msg to r based on their personal email policy for the event.
// For personal events (teamID="") PolicyAlways/PolicyNever applies directly.
// For team/role events, policy is resolved through user → team → group → system.
// entityID and threadKey drive email threading; pass zero UUID / "" to skip threading.
func sendTo(ctx context.Context, q *db.Queries, n Notifier, groupID, teamID string, r recipient, event, channel string, entityID pgtype.UUID, threadKey string, msg func(lang string) Message) {
	policy := ResolvePersonalEmailPolicy(ctx, q, r.id, groupID, teamID, event)
	switch policy {
	case PolicyNever:
		return
	case PolicyIfNoBroadcast:
		// Suppress personal email only if the team has at least one effective Gruppkanal
		// channel that is actually configured (has a real endpoint). A channel appearing in
		// the opted-in list but with no endpoint (e.g. "email" inherited from group default
		// but no notification_email set) does not count — nothing would be delivered there.
		team := GetTeamNotifSettings(ctx, q, groupID, teamID)
		groupDefaultsRow, _ := q.GetGroupNotificationDefaults(ctx, groupID)
		effective := EffectiveGruppkanalChannels(team.GruppkanalChannels, groupDefaultsRow.DefaultGruppkanalChannels)
		hasConfiguredBroadcast := false
		for _, ch := range effective {
			if ch == "email" && team.HasNotificationEmail {
				hasConfiguredBroadcast = true
				break
			}
			if ch == "gchat" && team.HasGchatSpace {
				hasConfiguredBroadcast = true
				break
			}
		}
		if hasConfiguredBroadcast {
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
// First message is a compact opener (team name + summary); second is the full detail, sent as a
// reply in the same thread via entityThreadKey. All events for the same booking/issue share one
// thread in a space, even when triggered by different teams. The team name prefix in the opener
// distinguishes which team the message is addressed to.
// Fires only if "gchat" is in the team's effective Gruppkanal and IsGruppkanalEnabled.
// Uses sentinel user_id "gchat:<teamID>" in notification_log.
func sendBroadcastGChat(ctx context.Context, q *db.Queries, gn Notifier, groupID string, teamID pgtype.UUID, event string, entityID pgtype.UUID, entityThreadKey, openerText, detailText string) {
	if !teamID.Valid {
		return
	}
	team, err := q.GetTeam(ctx, db.GetTeamParams{ID: teamID, GroupID: groupID})
	if err != nil || !team.GchatSpaceID.Valid || team.GchatSpaceID.String == "" {
		return
	}
	teamSettings := GetTeamNotifSettings(ctx, q, groupID, teamIDStr(teamID))
	groupDefaultsRow, _ := q.GetGroupNotificationDefaults(ctx, groupID)
	effective := EffectiveGruppkanalChannels(teamSettings.GruppkanalChannels, groupDefaultsRow.DefaultGruppkanalChannels)
	if !containsChannel(effective, "gchat") {
		return
	}
	if !IsGruppkanalEnabled(ctx, q, groupID, teamIDStr(teamID), event) {
		return
	}

	// Use the entity-level threadKey so all events for the same booking/issue land in
	// one thread regardless of which team triggered them.
	gchatThreadKey := entityThreadKey

	// In dev mode (LabelTeam=true) prepend the team name so messages from different
	// teams are easy to tell apart when one space is linked to both roles.
	labeledOpener := openerText
	if n, ok := gn.(*GChatNotifier); ok && n.LabelTeam {
		labeledOpener = "*" + team.Name + "*  " + openerText
	}

	space := team.GchatSpaceID.String
	sentinelUserID := "gchat:" + teamIDStr(teamID)

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

	// Use SendPaired when available: it captures the thread name from the opener response
	// and uses it for the detail reply, ensuring both messages land in the same thread.
	// Fall back to two separate Send calls for non-GChatNotifier implementations (e.g. tests).
	if gcn, ok := gn.(*GChatNotifier); ok {
		openerErr, detailErr := gcn.SendPaired(ctx, groupID, space, labeledOpener, detailText, gchatThreadKey)
		if openerErr != nil {
			slog.Error("gchat broadcast opener failed", "event", event, "space", space, "error", openerErr)
			logGChat("failed", openerErr.Error())
			return
		}
		logGChat("sent", "")
		if detailText != "" {
			if detailErr != nil {
				slog.Error("gchat broadcast detail failed", "event", event, "space", space, "error", detailErr)
			}
			logGChat("sent", func() string {
				if detailErr != nil {
					return detailErr.Error()
				}
				return ""
			}())
		}
		return
	}

	// Fallback for non-GChatNotifier (CapturingNotifier in tests).
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
// Fires only if "email" is in the team's effective Gruppkanal and IsGruppkanalEnabled.
// Uses sentinel user_id "broadcast:<teamID>" in notification_log so threading is independent
// from personal sends.
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
	teamSettings := GetTeamNotifSettings(ctx, q, groupID, teamIDStr(teamID))
	groupDefaultsRow, _ := q.GetGroupNotificationDefaults(ctx, groupID)
	effective := EffectiveGruppkanalChannels(teamSettings.GruppkanalChannels, groupDefaultsRow.DefaultGruppkanalChannels)
	if !containsChannel(effective, "email") {
		return
	}
	if !IsGruppkanalEnabled(ctx, q, groupID, teamIDStr(teamID), event) {
		return
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

// issueBroadcastTexts returns the GChat opener and detail text for an issue broadcast.
func issueBroadcastTexts(ctx context.Context, q *db.Queries, issue db.IssueReport, event, baseURL string) (opener, detail string) {
	data := fetchIssueEmailData(ctx, q, issue, event, "sv", "", baseURL)
	return IssueOpenerText(data), IssueDetailText(data)
}

// issueBroadcastEmail builds a broadcast email Message for an issue event (no personal recipient).
func issueBroadcastEmail(ctx context.Context, q *db.Queries, issue db.IssueReport, event, baseURL string) Message {
	data := fetchIssueEmailData(ctx, q, issue, event, "sv", "", baseURL)
	htmlBody, textBody := renderIssueEmail(data)
	return Message{
		Subject:  i18n.T("sv", "email_subject_"+event, map[string]string{"title": issue.Title}),
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
	mgrTeam := managerTeamID(ctx, q, b.GroupID)
	broadcastMsg := bookingMsg(ctx, q, b, EventBookingNeedsApproval, baseURL, recipient{lang: "sv"})
	opener, detail := bookingBroadcastTexts(ctx, q, b, EventBookingNeedsApproval, baseURL)
	sendBroadcastEmail(ctx, q, n, b.GroupID, mgrTeam, EventBookingNeedsApproval, b.ID, tk, broadcastMsg)
	sendBroadcastGChat(ctx, q, gn, b.GroupID, mgrTeam, EventBookingNeedsApproval, b.ID, tk, opener, detail)
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
	mgrTeam := managerTeamID(ctx, q, b.GroupID)
	broadcastMsg := bookingMsg(ctx, q, b, EventBookingSubmittedNoApproval, baseURL, recipient{lang: "sv"})
	opener, detail := bookingBroadcastTexts(ctx, q, b, EventBookingSubmittedNoApproval, baseURL)
	sendBroadcastEmail(ctx, q, n, b.GroupID, mgrTeam, EventBookingSubmittedNoApproval, b.ID, tk, broadcastMsg)
	sendBroadcastGChat(ctx, q, gn, b.GroupID, mgrTeam, EventBookingSubmittedNoApproval, b.ID, tk, opener, detail)
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
	opener, detail := bookingBroadcastTexts(ctx, q, b, EventBookingConfirmed, baseURL)
	sendBroadcastEmail(ctx, q, n, b.GroupID, b.UsedByTeamID, EventBookingConfirmed, b.ID, tk, broadcastMsg)
	sendBroadcastGChat(ctx, q, gn, b.GroupID, b.UsedByTeamID, EventBookingConfirmed, b.ID, tk, opener, detail)
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
	opener, detail := bookingBroadcastTexts(ctx, q, b, EventBookingRejected, baseURL)
	sendBroadcastEmail(ctx, q, n, b.GroupID, b.UsedByTeamID, EventBookingRejected, b.ID, tk, broadcastMsg)
	sendBroadcastGChat(ctx, q, gn, b.GroupID, b.UsedByTeamID, EventBookingRejected, b.ID, tk, opener, detail)
	sendTo(ctx, q, n, b.GroupID, tid, r, EventBookingRejected, "email", b.ID, tk, func(lang string) Message {
		return bookingMsg(ctx, q, b, EventBookingRejected, baseURL, r)
	})
}

func SendBookingCancelled(ctx context.Context, q *db.Queries, n Notifier, gn Notifier, b db.Booking, baseURL string) {
	tid := teamIDStr(b.UsedByTeamID)
	tk := bookingThreadKey(b)
	broadcastMsg := bookingMsg(ctx, q, b, EventBookingCancelled, baseURL, recipient{lang: "sv"})
	opener, detail := bookingBroadcastTexts(ctx, q, b, EventBookingCancelled, baseURL)
	sendBroadcastEmail(ctx, q, n, b.GroupID, b.UsedByTeamID, EventBookingCancelled, b.ID, tk, broadcastMsg)
	sendBroadcastGChat(ctx, q, gn, b.GroupID, b.UsedByTeamID, EventBookingCancelled, b.ID, tk, opener, detail)
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

func SendIssueCreated(ctx context.Context, q *db.Queries, n Notifier, gn Notifier, issue db.IssueReport, baseURL string) {
	tk := issueThreadKey(issue)
	mgrTeam := managerTeamID(ctx, q, issue.GroupID)
	mgrTeamID := teamIDStr(mgrTeam)
	opener, detail := issueBroadcastTexts(ctx, q, issue, EventIssueCreated, baseURL)
	sendBroadcastEmail(ctx, q, n, issue.GroupID, mgrTeam, EventIssueCreated, issue.ID, tk, issueBroadcastEmail(ctx, q, issue, EventIssueCreated, baseURL))
	sendBroadcastGChat(ctx, q, gn, issue.GroupID, mgrTeam, EventIssueCreated, issue.ID, tk, opener, detail)
	for _, r := range groupManagers(ctx, q, issue.GroupID) {
		r := r
		sendTo(ctx, q, n, issue.GroupID, mgrTeamID, r, EventIssueCreated, "email", issue.ID, tk, func(lang string) Message {
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

func SendIssueResolved(ctx context.Context, q *db.Queries, n Notifier, gn Notifier, issue db.IssueReport, baseURL string) {
	reporter, err := q.GetUser(ctx, db.GetUserParams{ID: issue.ReporterID, GroupID: issue.GroupID})
	if err != nil {
		return
	}
	assignees, _ := q.ListIssueAssignees(ctx, db.ListIssueAssigneesParams{IssueID: issue.ID, GroupID: issue.GroupID})
	tk := issueThreadKey(issue)

	mgrTeam := managerTeamID(ctx, q, issue.GroupID)
	mgrTeamID := teamIDStr(mgrTeam)
	opener, detail := issueBroadcastTexts(ctx, q, issue, EventIssueResolved, baseURL)
	sendBroadcastEmail(ctx, q, n, issue.GroupID, mgrTeam, EventIssueResolved, issue.ID, tk, issueBroadcastEmail(ctx, q, issue, EventIssueResolved, baseURL))
	sendBroadcastGChat(ctx, q, gn, issue.GroupID, mgrTeam, EventIssueResolved, issue.ID, tk, opener, detail)

	recipients := []recipient{fromGetUserRow(reporter)}
	for _, a := range assignees {
		if u, err := q.GetUser(ctx, db.GetUserParams{ID: a.UserID, GroupID: issue.GroupID}); err == nil {
			recipients = append(recipients, fromGetUserRow(u))
		}
	}
	for _, r := range dedup(recipients) {
		r := r
		sendTo(ctx, q, n, issue.GroupID, mgrTeamID, r, EventIssueResolved, "email", issue.ID, tk, func(lang string) Message {
			return issueMsg(ctx, q, issue, EventIssueResolved, baseURL, r)
		})
	}
}

func SendIssueCommented(ctx context.Context, q *db.Queries, n Notifier, gn Notifier, issue db.IssueReport, baseURL string) {
	reporter, err := q.GetUser(ctx, db.GetUserParams{ID: issue.ReporterID, GroupID: issue.GroupID})
	if err != nil {
		return
	}
	assignees, _ := q.ListIssueAssignees(ctx, db.ListIssueAssigneesParams{IssueID: issue.ID, GroupID: issue.GroupID})
	tk := issueThreadKey(issue)

	mgrTeam := managerTeamID(ctx, q, issue.GroupID)
	mgrTeamID := teamIDStr(mgrTeam)
	opener, detail := issueBroadcastTexts(ctx, q, issue, EventIssueCommented, baseURL)
	sendBroadcastEmail(ctx, q, n, issue.GroupID, mgrTeam, EventIssueCommented, issue.ID, tk, issueBroadcastEmail(ctx, q, issue, EventIssueCommented, baseURL))
	sendBroadcastGChat(ctx, q, gn, issue.GroupID, mgrTeam, EventIssueCommented, issue.ID, tk, opener, detail)

	recipients := []recipient{fromGetUserRow(reporter)}
	for _, a := range assignees {
		if u, err := q.GetUser(ctx, db.GetUserParams{ID: a.UserID, GroupID: issue.GroupID}); err == nil {
			recipients = append(recipients, fromGetUserRow(u))
		}
	}
	for _, r := range dedup(recipients) {
		r := r
		sendTo(ctx, q, n, issue.GroupID, mgrTeamID, r, EventIssueCommented, "email", issue.ID, tk, func(lang string) Message {
			return issueMsg(ctx, q, issue, EventIssueCommented, baseURL, r)
		})
	}
}

