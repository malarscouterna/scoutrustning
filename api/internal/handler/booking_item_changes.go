package handler

import (
	"encoding/json"
	"sort"

	"github.com/malarscouterna/scoutrustning/api/internal/db"
	"github.com/malarscouterna/scoutrustning/api/internal/notifications"
)

// bookingItemsSnapshot is stored in the metadata of each submitted booking
// event. The next submission compares against it to build the item diff.
type bookingItemsSnapshot struct {
	Items []bookingItemSnapshot `json:"items"`
}

type bookingItemSnapshot struct {
	CommercialName string `json:"commercial_name"`
	LocationName   string `json:"location_name"`
	Count          int    `json:"count"`
}

type bookingItemType struct {
	commercialName string
	locationName   string
}

func bookingItemCounts(items []db.ListBookingItemsRow) map[bookingItemType]int {
	counts := make(map[bookingItemType]int)
	for _, item := range items {
		counts[bookingItemType{commercialName: item.CommercialName, locationName: item.LocationName}]++
	}
	return counts
}

func bookingItemsSnapshotFor(items []db.ListBookingItemsRow) bookingItemsSnapshot {
	counts := bookingItemCounts(items)
	snapshot := bookingItemsSnapshot{Items: make([]bookingItemSnapshot, 0, len(counts))}
	for key, count := range counts {
		snapshot.Items = append(snapshot.Items, bookingItemSnapshot{
			CommercialName: key.commercialName,
			LocationName:   key.locationName,
			Count:          count,
		})
	}
	sortBookingItemSnapshots(snapshot.Items)
	return snapshot
}

// diffBookingItems returns the net item-type changes since the previous
// submission. It returns an empty diff when the previous event has no snapshot
// (for example, bookings submitted before this feature existed).
func diffBookingItems(previousMetadata json.RawMessage, current []db.ListBookingItemsRow) notifications.BookingItemChanges {
	var previous bookingItemsSnapshot
	if err := json.Unmarshal(previousMetadata, &previous); err != nil || len(previous.Items) == 0 {
		return notifications.BookingItemChanges{}
	}

	previousCounts := make(map[bookingItemType]int, len(previous.Items))
	for _, item := range previous.Items {
		previousCounts[bookingItemType{commercialName: item.CommercialName, locationName: item.LocationName}] = item.Count
	}
	currentCounts := bookingItemCounts(current)

	keys := make(map[bookingItemType]struct{}, len(previousCounts)+len(currentCounts))
	for key := range previousCounts {
		keys[key] = struct{}{}
	}
	for key := range currentCounts {
		keys[key] = struct{}{}
	}
	sortedKeys := make([]bookingItemType, 0, len(keys))
	for key := range keys {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Slice(sortedKeys, func(i, j int) bool {
		if sortedKeys[i].commercialName != sortedKeys[j].commercialName {
			return sortedKeys[i].commercialName < sortedKeys[j].commercialName
		}
		return sortedKeys[i].locationName < sortedKeys[j].locationName
	})

	var changes notifications.BookingItemChanges
	for _, key := range sortedKeys {
		delta := currentCounts[key] - previousCounts[key]
		change := notifications.ItemChange{
			CommercialName: key.commercialName,
			LocationName:   key.locationName,
			Count:          abs(delta),
		}
		switch {
		case delta > 0:
			changes.Added = append(changes.Added, change)
		case delta < 0:
			changes.Removed = append(changes.Removed, change)
		}
	}
	return changes
}

func sortBookingItemSnapshots(items []bookingItemSnapshot) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].CommercialName != items[j].CommercialName {
			return items[i].CommercialName < items[j].CommercialName
		}
		return items[i].LocationName < items[j].LocationName
	})
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
