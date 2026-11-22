package handler

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/malarscouterna/scoutrustning/api/internal/db"
	"github.com/malarscouterna/scoutrustning/api/internal/notifications"
)

func items(types ...string) []db.ListBookingItemsRow {
	out := make([]db.ListBookingItemsRow, 0, len(types))
	for _, t := range types {
		out = append(out, db.ListBookingItemsRow{CommercialName: t, LocationName: "Förrådet"})
	}
	return out
}

func TestDiffBookingItems(t *testing.T) {
	t.Run("groups item types and reports additions and removals", func(t *testing.T) {
		previous, _ := json.Marshal(bookingItemsSnapshotFor(items("Tält", "Tält", "Stormkök")))
		changes := diffBookingItems(previous, items("Tält", "Stormkök", "Stormkök", "Stormkök", "Liggunderlag"))

		want := notifications.BookingItemChanges{
			Added: []notifications.ItemChange{
				{CommercialName: "Liggunderlag", LocationName: "Förrådet", Count: 1},
				{CommercialName: "Stormkök", LocationName: "Förrådet", Count: 2},
			},
			Removed: []notifications.ItemChange{
				{CommercialName: "Tält", LocationName: "Förrådet", Count: 1},
			},
		}
		if !reflect.DeepEqual(changes, want) {
			t.Fatalf("unexpected changes:\n got %+v\nwant %+v", changes, want)
		}
	})

	t.Run("unchanged items produce an empty diff", func(t *testing.T) {
		previous, _ := json.Marshal(bookingItemsSnapshotFor(items("Tält", "Stormkök")))
		if changes := diffBookingItems(previous, items("Stormkök", "Tält")); !changes.IsEmpty() {
			t.Fatalf("expected empty diff, got %+v", changes)
		}
	})

	t.Run("legacy submitted events without a snapshot are ignored", func(t *testing.T) {
		if changes := diffBookingItems(json.RawMessage(`{}`), items("Tält")); !changes.IsEmpty() {
			t.Fatalf("expected empty diff, got %+v", changes)
		}
	})

	t.Run("same type in different locations is separate", func(t *testing.T) {
		previous, _ := json.Marshal(bookingItemsSnapshot{
			Items: []bookingItemSnapshot{{CommercialName: "Tält", LocationName: "Förrådet", Count: 1}},
		})
		changes := diffBookingItems(previous, []db.ListBookingItemsRow{
			{CommercialName: "Tält", LocationName: "Förrådet"},
			{CommercialName: "Tält", LocationName: "Källaren"},
		})
		if len(changes.Added) != 1 || changes.Added[0].LocationName != "Källaren" {
			t.Fatalf("expected one added item in Källaren, got %+v", changes)
		}
	})
}
