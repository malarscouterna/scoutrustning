package handler

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/malarscouterna/scoutrustning/api/internal/db"
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
// booking exists but no equivalent unit was found - the "notify instead"
// half of decision 4 is wired in separately (never reached in upgradeOnly mode,
// which never notifies per decision 6).
func ResolveBlockedItemsForArticle(ctx context.Context, q *db.Queries, groupID string, articleID pgtype.UUID, checkDate time.Time, upgradeOnly bool) (bool, error) {
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
	replacementID, err := q.FindReplacementArticle(ctx, db.FindReplacementArticleParams{
		GroupID:        groupID,
		CommercialName: article.CommercialName,
		LocationID:     article.LocationID,
		Statuses:       statuses,
		ExcludeIds:     []pgtype.UUID{articleID},
		StartDate:      earliest.StartDate,
		EndDate:        earliest.EndDate,
	})
	if err != nil {
		// No equivalent unit available - fall through to notification (later phase).
		return false, nil
	}

	replacement, err := q.GetArticle(ctx, db.GetArticleParams{ID: replacementID, GroupID: groupID})
	if err != nil {
		return false, err
	}

	if _, err := q.SwapBookingItemArticleByArticle(ctx, db.SwapBookingItemArticleByArticleParams{
		NewArticleID: replacementID,
		OldArticleID: articleID,
		BookingID:    earliest.BookingID,
		GroupID:      groupID,
	}); err != nil {
		return false, err
	}

	if _, err := q.CreateBookingEvent(ctx, db.CreateBookingEventParams{
		GroupID:   groupID,
		BookingID: earliest.BookingID,
		ActorID:   earliest.CreatedBy,
		EventType: "swap",
		Message:   "Bytte automatiskt ut " + article.CommonName + " mot " + replacement.CommonName + " eftersom det ursprungliga föremålet inte längre var tillgängligt.",
		Metadata:  []byte("{}"),
	}); err != nil {
		slog.Error("swap: failed to log event", "booking_id", earliest.BookingID, "error", err)
	}

	return true, nil
}

// ResolveOverdueSwaps is the nightly entry point (docs/delayed-return-swap.md
// decision 5), folded into the existing booking-cleanup loop. It enumerates
// every delayed/overdue item across all groups and tries to resolve each
// affected article's blocked bookings once. Multiple items sharing the same
// (group, article) pair are only resolved once per pass.
func ResolveOverdueSwaps(ctx context.Context, q *db.Queries) (int, error) {
	items, err := q.FindDelayedOrOverdueItems(ctx, pgtype.Date{Time: time.Now(), Valid: true})
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

		ok, err := ResolveBlockedItemsForArticle(ctx, q, item.GroupID, item.ArticleID, time.Now(), false)
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
