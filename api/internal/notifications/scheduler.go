package notifications

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/malarscouterna/ms-utrustning/api/internal/db"
)

func pgText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

// StartScheduler launches the daily notification scheduler in a background goroutine.
// It fires at NOTIFICATION_REMINDER_TIME (default "08:00") in the server's local timezone.
func StartScheduler(q *db.Queries, n Notifier, baseURL string) {
	go runScheduler(q, n, baseURL)
}

func runScheduler(q *db.Queries, n Notifier, baseURL string) {
	hour, minute := parseReminderTime(os.Getenv("NOTIFICATION_REMINDER_TIME"))

	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}
		time.Sleep(time.Until(next))

		today := pgtype.Date{Time: time.Now(), Valid: true}
		ctx := context.Background()
		slog.Info("running scheduled notifications", "date", today.Time.Format("2006-01-02"))
		SendReminders(ctx, q, n, today, baseURL)
		SendOverdueAlerts(ctx, q, n, today, baseURL)
	}
}

func parseReminderTime(s string) (hour, minute int) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) == 2 {
		h, err1 := strconv.Atoi(parts[0])
		m, err2 := strconv.Atoi(parts[1])
		if err1 == nil && err2 == nil && h >= 0 && h < 24 && m >= 0 && m < 60 {
			return h, m
		}
	}
	return 8, 0
}

// SendReminders sends booking_reminder to creators and team members of bookings
// starting on date.Time + 1 day (i.e., date is "today", bookings start "tomorrow").
func SendReminders(ctx context.Context, q *db.Queries, n Notifier, today pgtype.Date, baseURL string) {
	tomorrow := pgtype.Date{Time: today.Time.AddDate(0, 0, 1), Valid: true}
	bookings, err := q.GetAllBookingsStartingOn(ctx, tomorrow)
	if err != nil {
		slog.Error("scheduler: GetAllBookingsStartingOn failed", "error", err)
		return
	}
	for _, b := range bookings {
		sendReminderForBooking(ctx, q, n, b, baseURL)
	}
}

func sendReminderForBooking(ctx context.Context, q *db.Queries, n Notifier, b db.GetAllBookingsStartingOnRow, baseURL string) {
	recipients := bookingRecipients(ctx, q, b.GroupID, b.CreatedBy, b.UsedByTeamID)
	for _, r := range recipients {
		r := r
		sent, err := q.HasNotificationBeenSent(ctx, db.HasNotificationBeenSentParams{
			EntityID:  b.ID,
			EventType: EventBookingReminder,
			UserID:    r.id,
			Channel:   "email",
		})
		if err != nil || sent {
			continue
		}
		sendTo(ctx, q, n, b.GroupID, teamIDStr(b.UsedByTeamID), r, EventBookingReminder, "email", b.ID, "booking_"+teamIDStr(b.ID), func(lang string) Message {
			booking := db.Booking{
				ID:           b.ID,
				GroupID:      b.GroupID,
				CreatedBy:    b.CreatedBy,
				UsedByTeamID: b.UsedByTeamID,
				StartDate:    b.StartDate,
				EndDate:      b.EndDate,
				Status:       "confirmed",
			}
			return bookingMsg(ctx, q, booking, EventBookingReminder, baseURL, r)
		})
	}
}

// SendOverdueAlerts sends booking_overdue once per (booking, user) for picked_up
// bookings whose end_date is before today. Uses notification_log to prevent duplicates.
func SendOverdueAlerts(ctx context.Context, q *db.Queries, n Notifier, today pgtype.Date, baseURL string) {
	bookings, err := q.GetAllOverdueBookings(ctx, today)
	if err != nil {
		slog.Error("scheduler: GetAllOverdueBookings failed", "error", err)
		return
	}
	for _, b := range bookings {
		sendOverdueForBooking(ctx, q, n, b, today, baseURL)
	}
}

