package notifications

import (
	"context"
	"encoding/json"

	"github.com/malarscouterna/ms-utrustning/api/internal/db"
)

// EventKey identifies a notification event type.
type EventKey = string

const (
	EventBookingNeedsApproval       EventKey = "booking_needs_approval"
	EventBookingSubmittedNoApproval EventKey = "booking_submitted_no_approval"
	EventBookingConfirmed           EventKey = "booking_confirmed"
	EventBookingRejected            EventKey = "booking_rejected"
	EventBookingCancelled           EventKey = "booking_cancelled"
	EventBookingReminder            EventKey = "booking_reminder"
	EventBookingOverdue             EventKey = "booking_overdue"
	EventBookingAnyCreated          EventKey = "booking_any_created"
	EventIssueCreated               EventKey = "issue_created"
	EventIssueAssignedToMe          EventKey = "issue_assigned_to_me"
	EventIssueResolved              EventKey = "issue_resolved"
	EventIssueCommented             EventKey = "issue_commented"
)

// AllEvents is the ordered list of all event keys, used for building responses.
var AllEvents = []EventKey{
	EventBookingNeedsApproval,
	EventBookingSubmittedNoApproval,
	EventBookingConfirmed,
	EventBookingRejected,
	EventBookingCancelled,
	EventBookingReminder,
	EventBookingOverdue,
	EventBookingAnyCreated,
	EventIssueCreated,
	EventIssueAssignedToMe,
	EventIssueResolved,
	EventIssueCommented,
}

// SystemDefaults returns the hardcoded default for (eventKey, channel, isManager).
func SystemDefaults(isManager bool) map[EventKey]map[string]bool {
	d := map[EventKey]map[string]bool{
		EventBookingNeedsApproval:       {"email": isManager},
		EventBookingSubmittedNoApproval: {"email": false},
		EventBookingConfirmed:           {"email": true},
		EventBookingRejected:            {"email": true},
		EventBookingCancelled:           {"email": true},
		EventBookingReminder:            {"email": true},
		EventBookingOverdue:             {"email": true},
		EventBookingAnyCreated:          {"email": false},
		EventIssueCreated:               {"email": isManager},
		EventIssueAssignedToMe:          {"email": true},
		EventIssueResolved:              {"email": true},
		EventIssueCommented:             {"email": true},
	}
	return d
}

// PrefSource describes where an effective preference value came from.
type PrefSource string

const (
	SourceUser          PrefSource = "user"
	SourceGroupDefault  PrefSource = "group_default"
	SourceSystemDefault PrefSource = "system_default"
)

// ChannelPref is the effective value for one (event, channel) pair.
// DefaultEnabled is the group/system default regardless of any user override —
// used by the UI to show a "(standard)" hint when the user's value matches the default.
type ChannelPref struct {
	Enabled        bool       `json:"enabled"`
	Source         PrefSource `json:"source"`
	DefaultEnabled bool       `json:"default_enabled"`
}

// ResolvedPrefs maps event key → channel → effective pref.
type ResolvedPrefs map[EventKey]map[string]ChannelPref

// ResolvePrefs returns the merged effective preferences for a user across all
// known events and the given channels. Resolution order: user → group → system.
func ResolvePrefs(ctx context.Context, q *db.Queries, userID, groupID string, channels []string, isManager bool) (ResolvedPrefs, error) {
	sysDefaults := SystemDefaults(isManager)

	// Load user prefs (may be empty if never set).
	userPrefsRaw, _ := q.GetUserNotificationPrefs(ctx, db.GetUserNotificationPrefsParams{
		ID: userID, GroupID: groupID,
	})
	var userPrefs map[string]map[string]bool
	json.Unmarshal(userPrefsRaw, &userPrefs) //nolint:errcheck — empty/null is fine as nil map

	// Load group defaults (may be empty).
	groupDefaultsRaw, _ := q.GetGroupNotificationDefaults(ctx, groupID)
	var groupDefaults map[string]map[string]bool
	json.Unmarshal(groupDefaultsRaw, &groupDefaults) //nolint:errcheck

	result := make(ResolvedPrefs, len(AllEvents))
	for _, event := range AllEvents {
		result[event] = make(map[string]ChannelPref, len(channels))
		for _, ch := range channels {
			// Compute the non-user default (group → system).
			defaultEnabled := sysDefaults[event][ch]
			if gp, ok := groupDefaults[event][ch]; ok {
				defaultEnabled = gp
			}

			if up, ok := userPrefs[event][ch]; ok {
				result[event][ch] = ChannelPref{Enabled: up, Source: SourceUser, DefaultEnabled: defaultEnabled}
			} else if gp, ok := groupDefaults[event][ch]; ok {
				result[event][ch] = ChannelPref{Enabled: gp, Source: SourceGroupDefault, DefaultEnabled: defaultEnabled}
			} else {
				result[event][ch] = ChannelPref{Enabled: defaultEnabled, Source: SourceSystemDefault, DefaultEnabled: defaultEnabled}
			}
		}
	}
	return result, nil
}

// IsEnabled returns the effective enabled value for a single (event, channel) pair.
// Fast path used by send functions — avoids building the full ResolvedPrefs map.
func IsEnabled(ctx context.Context, q *db.Queries, userID, groupID, event, channel string, isManager bool) bool {
	userPrefsRaw, _ := q.GetUserNotificationPrefs(ctx, db.GetUserNotificationPrefsParams{
		ID: userID, GroupID: groupID,
	})
	var userPrefs map[string]map[string]bool
	json.Unmarshal(userPrefsRaw, &userPrefs) //nolint:errcheck
	if up, ok := userPrefs[event]; ok {
		if v, ok := up[channel]; ok {
			return v
		}
	}

	groupDefaultsRaw, _ := q.GetGroupNotificationDefaults(ctx, groupID)
	var groupDefaults map[string]map[string]bool
	json.Unmarshal(groupDefaultsRaw, &groupDefaults) //nolint:errcheck
	if gp, ok := groupDefaults[event]; ok {
		if v, ok := gp[channel]; ok {
			return v
		}
	}

	sd := SystemDefaults(isManager)
	if ev, ok := sd[event]; ok {
		return ev[channel]
	}
	return false
}
