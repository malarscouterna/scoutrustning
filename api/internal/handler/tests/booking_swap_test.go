package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/malarscouterna/scoutrustning/api/internal/handler"
	"github.com/malarscouterna/scoutrustning/api/internal/testutil"
)

// TestReturnFlow_DelayedTriggersSwap exercises the delayed-return swap
// (docs/delayed-return-swap.md): marking an item delayed on booking A, whose
// article is also assigned to a later booking B, silently substitutes the
// equivalent free unit into B and logs a "swap" event - rather than leaving B
// blocked on an article that won't come back in time.
func TestReturnFlow_DelayedTriggersSwap(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountReturnRoutes(env)

	leaderA := env.ClientAs("leader-yggdrasil")
	leaderB := env.ClientAs("leader-flaskpost")

	// setupReturnEnv seeds 2 identical articles ("ReturnTest 1" / "ReturnTest 2",
	// created in that order) and books+confirms+picks up 1 unit for booking A
	// over today..today+5d. AvailableArticlesExcludingBooking orders by
	// commercial_name, common_name (not created_at), so "ReturnTest 1" (X) is
	// always assigned first when both units are free.
	bookingA, itemIDs, articleIDs := setupReturnEnv(t, env, 2, 1)
	articleX := articleIDs[0] // "ReturnTest 1" - assigned to booking A
	articleY := articleIDs[1] // "ReturnTest 2" - still free

	// Booking B: a later, non-overlapping date range for a different team.
	// Both X and Y are free for these dates, so adding by commercial_name also
	// picks X deterministically.
	teamID := getTeamID(t, leaderB, "Flaskpostorné")
	now := time.Now()
	startB := now.AddDate(0, 0, 10).Format("2006-01-02")
	endB := now.AddDate(0, 0, 15).Format("2006-01-02")
	b, _ := json.Marshal(map[string]any{"start_date": startB, "end_date": endB, "used_by_team_id": teamID, "title": "Booking B"})
	resp, err := leaderB.Post("/api/v0/bookings", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	var bookingBResp map[string]any
	json.NewDecoder(resp.Body).Decode(&bookingBResp)
	resp.Body.Close()
	bookingB := bookingBResp["id"].(string)

	b, _ = json.Marshal(map[string]any{"commercial_name": "ReturnTest", "quantity": 1})
	resp, err = leaderB.Post("/api/v0/bookings/"+bookingB+"/items", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Confirm booking B lands in a non-terminal status and got assigned X.
	resp, _ = leaderB.Get("/api/v0/bookings/" + bookingB)
	var detailBefore map[string]any
	json.NewDecoder(resp.Body).Decode(&detailBefore)
	resp.Body.Close()
	bookingBStatus := detailBefore["booking"].(map[string]any)["status"].(string)
	nonTerminal := map[string]bool{"draft": true, "submitted": true, "approved": true, "confirmed": true}
	if !nonTerminal[bookingBStatus] {
		t.Fatalf("expected booking B in a non-terminal status, got %v", bookingBStatus)
	}
	itemsBefore := detailBefore["items"].([]any)
	if len(itemsBefore) != 1 {
		t.Fatalf("expected 1 item on booking B, got %d", len(itemsBefore))
	}
	itemB := itemsBefore[0].(map[string]any)
	if itemB["article_id"] != articleX {
		t.Fatalf("expected booking B assigned article X (%s), got %v", articleX, itemB["article_id"])
	}
	itemBID := itemB["id"].(string)

	// Mark booking A's item delayed, with an expected return date at/after
	// booking B's start date - this is what synchronously triggers
	// ResolveBlockedItemsForArticle for article X.
	expectedReturn := now.AddDate(0, 0, 12).Format("2006-01-02")
	b, _ = json.Marshal(map[string]any{
		"return_status":        "delayed",
		"expected_return_date": expectedReturn,
	})
	resp, err = leaderA.Put("/api/v0/bookings/"+bookingA+"/items/"+itemIDs[0]+"/return", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	t.Run("booking B item swapped to article Y", func(t *testing.T) {
		resp, _ := leaderB.Get("/api/v0/bookings/" + bookingB)
		defer resp.Body.Close()
		var detail map[string]any
		json.NewDecoder(resp.Body).Decode(&detail)
		items := detail["items"].([]any)
		if len(items) != 1 {
			t.Fatalf("expected 1 item on booking B, got %d", len(items))
		}
		item := items[0].(map[string]any)
		if item["id"] != itemBID {
			t.Errorf("expected same item id %s, got %v", itemBID, item["id"])
		}
		if item["article_id"] != articleY {
			t.Errorf("expected article swapped to Y (%s), got %v", articleY, item["article_id"])
		}
	})

	t.Run("booking B has a swap event", func(t *testing.T) {
		resp, err := leaderB.Get("/api/v0/bookings/" + bookingB + "/events")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var events []map[string]any
		json.NewDecoder(resp.Body).Decode(&events)
		found := false
		for _, e := range events {
			if e["event_type"] == "swap" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected a swap event on booking B, got events: %v", events)
		}
	})
}

// TestResolveOverdueSwaps_NightlyJob exercises ResolveOverdueSwaps directly
// (the nightly-job entry point), mirroring how other background-job logic
// (e.g. ArchiveExpiredBookings) is called straight against env.Queries in
// tests rather than through an HTTP endpoint. It covers the "no manager ever
// marked it delayed" path: booking A's item just becomes overdue (end_date in
// the past, no return_status recorded), and the nightly pass alone should
// detect and resolve the block on booking B.
func TestResolveOverdueSwaps_NightlyJob(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountReturnRoutes(env)

	leaderB := env.ClientAs("leader-flaskpost")

	bookingA, _, articleIDs := setupReturnEnv(t, env, 2, 1)
	articleX := articleIDs[0]
	articleY := articleIDs[1]

	// Force booking A's end_date into the past so FindDelayedOrOverdueItems
	// picks it up as overdue (no explicit "delayed" return status needed).
	_, err := env.Pool.Exec(context.Background(),
		"UPDATE bookings SET start_date = CURRENT_DATE - INTERVAL '10 day', end_date = CURRENT_DATE - INTERVAL '1 day' WHERE id = $1", bookingA)
	if err != nil {
		t.Fatalf("failed to backdate booking A: %v", err)
	}

	// Booking A is now entirely in the past (backdated above), so article X is
	// fully free for booking B even though B's window starts today - and
	// because B's start_date has arrived (<= the nightly job's check date of
	// "now"), the nightly pass will consider it actively blocked once X is
	// found overdue.
	teamID := getTeamID(t, leaderB, "Flaskpostorné")
	now := time.Now()
	startB := now.Format("2006-01-02")
	endB := now.AddDate(0, 0, 5).Format("2006-01-02")
	b, _ := json.Marshal(map[string]any{"start_date": startB, "end_date": endB, "used_by_team_id": teamID, "title": "Booking B"})
	resp, _ := leaderB.Post("/api/v0/bookings", bytes.NewReader(b))
	var bookingBResp map[string]any
	json.NewDecoder(resp.Body).Decode(&bookingBResp)
	resp.Body.Close()
	bookingB := bookingBResp["id"].(string)

	b, _ = json.Marshal(map[string]any{"commercial_name": "ReturnTest", "quantity": 1})
	resp, _ = leaderB.Post("/api/v0/bookings/"+bookingB+"/items", bytes.NewReader(b))
	resp.Body.Close()

	resp, _ = leaderB.Get("/api/v0/bookings/" + bookingB)
	var detailBefore map[string]any
	json.NewDecoder(resp.Body).Decode(&detailBefore)
	resp.Body.Close()
	itemBefore := detailBefore["items"].([]any)[0].(map[string]any)
	if itemBefore["article_id"] != articleX {
		t.Fatalf("expected booking B assigned article X (%s), got %v", articleX, itemBefore["article_id"])
	}

	swapped, err := handler.ResolveOverdueSwaps(context.Background(), env.Queries)
	if err != nil {
		t.Fatalf("ResolveOverdueSwaps failed: %v", err)
	}
	if swapped != 1 {
		t.Errorf("expected 1 swap performed by nightly job, got %d", swapped)
	}

	resp, _ = leaderB.Get("/api/v0/bookings/" + bookingB)
	defer resp.Body.Close()
	var detailAfter map[string]any
	json.NewDecoder(resp.Body).Decode(&detailAfter)
	itemAfter := detailAfter["items"].([]any)[0].(map[string]any)
	if itemAfter["article_id"] != articleY {
		t.Errorf("expected article swapped to Y (%s) after nightly job, got %v", articleY, itemAfter["article_id"])
	}
}

// TestReturnFlow_ReportedUnusableTriggersSwap exercises decision 6's
// "genuinely unbookable" condition-change path: marking booking A's item
// reported_unusable at return time gets the same full swap treatment as a
// delayed item, since a waiting booking B holding that exact article is now
// blocked just as surely as if it never came back at all.
func TestReturnFlow_ReportedUnusableTriggersSwap(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountReturnRoutes(env)

	leaderB := env.ClientAs("leader-flaskpost")

	bookingA, itemIDs, articleIDs := setupReturnEnv(t, env, 2, 1)
	articleX := articleIDs[0]
	articleY := articleIDs[1]

	// Backdate booking A's window so it's already over (but A itself is still
	// picked_up/unresolved) before booking B's window begins - otherwise B
	// couldn't have been assigned X in the first place (still held by A for
	// the overlap). Booking B's own window must still have started by today,
	// since check_date for a condition-change trigger is always "today" (no
	// manager-entered estimate like the delayed case has).
	_, err := env.Pool.Exec(context.Background(),
		"UPDATE bookings SET start_date = CURRENT_DATE - INTERVAL '10 day', end_date = CURRENT_DATE - INTERVAL '3 day' WHERE id = $1", bookingA)
	if err != nil {
		t.Fatalf("failed to backdate booking A: %v", err)
	}

	teamID := getTeamID(t, leaderB, "Flaskpostorné")
	now := time.Now()
	startB := now.AddDate(0, 0, -2).Format("2006-01-02")
	endB := now.AddDate(0, 0, 3).Format("2006-01-02")
	b, _ := json.Marshal(map[string]any{"start_date": startB, "end_date": endB, "used_by_team_id": teamID, "title": "Booking B"})
	resp, _ := leaderB.Post("/api/v0/bookings", bytes.NewReader(b))
	var bookingBResp map[string]any
	json.NewDecoder(resp.Body).Decode(&bookingBResp)
	resp.Body.Close()
	bookingB := bookingBResp["id"].(string)

	b, _ = json.Marshal(map[string]any{"commercial_name": "ReturnTest", "quantity": 1})
	resp, _ = leaderB.Post("/api/v0/bookings/"+bookingB+"/items", bytes.NewReader(b))
	resp.Body.Close()

	resp, _ = leaderB.Get("/api/v0/bookings/" + bookingB)
	var detailBefore map[string]any
	json.NewDecoder(resp.Body).Decode(&detailBefore)
	resp.Body.Close()
	itemBefore := detailBefore["items"].([]any)[0].(map[string]any)
	if itemBefore["article_id"] != articleX {
		t.Fatalf("expected booking B assigned article X (%s), got %v", articleX, itemBefore["article_id"])
	}

	leaderA := env.ClientAs("leader-yggdrasil")
	b, _ = json.Marshal(map[string]any{"return_status": "reported_unusable"})
	resp, _ = leaderA.Put("/api/v0/bookings/"+bookingA+"/items/"+itemIDs[0]+"/return", bytes.NewReader(b))
	resp.Body.Close()

	resp, _ = leaderB.Get("/api/v0/bookings/" + bookingB)
	defer resp.Body.Close()
	var detailAfter map[string]any
	json.NewDecoder(resp.Body).Decode(&detailAfter)
	itemAfter := detailAfter["items"].([]any)[0].(map[string]any)
	if itemAfter["article_id"] != articleY {
		t.Errorf("expected article swapped to Y (%s), got %v", articleY, itemAfter["article_id"])
	}
}

// TestReturnFlow_ReportedUsableUpgradeOnly exercises decision 6's opportunistic
// upgrade: a damaged-but-still-bookable ('reported_usable') return must only
// swap the waiting booking onto a strictly-'ok' unit, never onto another
// reported_usable one - even if that other unit would otherwise be found first.
func TestReturnFlow_ReportedUsableUpgradeOnly(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountReturnRoutes(env)

	leaderB := env.ClientAs("leader-flaskpost")

	// 3 identical articles: X assigned to booking A, Y already reported_usable
	// (created before Z, so an unrestricted search would find it first), Z
	// fully 'ok' - the only valid upgrade target.
	bookingA, itemIDs, articleIDs := setupReturnEnv(t, env, 3, 1)
	articleX := articleIDs[0]
	articleY := articleIDs[1]
	articleZ := articleIDs[2]

	_, err := env.Pool.Exec(context.Background(),
		"UPDATE articles SET status = 'reported_usable' WHERE id = $1", articleY)
	if err != nil {
		t.Fatalf("failed to mark article Y reported_usable: %v", err)
	}

	// Backdate booking A's window so it's already over before booking B's
	// window begins - otherwise B couldn't have been assigned X in the first
	// place (still held by A for the overlap).
	_, err = env.Pool.Exec(context.Background(),
		"UPDATE bookings SET start_date = CURRENT_DATE - INTERVAL '10 day', end_date = CURRENT_DATE - INTERVAL '3 day' WHERE id = $1", bookingA)
	if err != nil {
		t.Fatalf("failed to backdate booking A: %v", err)
	}

	teamID := getTeamID(t, leaderB, "Flaskpostorné")
	now := time.Now()
	startB := now.AddDate(0, 0, -2).Format("2006-01-02")
	endB := now.AddDate(0, 0, 3).Format("2006-01-02")
	b, _ := json.Marshal(map[string]any{"start_date": startB, "end_date": endB, "used_by_team_id": teamID, "title": "Booking B"})
	resp, _ := leaderB.Post("/api/v0/bookings", bytes.NewReader(b))
	var bookingBResp map[string]any
	json.NewDecoder(resp.Body).Decode(&bookingBResp)
	resp.Body.Close()
	bookingB := bookingBResp["id"].(string)

	b, _ = json.Marshal(map[string]any{"commercial_name": "ReturnTest", "quantity": 1})
	resp, _ = leaderB.Post("/api/v0/bookings/"+bookingB+"/items", bytes.NewReader(b))
	resp.Body.Close()

	resp, _ = leaderB.Get("/api/v0/bookings/" + bookingB)
	var detailBefore map[string]any
	json.NewDecoder(resp.Body).Decode(&detailBefore)
	resp.Body.Close()
	itemBefore := detailBefore["items"].([]any)[0].(map[string]any)
	if itemBefore["article_id"] != articleX {
		t.Fatalf("expected booking B assigned article X (%s), got %v", articleX, itemBefore["article_id"])
	}

	leaderA := env.ClientAs("leader-yggdrasil")
	b, _ = json.Marshal(map[string]any{"return_status": "reported_usable"})
	resp, _ = leaderA.Put("/api/v0/bookings/"+bookingA+"/items/"+itemIDs[0]+"/return", bytes.NewReader(b))
	resp.Body.Close()

	resp, _ = leaderB.Get("/api/v0/bookings/" + bookingB)
	defer resp.Body.Close()
	var detailAfter map[string]any
	json.NewDecoder(resp.Body).Decode(&detailAfter)
	itemAfter := detailAfter["items"].([]any)[0].(map[string]any)
	if itemAfter["article_id"] != articleZ {
		t.Errorf("expected article upgraded to strictly-ok Z (%s), got %v", articleZ, itemAfter["article_id"])
	}
}

// TestUpdateFlow_ConflictPathSwaps exercises decision 7: changing a booking's
// dates onto a range where its exact assigned unit is already held by another
// booking now silently swaps in an equivalent free unit instead of hard-409ing
// - this is also what makes item 10's copy-then-reschedule flow "just work"
// without any copy-specific code, since Copy's follow-up PATCH goes through
// this same Update handler.
func TestUpdateFlow_ConflictPathSwaps(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	mountReturnRoutes(env)

	manager := env.ClientAs("manager-equipment")
	leaderA := env.ClientAs("leader-yggdrasil")
	leaderC := env.ClientAs("leader-flaskpost")

	resp, _ := manager.Get("/api/v0/locations")
	var locations []map[string]any
	json.NewDecoder(resp.Body).Decode(&locations)
	resp.Body.Close()
	locID := locations[0]["id"].(string)
	resp, _ = manager.Get("/api/v0/categories")
	var categories []map[string]any
	json.NewDecoder(resp.Body).Decode(&categories)
	resp.Body.Close()
	catID := categories[0]["id"].(string)

	var articleIDs []string
	for i := range 2 {
		b, _ := json.Marshal(map[string]any{
			"commercial_name": "UpdateSwapTest", "common_name": "UpdateSwapTest " + string(rune('1'+i)),
			"category_id": catID, "location_id": locID, "individually_tracked": true,
		})
		resp, _ := manager.Post("/api/v0/articles", bytes.NewReader(b))
		var article map[string]any
		json.NewDecoder(resp.Body).Decode(&article)
		resp.Body.Close()
		articleIDs = append(articleIDs, article["id"].(string))
	}
	articleX := articleIDs[0]
	articleY := articleIDs[1]

	teamA := getTeamID(t, leaderA, "Yggdrasil")
	b, _ := json.Marshal(map[string]any{"start_date": "2027-03-01", "end_date": "2027-03-05", "used_by_team_id": teamA, "title": "Booking A"})
	resp, _ = leaderA.Post("/api/v0/bookings", bytes.NewReader(b))
	var bookingAResp map[string]any
	json.NewDecoder(resp.Body).Decode(&bookingAResp)
	resp.Body.Close()
	bookingA := bookingAResp["id"].(string)

	b, _ = json.Marshal(map[string]any{"commercial_name": "UpdateSwapTest", "quantity": 1})
	resp, _ = leaderA.Post("/api/v0/bookings/"+bookingA+"/items", bytes.NewReader(b))
	resp.Body.Close()
	resp, _ = leaderA.Post("/api/v0/bookings/"+bookingA+"/submit", nil)
	resp.Body.Close()

	resp, _ = leaderA.Get("/api/v0/bookings/" + bookingA)
	var detailA map[string]any
	json.NewDecoder(resp.Body).Decode(&detailA)
	resp.Body.Close()
	itemA := detailA["items"].([]any)[0].(map[string]any)
	if itemA["article_id"] != articleX {
		t.Fatalf("expected booking A assigned article X (%s), got %v", articleX, itemA["article_id"])
	}

	// Booking C: a separate, later window that overlaps where we're about to
	// move booking A's dates to. Both X and Y are free for these dates, so it
	// also deterministically gets assigned X.
	teamC := getTeamID(t, leaderC, "Flaskpostorné")
	b, _ = json.Marshal(map[string]any{"start_date": "2027-03-10", "end_date": "2027-03-15", "used_by_team_id": teamC, "title": "Booking C"})
	resp, _ = leaderC.Post("/api/v0/bookings", bytes.NewReader(b))
	var bookingCResp map[string]any
	json.NewDecoder(resp.Body).Decode(&bookingCResp)
	resp.Body.Close()
	bookingC := bookingCResp["id"].(string)

	b, _ = json.Marshal(map[string]any{"commercial_name": "UpdateSwapTest", "quantity": 1})
	resp, _ = leaderC.Post("/api/v0/bookings/"+bookingC+"/items", bytes.NewReader(b))
	resp.Body.Close()
	resp, _ = leaderC.Post("/api/v0/bookings/"+bookingC+"/submit", nil)
	resp.Body.Close()

	// Move booking A's dates to overlap booking C's window. Its exact unit
	// (X) is now held by C, but Y is free - expect a silent swap, not a 409.
	b, _ = json.Marshal(map[string]any{"start_date": "2027-03-12", "end_date": "2027-03-13", "title": "Booking A - rescheduled"})
	resp, err := leaderA.Put("/api/v0/bookings/"+bookingA, bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 (silent swap, not a conflict), got %d: %s", resp.StatusCode, body)
	}

	resp, _ = leaderA.Get("/api/v0/bookings/" + bookingA)
	defer resp.Body.Close()
	var detailAfter map[string]any
	json.NewDecoder(resp.Body).Decode(&detailAfter)
	itemAfter := detailAfter["items"].([]any)[0].(map[string]any)
	if itemAfter["article_id"] != articleY {
		t.Errorf("expected article swapped to Y (%s), got %v", articleY, itemAfter["article_id"])
	}

	resp, err = leaderA.Get("/api/v0/bookings/" + bookingA + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var events []map[string]any
	json.NewDecoder(resp.Body).Decode(&events)
	found := false
	for _, e := range events {
		if e["event_type"] == "swap" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a swap event on booking A, got events: %v", events)
	}
}
