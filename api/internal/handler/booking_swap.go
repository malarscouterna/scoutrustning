package handler

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/malarscouterna/scoutrustning/api/internal/db"
	"github.com/malarscouterna/scoutrustning/api/internal/notifications"
)

// ResolveBlockedItemsForArticle looks for the earliest non-terminal booking
// (docs/delayed-return-swap.md decision 2) that is blocked by articleID as of
// checkDate, and tries to silently substitute an equivalent available unit
// into it. If more than one waiting booking is blocked, only the earliest by
// start_date is resolved per call - a later pass (the next mark-delayed call,
// or the nightly job) picks up whatever's left, per the doc's accepted
// simplification.
//
// upgradeOnly restricts the replacement search to strictly 'ok' units (decision
// 6's reported_usable case: the current unit is still bookable, so this is an
// opportunistic upgrade, not a rescue - a 'reported_usable' replacement would be
// no better than what's already assigned). When false, 'reported_usable' also
// qualifies as a replacement, matching every other availability query.
//
// Returns swapped=true if a substitution was made. Returns swapped=false with
// no error both when nothing is waiting on this article and when a waiting
// booking exists but no equivalent unit was found - in the latter case (and
// only when upgradeOnly is false, per decision 6) it fires the "no swap
// available" notification (decision 4) at the waiting booking instead.
func ResolveBlockedItemsForArticle(ctx context.Context, pool *pgxpool.Pool, q *db.Queries, n, gn notifications.Notifier, baseURL, groupID string, articleID pgtype.UUID, checkDate time.Time, upgradeOnly bool) (bool, error) {
	waiting, err := q.FindWaitingBookingItemsForArticle(ctx, db.FindWaitingBookingItemsForArticleParams{
		ArticleID: articleID,
		GroupID:   groupID,
		CheckDate: pgtype.Date{Time: checkDate, Valid: true},
	})
	if err != nil {
		return false, err
	}
	if len(waiting) == 0 {
		return false, nil
	}
	earliest := waiting[0]

	article, err := q.GetArticle(ctx, db.GetArticleParams{ID: articleID, GroupID: groupID})
	if err != nil {
		return false, err
	}

	statuses := []string{"ok", "reported_usable"}
	if upgradeOnly {
		statuses = []string{"ok"}
	}

	// The find-and-swap must run in one transaction: FindReplacementArticle
	// takes FOR UPDATE SKIP LOCKED on the candidate row, and that lock is only
	// meaningful until the swap that consumes it commits. Without this, the
	// nightly job and a request-time swap can both SELECT the same free unit
	// before either commits its UPDATE, double-booking it.
	tx, err := pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	txQ := q.WithTx(tx)

	replacementID, err := txQ.FindReplacementArticle(ctx, db.FindReplacementArticleParams{
		GroupID:        groupID,
		CommercialName: article.CommercialName,
		LocationID:     article.LocationID,
		Statuses:       statuses,
		ExcludeIds:     []pgtype.UUID{articleID},
		StartDate:      earliest.StartDate,
		EndDate:        earliest.EndDate,
	})
	if err != nil {
		// No equivalent unit available. The reported_usable upgrade-only case never
		// notifies (decision 6) - nothing is actually blocked, since the current unit
		// remains bookable.
		if !upgradeOnly {
			waitingBooking, err := q.GetBooking(ctx, db.GetBookingParams{ID: earliest.BookingID, GroupID: groupID})
			if err == nil {
				b := db.Booking{
					ID: waitingBooking.ID, GroupID: waitingBooking.GroupID, CreatedBy: waitingBooking.CreatedBy,
					UsedByTeamID: waitingBooking.UsedByTeamID, StartDate: waitingBooking.StartDate,
					EndDate: waitingBooking.EndDate, Status: waitingBooking.Status, Title: waitingBooking.Title,
				}
				go notifications.SendBookingItemBlocked(context.Background(), q, n, gn, b, earliest.BookingItemID, article.CommonName, baseURL)
			}
		}
		return false, nil
	}

	replacement, err := txQ.GetArticle(ctx, db.GetArticleParams{ID: replacementID, GroupID: groupID})
	if err != nil {
		return false, err
	}

	if _, err := txQ.SwapBookingItemArticleByArticle(ctx, db.SwapBookingItemArticleByArticleParams{
		NewArticleID: replacementID,
		OldArticleID: articleID,
		BookingID:    earliest.BookingID,
		GroupID:      groupID,
	}); err != nil {
		return false, err
	}

	if _, err := txQ.CreateBookingEvent(ctx, db.CreateBookingEventParams{
		GroupID:   groupID,
		BookingID: earliest.BookingID,
		ActorID:   earliest.CreatedBy,
		EventType: "swap",
		Message:   "Bytte automatiskt ut " + article.CommonName + " mot " + replacement.CommonName + " eftersom det ursprungliga föremålet inte längre var tillgängligt.",
		Metadata:  []byte("{}"),
	}); err != nil {
		slog.Error("swap: failed to log event", "booking_id", earliest.BookingID, "error", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, err
	}

	return true, nil
}

// overdueSwapGracePeriod is how long a never-flagged item may sit overdue
// before the nightly job will swap it out from under its holder. Keeps a
// booking that's merely a day late from immediately losing its item to
// another booking just because someone else happened to be waiting - an
// explicit "delayed" mark during return (decision 1) is already a known
// problem and always resolves immediately, unaffected by this grace period.
//
// Effective precision is day-granular, not hour-granular: graceCutoff below
// carries a time-of-day component, but it's compared against b.end_date,
// which is date-only. Depending on what time of day the nightly job runs,
// the enforced grace period ranges from ~24h to ~72h rather than exactly
// 48h. This is accepted as-is (docs/delayed-return-swap.md) since end_date
// is date-only anyway - a booking is never "a few hours overdue" in this
// model, only "overdue as of a given day".
const overdueSwapGracePeriod = 48 * time.Hour

// ResolveOverdueSwaps is the nightly entry point (docs/delayed-return-swap.md
// decision 5), folded into the existing booking-cleanup loop. It enumerates
// every delayed/overdue item across all groups and tries to resolve each
// affected article's blocked bookings once. Multiple items sharing the same
// (group, article) pair are only resolved once per pass.
func ResolveOverdueSwaps(ctx context.Context, pool *pgxpool.Pool, q *db.Queries, n, gn notifications.Notifier, baseURL string) (int, error) {
	graceCutoff := time.Now().Add(-overdueSwapGracePeriod)
	items, err := q.FindDelayedOrOverdueItems(ctx, pgtype.Date{Time: graceCutoff, Valid: true})
	if err != nil {
		return 0, err
	}

	type key struct {
		groupID   string
		articleID pgtype.UUID
	}
	seen := map[key]bool{}
	swapped := 0
	for _, item := range items {
		k := key{groupID: item.GroupID, articleID: item.ArticleID}
		if seen[k] {
			continue
		}
		seen[k] = true

		ok, err := ResolveBlockedItemsForArticle(ctx, pool, q, n, gn, baseURL, item.GroupID, item.ArticleID, time.Now(), false)
		if err != nil {
			slog.Error("nightly swap resolution failed", "article_id", item.ArticleID, "error", err)
			continue
		}
		if ok {
			swapped++
		}
	}
	return swapped, nil
}
