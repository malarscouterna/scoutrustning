package notifications

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/malarscouterna/scoutrustning/api/internal/db"
)

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
	ds := loadDispatchSettings(ctx, q, b.GroupID, formatUUID(b.UsedByTeamID))
	recipients := bookingRecipients(ctx, q, b.GroupID, b.CreatedBy, b.UsedByTeamID)
	tk := "booking_" + formatUUID(b.ID)
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
		sendTo(ctx, q, n, ds, b.GroupID, r, EventBookingReminder, "email", b.ID, tk, func(lang string) Message {
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

// SendArchiveWarnings sends a one-time booking_archive_warning - to the team's broadcast
// channels (Gruppkanal email/GChat) plus a personal email to the creator and team members -
// for any draft-with-items or rejected-awaiting-resubmission booking whose auto-archive
// deadline (docs/implementation/pre-release.md "Booking auto-archive setting") falls within the next 24
// hours. Deduped via notification_log like reminders/overdue alerts.
func SendArchiveWarnings(ctx context.Context, q *db.Queries, n, gn Notifier, baseURL string) {
	bookings, err := q.GetBookingsNearingArchive(ctx)
	if err != nil {
		slog.Error("scheduler: GetBookingsNearingArchive failed", "error", err)
		return
	}
	for _, b := range bookings {
		sendArchiveWarningForBooking(ctx, q, n, gn, b, baseURL)
	}
}

func sendArchiveWarningForBooking(ctx context.Context, q *db.Queries, n, gn Notifier, row db.GetBookingsNearingArchiveRow, baseURL string) {
	b := db.Booking{
		ID: row.ID, GroupID: row.GroupID, CreatedBy: row.CreatedBy,
		UsedByTeamID: row.UsedByTeamID, StartDate: row.StartDate, EndDate: row.EndDate,
		Status: row.Status, Title: row.Title,
	}
	ds := loadDispatchSettings(ctx, q, b.GroupID, formatUUID(b.UsedByTeamID))
	tk := bookingThreadKey(b)

	// Broadcast to the team's shared channels first. Unlike the other broadcast events
	// (confirmed/rejected/cancelled), which fire exactly once from a handler action, this
	// one is discovered by hourly polling - so guard on notification_log the same way the
	// personal loop below does, using the same sentinel user IDs sendBroadcastEmail/GChat
	// log under, to avoid re-broadcasting if a booking is somehow caught in two ticks.
	broadcastEmailSent, _ := q.HasNotificationBeenSent(ctx, db.HasNotificationBeenSentParams{
		EntityID: b.ID, EventType: EventBookingArchiveWarning, UserID: "broadcast:" + formatUUID(b.UsedByTeamID), Channel: "email",
	})
	if !broadcastEmailSent {
		broadcastMsg := archiveWarningMsg(ctx, q, b, row.ArchiveDeadline, baseURL, recipient{lang: "sv"})
		sendBroadcastEmail(ctx, q, n, b.GroupID, b.UsedByTeamID, ds, EventBookingArchiveWarning, b.ID, tk, broadcastMsg)
	}
	gchatSent, _ := q.HasNotificationBeenSent(ctx, db.HasNotificationBeenSentParams{
		EntityID: b.ID, EventType: EventBookingArchiveWarning, UserID: "gchat:" + formatUUID(b.UsedByTeamID), Channel: "gchat",
	})
	if !gchatSent {
		opener, detail := bookingBroadcastTexts(ctx, q, b, EventBookingArchiveWarning, baseURL)
		sendBroadcastGChat(ctx, q, gn, b.GroupID, b.UsedByTeamID, ds, EventBookingArchiveWarning, b.ID, tk, opener, detail)
	}

	for _, r := range bookingRecipients(ctx, q, b.GroupID, b.CreatedBy, b.UsedByTeamID) {
		r := r
		sent, err := q.HasNotificationBeenSent(ctx, db.HasNotificationBeenSentParams{
			EntityID: b.ID, EventType: EventBookingArchiveWarning, UserID: r.id, Channel: "email",
		})
		if err != nil || sent {
			continue
		}
		sendTo(ctx, q, n, ds, b.GroupID, r, EventBookingArchiveWarning, "email", b.ID, tk, func(lang string) Message {
			return archiveWarningMsg(ctx, q, b, row.ArchiveDeadline, baseURL, r)
		})
	}
}

func sendOverdueForBooking(ctx context.Context, q *db.Queries, n Notifier, b db.GetAllOverdueBookingsRow, today pgtype.Date, baseURL string) {
	ds := loadDispatchSettings(ctx, q, b.GroupID, formatUUID(b.UsedByTeamID))
	booking := db.Booking{
		ID: b.ID, GroupID: b.GroupID, CreatedBy: b.CreatedBy,
		UsedByTeamID: b.UsedByTeamID, StartDate: b.StartDate, EndDate: b.EndDate,
		Status: "picked_up",
	}
	tk := "booking_" + formatUUID(b.ID)

	recipients := bookingRecipients(ctx, q, b.GroupID, b.CreatedBy, b.UsedByTeamID)
	managers, _ := q.GetGroupManagers(ctx, b.GroupID)
	for _, m := range managers {
		recipients = append(recipients, fromGetGroupManagersRow(m))
	}

	for _, r := range dedup(recipients) {
		r := r
		sent, err := q.HasNotificationBeenSent(ctx, db.HasNotificationBeenSentParams{
			EntityID: b.ID, EventType: EventBookingOverdue, UserID: r.id, Channel: "email",
		})
		if err != nil || sent {
			continue
		}
		sendTo(ctx, q, n, ds, b.GroupID, r, EventBookingOverdue, "email", b.ID, tk, func(lang string) Message {
			return bookingMsg(ctx, q, booking, EventBookingOverdue, baseURL, r)
		})
	}
}

