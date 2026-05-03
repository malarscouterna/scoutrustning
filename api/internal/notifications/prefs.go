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

// PersonalEmailPolicy controls whether personal email is sent for an event.
type PersonalEmailPolicy = string

const (
	// PolicyAlways — always send personal email regardless of Gruppkanal.
	PolicyAlways PersonalEmailPolicy = "always"
	// PolicyIfNoBroadcast — send personal email only if team's Gruppkanal is empty.
	PolicyIfNoBroadcast PersonalEmailPolicy = "if_no_broadcast"
	// PolicyNever — never send personal email.
	PolicyNever PersonalEmailPolicy = "never"
)

// PerEventPrefs is the JSONB shape for one event in any notification_prefs column.
// Used in teams.notification_prefs, group_settings.notification_defaults,
// and users.notification_prefs. Users only store PersonalEmailPolicy; Gruppkanal
// is only meaningful in team and group prefs.
type PerEventPrefs struct {
	Gruppkanal          *bool               `json:"gruppkanal,omitempty"`
	PersonalEmailPolicy PersonalEmailPolicy `json:"personal_email_policy,omitempty"`
}

// NotificationPrefs is the parsed form of any notification_prefs JSONB column.
type NotificationPrefs map[EventKey]PerEventPrefs

func ParseNotificationPrefs(raw []byte) NotificationPrefs {
	var p NotificationPrefs
	json.Unmarshal(raw, &p) //nolint:errcheck — missing keys treated as zero value
	return p
}

func (p NotificationPrefs) GruppkanalEnabled(event EventKey) (bool, bool) {
	if ev, ok := p[event]; ok && ev.Gruppkanal != nil {
		return *ev.Gruppkanal, true
	}
	return false, false
}

func (p NotificationPrefs) Policy(event EventKey) (PersonalEmailPolicy, bool) {
	if ev, ok := p[event]; ok && ev.PersonalEmailPolicy != "" {
		return ev.PersonalEmailPolicy, true
	}
	return "", false
}

// BroadcastSystemDefaults returns the hardcoded system default for Gruppkanal delivery.
// Most team/role events default to on. Personal events have no Gruppkanal.
func BroadcastSystemDefaults() NotificationPrefs {
	on := func() *bool { b := true; return &b }()
	off := func() *bool { b := false; return &b }()
	return NotificationPrefs{
		EventBookingNeedsApproval:       {Gruppkanal: on, PersonalEmailPolicy: PolicyIfNoBroadcast},
		EventBookingSubmittedNoApproval: {Gruppkanal: on, PersonalEmailPolicy: PolicyIfNoBroadcast},
		EventBookingConfirmed:           {Gruppkanal: on, PersonalEmailPolicy: PolicyIfNoBroadcast},
		EventBookingCancelled:           {Gruppkanal: on, PersonalEmailPolicy: PolicyIfNoBroadcast},
		EventBookingReminder:            {Gruppkanal: on, PersonalEmailPolicy: PolicyIfNoBroadcast},
		EventBookingOverdue:             {Gruppkanal: on, PersonalEmailPolicy: PolicyIfNoBroadcast},
		EventBookingAnyCreated:          {Gruppkanal: off, PersonalEmailPolicy: PolicyIfNoBroadcast},
		EventIssueCreated:               {Gruppkanal: on, PersonalEmailPolicy: PolicyIfNoBroadcast},
		// Personal events — no Gruppkanal, always-on by default.
		EventBookingRejected:   {PersonalEmailPolicy: PolicyAlways},
		EventIssueAssignedToMe: {PersonalEmailPolicy: PolicyAlways},
		EventIssueResolved:     {PersonalEmailPolicy: PolicyAlways},
		EventIssueCommented:    {PersonalEmailPolicy: PolicyAlways},
	}
}

// PrefSource describes where an effective preference value came from.
type PrefSource string

const (
	SourceUser          PrefSource = "user"
	SourceTeamDefault   PrefSource = "team_default"
	SourceGroupDefault  PrefSource = "group_default"
	SourceSystemDefault PrefSource = "system_default"
)

// ResolvedPref is the effective personal email policy for one event, with source info.
type ResolvedPref struct {
	Policy        PersonalEmailPolicy `json:"policy"`
	Source        PrefSource          `json:"source"`
	DefaultPolicy PersonalEmailPolicy `json:"default_policy"`
}

// ResolvedPrefs maps event key → effective personal email pref.
type ResolvedPrefs map[EventKey]ResolvedPref

// TeamNotifSettings holds the notification-relevant fields fetched from a team row.
type TeamNotifSettings struct {
	// GruppkanalChannels is the team's explicit opt-in; nil means inherit group default.
	GruppkanalChannels []string
	Prefs              NotificationPrefs
}

// EffectiveGruppkanalChannels resolves the team's opted-in channels:
// nil → inherit groupDefault, otherwise the team's explicit selection.
func EffectiveGruppkanalChannels(teamChannels []string, groupDefault []string) []string {
	if teamChannels != nil {
		return teamChannels
	}
	return groupDefault
}