func sendOverdueForBooking(ctx context.Context, q *db.Queries, n Notifier, b db.GetAllOverdueBookingsRow, today pgtype.Date, baseURL string) {
	recipients := bookingRecipients(ctx, q, b.GroupID, b.CreatedBy, b.UsedByTeamID)

	// Managers who have opted into booking_overdue (not in system default, must be explicit).
	managers, _ := q.GetGroupManagers(ctx, b.GroupID)
	for _, m := range managers {
		recipients = append(recipients, fromGetGroupManagersRow(m))
	}
	recipients = dedup(recipients)

	for _, r := range recipients {
		r := r
		sent, err := q.HasNotificationBeenSent(ctx, db.HasNotificationBeenSentParams{
			EntityID:  b.ID,
			EventType: EventBookingOverdue,
			UserID:    r.id,
			Channel:   "email",
		})
		if err != nil || sent {
			continue
		}
		policy := ResolvePersonalEmailPolicy(ctx, q, r.id, b.GroupID, teamIDStr(b.UsedByTeamID), EventBookingOverdue)
		if policy == PolicyNever {
			continue
		}
		if policy == PolicyIfNoBroadcast {
			teamSettings := GetTeamNotifSettings(ctx, q, b.GroupID, teamIDStr(b.UsedByTeamID))
			groupDefaultsRow, _ := q.GetGroupNotificationDefaults(ctx, b.GroupID)
			effective := EffectiveGruppkanalChannels(teamSettings.GruppkanalChannels, groupDefaultsRow.DefaultGruppkanalChannels)
			if len(effective) > 0 {
				continue
			}
		}
		booking := db.Booking{
			ID:           b.ID,
			GroupID:      b.GroupID,
			CreatedBy:    b.CreatedBy,
			UsedByTeamID: b.UsedByTeamID,
			StartDate:    b.StartDate,
			EndDate:      b.EndDate,
			Status:       "picked_up",
		}
		tk := "booking_" + teamIDStr(b.ID)
		idSuffix := r.id
		if len(idSuffix) > 8 {
			idSuffix = idSuffix[:8]
		}
		newMsgID := tk + "-" + idSuffix
		msg := bookingMsg(ctx, q, booking, EventBookingOverdue, baseURL, r)
		msg.GroupID = b.GroupID
		prior, err := q.GetThreadMessageID(ctx, db.GetThreadMessageIDParams{
			ThreadKey: pgtype.Text{String: tk, Valid: true},
			UserID:    r.id,
			Channel:   "email",
		})
		logMsgID := pgtype.Text{}
		if err == nil && prior.Valid {
			msg.InReplyTo = prior.String
		} else {
			msg.MessageID = newMsgID
			logMsgID = pgtype.Text{String: newMsgID, Valid: true}
		}
		sendErr := n.Send(ctx, msg)

		status := "sent"
		errStr := ""
		if sendErr != nil {
			status = "failed"
			errStr = sendErr.Error()
			slog.Error("overdue notification failed", "booking", b.ID, "user", r.id, "error", sendErr)
		}
		_ = q.LogNotification(ctx, db.LogNotificationParams{
			GroupID:   b.GroupID,
			UserID:    r.id,
			EventType: EventBookingOverdue,
			EntityID:  b.ID,
			Channel:   "email",
			Status:    status,
			Error:     pgText(errStr),
			ThreadKey: pgtype.Text{String: tk, Valid: true},
			MessageID: logMsgID,
		})
	}
}

// bookingRecipients returns the creator and all team members for a booking,
// deduped. Does not filter by preference — callers do that via sendTo or IsEnabled.
func bookingRecipients(ctx context.Context, q *db.Queries, groupID, createdBy string, teamID pgtype.UUID) []recipient {
	var out []recipient
	if creator, ok := bookingCreator(ctx, q, groupID, createdBy); ok {
		out = append(out, creator)
	}
	out = append(out, teamMembers(ctx, q, groupID, teamID)...)
	return dedup(out)
}
