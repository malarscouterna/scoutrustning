package notifications

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
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
	SourceTeamDefault   PrefSource = "team_default"
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

// GroupNotificationDefaults is the parsed form of group_settings.notification_defaults.
// Shape: { "user": { event: { channel: bool } }, "manager": { event: { channel: bool } } }
type GroupNotificationDefaults struct {
	User    map[string]map[string]bool `json:"user"`
	Manager map[string]map[string]bool `json:"manager"`
}

func ParseGroupDefaults(raw []byte) GroupNotificationDefaults {
	var d GroupNotificationDefaults
	json.Unmarshal(raw, &d) //nolint:errcheck — missing keys are nil maps, treated as no override
	return d
}

func (d GroupNotificationDefaults) Lookup(event, channel string, isManager bool) (bool, bool) {
	m := d.User
	if isManager {
		m = d.Manager
	}
	if ev, ok := m[event]; ok {
		if v, ok := ev[channel]; ok {
			return v, true
		}
	}
	return false, false
}

// parseSimplePrefs parses a flat JSONB pref map: { event: { channel: bool } }.
// Used for both user prefs and team notification_prefs.
func parseSimplePrefs(raw []byte) map[string]map[string]bool {
	var m map[string]map[string]bool
	json.Unmarshal(raw, &m) //nolint:errcheck — empty/null treated as nil map
	return m
}

// lookupSimplePrefs returns (value, found) from a flat pref map.
func lookupSimplePrefs(prefs map[string]map[string]bool, event, channel string) (bool, bool) {
	if ev, ok := prefs[event]; ok {
		if v, ok := ev[channel]; ok {
			return v, true
		}
	}
	return false, false
}

// TeamNotifSettings holds the notification-relevant fields fetched from a team row.
type TeamNotifSettings struct {
	IndividualEnabled bool
	Prefs             map[string]map[string]bool
}

// GetTeamNotifSettings loads notification settings for a team. Returns zero value if teamID is empty.
func GetTeamNotifSettings(ctx context.Context, q *db.Queries, groupID, teamID string) TeamNotifSettings {
	if teamID == "" {
		return TeamNotifSettings{IndividualEnabled: true}
	}
	row, err := q.GetTeamNotificationSettings(ctx, db.GetTeamNotificationSettingsParams{
		ID: mustParseUUID(teamID), GroupID: groupID,
	})
	if err != nil {
		return TeamNotifSettings{IndividualEnabled: true}
	}
	return TeamNotifSettings{
		IndividualEnabled: row.IndividualNotificationsEnabled,
		Prefs:             parseSimplePrefs(row.NotificationPrefs),
	}
}

// ResolvePrefs returns the merged effective preferences for a user across all
// known events and the given channels. Resolution order: user → team → group → system.
// Pass teamID="" when no team context exists.
func ResolvePrefs(ctx context.Context, q *db.Queries, userID, groupID, teamID string, channels []string, isManager bool) (ResolvedPrefs, error) {
	sysDefaults := SystemDefaults(isManager)

	userPrefsRaw, _ := q.GetUserNotificationPrefs(ctx, db.GetUserNotificationPrefsParams{
		ID: userID, GroupID: groupID,
	})
	userPrefs := parseSimplePrefs(userPrefsRaw)

	team := GetTeamNotifSettings(ctx, q, groupID, teamID)

	groupDefaultsRaw, _ := q.GetGroupNotificationDefaults(ctx, groupID)
	groupDefaults := ParseGroupDefaults(groupDefaultsRaw)

	result := make(ResolvedPrefs, len(AllEvents))
	for _, event := range AllEvents {
		result[event] = make(map[string]ChannelPref, len(channels))
		for _, ch := range channels {
			// Compute the non-user default (team → group → system).
			defaultEnabled := sysDefaults[event][ch]
			if gv, ok := groupDefaults.Lookup(event, ch, isManager); ok {
				defaultEnabled = gv
			}
			if tv, ok := lookupSimplePrefs(team.Prefs, event, ch); ok {
				defaultEnabled = tv
			}

			// Explicit user override.
			if up, ok := lookupSimplePrefs(userPrefs, event, ch); ok {
				result[event][ch] = ChannelPref{Enabled: up, Source: SourceUser, DefaultEnabled: defaultEnabled}
				continue
			}
			// Team suppresses individual by default (and user has no explicit override).
			if !team.IndividualEnabled {
				result[event][ch] = ChannelPref{Enabled: false, Source: SourceTeamDefault, DefaultEnabled: defaultEnabled}
				continue
			}
			if tv, ok := lookupSimplePrefs(team.Prefs, event, ch); ok {
				result[event][ch] = ChannelPref{Enabled: tv, Source: SourceTeamDefault, DefaultEnabled: defaultEnabled}
				continue
			}
			if gv, ok := groupDefaults.Lookup(event, ch, isManager); ok {
				result[event][ch] = ChannelPref{Enabled: gv, Source: SourceGroupDefault, DefaultEnabled: defaultEnabled}
				continue
			}
			result[event][ch] = ChannelPref{Enabled: sysDefaults[event][ch], Source: SourceSystemDefault, DefaultEnabled: defaultEnabled}
		}
	}
	return result, nil
}

// IsEnabled returns the effective enabled value for a single (event, channel) pair.
// Fast path used by send functions. Pass teamID="" when no team context exists.
func IsEnabled(ctx context.Context, q *db.Queries, userID, groupID, teamID, event, channel string, isManager bool) bool {
	userPrefsRaw, _ := q.GetUserNotificationPrefs(ctx, db.GetUserNotificationPrefsParams{
		ID: userID, GroupID: groupID,
	})
	userPrefs := parseSimplePrefs(userPrefsRaw)
	if up, ok := lookupSimplePrefs(userPrefs, event, channel); ok {
		return up
	}

	team := GetTeamNotifSettings(ctx, q, groupID, teamID)
	if !team.IndividualEnabled {
		return false
	}
	if tv, ok := lookupSimplePrefs(team.Prefs, event, channel); ok {
		return tv
	}

	groupDefaultsRaw, _ := q.GetGroupNotificationDefaults(ctx, groupID)
	groupDefaults := ParseGroupDefaults(groupDefaultsRaw)
	if gv, ok := groupDefaults.Lookup(event, channel, isManager); ok {
		return gv
	}

	sd := SystemDefaults(isManager)
	if ev, ok := sd[event]; ok {
		return ev[channel]
	}
	return false
}

// mustParseUUID parses a UUID string; returns zero UUID on error.
func mustParseUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	_ = u.Scan(s)
	return u
}

// teamIDStr converts a nullable UUID (e.g. booking.UsedByTeamID) to a string
// suitable for passing as teamID to IsEnabled/ResolvePrefs. Returns "" if unset.
func teamIDStr(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", u.Bytes[0:4], u.Bytes[4:6], u.Bytes[6:8], u.Bytes[8:10], u.Bytes[10:])
}
