package handler

import (
	"context"
	"log/slog"

	"github.com/malarscouterna/scoutrustning/api/internal/db"
)

// ArchiveExpiredBookings cancels every booking past its group's auto-archive
// deadline (draft-with-items or rejected-awaiting-resubmission, per
// docs/implementation/pre-release.md "Booking auto-archive setting"), releasing their items
// and recording the archival in the booking's event thread rather than
// deleting the row - unlike a manual Cancel of a draft, so the history stays
// visible for anyone who had the booking open.
func ArchiveExpiredBookings(ctx context.Context, q *db.Queries) (int, error) {
	bookings, err := q.GetBookingsPastArchiveDeadline(ctx)
	if err != nil {
		return 0, err
	}
	for _, b := range bookings {
		if _, err := q.UpdateBookingStatus(ctx, db.UpdateBookingStatusParams{
			ID: b.ID, GroupID: b.GroupID, Status: "cancelled",
		}); err != nil {
			slog.Error("auto-archive: failed to cancel booking", "booking_id", b.ID, "error", err)
			continue
		}
		message := "Arkiverades automatiskt efter för lång inaktivitet - föremålen är inte längre reserverade åt er. Kopiera bokningen om utrustningen fortfarande behövs."
		if b.Status == "rejected" {
			message = "Arkiverades automatiskt - inte återinskickad i tid, föremålen är inte längre reserverade åt er. Kopiera bokningen om utrustningen fortfarande behövs."
		}
		if _, err := q.CreateBookingEvent(ctx, db.CreateBookingEventParams{
			GroupID:   b.GroupID,
			BookingID: b.ID,
			ActorID:   b.CreatedBy,
			EventType: "auto_archived",
			Message:   message,
			Metadata:  []byte("{}"),
		}); err != nil {
			slog.Error("auto-archive: failed to log event", "booking_id", b.ID, "error", err)
		}
	}
	return len(bookings), nil
}
