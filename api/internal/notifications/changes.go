package notifications

// ItemChange is one item type (commercial name + location) and how many of it
// were added or removed since the previous submission.
type ItemChange struct {
	CommercialName string
	LocationName   string
	Count          int
}

// BookingItemChanges is the add/remove diff between two booking submissions.
type BookingItemChanges struct {
	Added   []ItemChange
	Removed []ItemChange
}

// IsEmpty reports whether there is nothing to notify about.
func (c BookingItemChanges) IsEmpty() bool {
	return len(c.Added) == 0 && len(c.Removed) == 0
}