// GetTeamNotifSettings loads notification settings for a team. Returns zero value if teamID is empty.
func GetTeamNotifSettings(ctx context.Context, q *db.Queries, groupID, teamID string) TeamNotifSettings {
	if teamID == "" {
		return TeamNotifSettings{}
	}
	row, err := q.GetTeamNotificationSettings(ctx, db.GetTeamNotificationSettingsParams{
		ID: mustParseUUID(teamID), GroupID: groupID,
	})
	if err != nil {
		return TeamNotifSettings{}
	}
	return TeamNotifSettings{
		GruppkanalChannels: row.GruppkanalChannels, // nil when NULL in DB
		Prefs:              ParseNotificationPrefs(row.NotificationPrefs),
	}
}

// ResolvePrefs returns the merged effective personal email preferences for a user across all
// known events. Resolution order: user → team → group → system.
// Pass teamID="" when no team context exists.
func ResolvePrefs(ctx context.Context, q *db.Queries, userID, groupID, teamID string, _ []string, isManager bool) (ResolvedPrefs, error) {
	sys := BroadcastSystemDefaults()

	userPrefsRaw, _ := q.GetUserNotificationPrefs(ctx, db.GetUserNotificationPrefsParams{
		ID: userID, GroupID: groupID,
	})
	userPrefs := ParseNotificationPrefs(userPrefsRaw)

	team := GetTeamNotifSettings(ctx, q, groupID, teamID)

	groupDefaultsRow, _ := q.GetGroupNotificationDefaults(ctx, groupID)
	groupDefaults := ParseNotificationPrefs(groupDefaultsRow.NotificationDefaults)

	result := make(ResolvedPrefs, len(AllEvents))
	for _, event := range AllEvents {
		// Compute non-user default: team → group → system.
		defaultPolicy := sys[event].PersonalEmailPolicy
		if gp, ok := groupDefaults.Policy(event); ok {
			defaultPolicy = gp
		}
		if tp, ok := team.Prefs.Policy(event); ok {
			defaultPolicy = tp
		}

		// User explicit override.
		if up, ok := userPrefs.Policy(event); ok {
			result[event] = ResolvedPref{Policy: up, Source: SourceUser, DefaultPolicy: defaultPolicy}
			continue
		}

		// No user override — use team/group/system default.
		result[event] = ResolvedPref{Policy: defaultPolicy, Source: sourceFor(team, groupDefaults, sys, event), DefaultPolicy: defaultPolicy}
	}
	return result, nil
}

func sourceFor(team TeamNotifSettings, groupDefaults, sys NotificationPrefs, event EventKey) PrefSource {
	if _, ok := team.Prefs.Policy(event); ok {
		return SourceTeamDefault
	}
	if _, ok := groupDefaults.Policy(event); ok {
		return SourceGroupDefault
	}
	if _, ok := sys.Policy(event); ok {
		return SourceSystemDefault
	}
	return SourceSystemDefault
}

// IsGruppkanalEnabled returns true if the Gruppkanal should fire for a given event,
// resolved through team → group → system defaults.
func IsGruppkanalEnabled(ctx context.Context, q *db.Queries, groupID, teamID, event string) bool {
	team := GetTeamNotifSettings(ctx, q, groupID, teamID)
	if enabled, ok := team.Prefs.GruppkanalEnabled(event); ok {
		return enabled
	}
	groupDefaultsRow, _ := q.GetGroupNotificationDefaults(ctx, groupID)
	groupDefaults := ParseNotificationPrefs(groupDefaultsRow.NotificationDefaults)
	if enabled, ok := groupDefaults.GruppkanalEnabled(event); ok {
		return enabled
	}
	sys := BroadcastSystemDefaults()
	if ev, ok := sys[event]; ok && ev.Gruppkanal != nil {
		return *ev.Gruppkanal
	}
	return false
}

// ResolvePersonalEmailPolicy returns the effective personal email policy for a user
// for a given team/role event. Pass teamID="" for personal events.
func ResolvePersonalEmailPolicy(ctx context.Context, q *db.Queries, userID, groupID, teamID, event string) PersonalEmailPolicy {
	userPrefsRaw, _ := q.GetUserNotificationPrefs(ctx, db.GetUserNotificationPrefsParams{
		ID: userID, GroupID: groupID,
	})
	userPrefs := ParseNotificationPrefs(userPrefsRaw)
	if up, ok := userPrefs.Policy(event); ok {
		return up
	}

	team := GetTeamNotifSettings(ctx, q, groupID, teamID)
	if tp, ok := team.Prefs.Policy(event); ok {
		return tp
	}

	groupDefaultsRow, _ := q.GetGroupNotificationDefaults(ctx, groupID)
	groupDefaults := ParseNotificationPrefs(groupDefaultsRow.NotificationDefaults)
	if gp, ok := groupDefaults.Policy(event); ok {
		return gp
	}

	sys := BroadcastSystemDefaults()
	if ev, ok := sys[event]; ok && ev.PersonalEmailPolicy != "" {
		return ev.PersonalEmailPolicy
	}
	return PolicyAlways
}

// mustParseUUID parses a UUID string; returns zero UUID on error.
func mustParseUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	_ = u.Scan(s)
	return u
}

// teamIDStr converts a nullable UUID (e.g. booking.UsedByTeamID) to a string
// suitable for passing as teamID. Returns "" if unset.
func teamIDStr(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", u.Bytes[0:4], u.Bytes[4:6], u.Bytes[6:8], u.Bytes[8:10], u.Bytes[10:])
}
