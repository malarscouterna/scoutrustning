package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/ms-utrustning/api/internal/auth"
	"github.com/malarscouterna/ms-utrustning/api/internal/db"
	"github.com/malarscouterna/ms-utrustning/api/internal/notifications"
)

// fallbackChannels is used when group settings cannot be loaded.
var fallbackChannels = []string{"email"}

type NotificationPrefsHandler struct {
	Q *db.Queries
}

func (h *NotificationPrefsHandler) MeRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.GetMe)
	r.Put("/", h.PutMe)
	r.Delete("/", h.DeleteMe)
	return r
}

func (h *NotificationPrefsHandler) GroupRoutes() chi.Router {
	r := chi.NewRouter()
	r.Use(auth.RequireRole("equipment_manager"))
	r.Get("/", h.GetGroupDefaults)
	r.Put("/", h.PutGroupDefaults)
	return r
}

func (h *NotificationPrefsHandler) ForceDefaultsRoute() chi.Router {
	r := chi.NewRouter()
	r.Use(auth.RequireRole("equipment_manager"))
	r.Post("/", h.ForceDefaults)
	return r
}

// GET /me/notification-prefs — returns merged effective prefs with source info.
func (h *NotificationPrefsHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	channels, err := h.Q.GetGroupEnabledChannels(r.Context(), claims.GroupID)
	if err != nil || len(channels) == 0 {
		channels = fallbackChannels
	}
	prefs, err := notifications.ResolvePrefs(r.Context(), h.Q, claims.MemberID, claims.GroupID, "", channels, claims.IsManager())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to load preferences")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"prefs": prefs})
}

// PUT /me/notification-prefs — partial update; only keys present are changed.
// A null channel value removes that channel's explicit override (reverts to
// team/group/system default — the "Följ avdelningsstandard" middle column).
// If all channels for an event are removed the event key is pruned entirely.
func (h *NotificationPrefsHandler) PutMe(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	var incoming map[string]map[string]*bool
	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Load existing prefs so we can merge (partial update).
	existing := map[string]map[string]bool{}
	raw, _ := h.Q.GetUserNotificationPrefs(r.Context(), db.GetUserNotificationPrefsParams{
		ID: claims.MemberID, GroupID: claims.GroupID,
	})
	json.Unmarshal(raw, &existing) //nolint:errcheck

	for event, channels := range incoming {
		if _, ok := existing[event]; !ok {
			existing[event] = map[string]bool{}
		}
		for ch, enabled := range channels {
			if enabled == nil {
				delete(existing[event], ch)
			} else {
				existing[event][ch] = *enabled
			}
		}
		if len(existing[event]) == 0 {
			delete(existing, event)
		}
	}

	merged, err := json.Marshal(existing)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to encode preferences")
		return
	}
	if err := h.Q.SetUserNotificationPrefs(r.Context(), db.SetUserNotificationPrefsParams{
		ID: claims.MemberID, GroupID: claims.GroupID, NotificationPrefs: merged,
	}); err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to save preferences")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /me/notification-prefs — reverts all user-level prefs to group/system defaults.
func (h *NotificationPrefsHandler) DeleteMe(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	if err := h.Q.ClearUserNotificationPrefs(r.Context(), db.ClearUserNotificationPrefsParams{
		ID: claims.MemberID, GroupID: claims.GroupID,
	}); err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to clear preferences")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /group-settings/notification-defaults — manager only.
// Returns effective defaults merged with system defaults, plus default_gruppkanal_channels.
func (h *NotificationPrefsHandler) GetGroupDefaults(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	row, _ := h.Q.GetGroupNotificationDefaults(r.Context(), claims.GroupID)
	groupDefaults := notifications.ParseNotificationPrefs(row.NotificationDefaults)

	sys := notifications.BroadcastSystemDefaults()
	out := make(notifications.NotificationPrefs, len(sys))
	for event, sd := range sys {
		ep := sd
		if gd, ok := groupDefaults[event]; ok {
			if gd.Gruppkanal != nil {
				ep.Gruppkanal = gd.Gruppkanal
			}
			if gd.PersonalEmailPolicy != "" {
				ep.PersonalEmailPolicy = gd.PersonalEmailPolicy
			}
		}
		out[event] = ep
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"defaults":                    out,
		"system_defaults":             sys,
		"default_gruppkanal_channels": row.DefaultGruppkanalChannels,
	})
}

// POST /group-settings/force-notification-defaults — manager only.
// Resets user notification_prefs to '{}' and team gruppkanal_channels to NULL,
// so all inherit the current group defaults.
func (h *NotificationPrefsHandler) ForceDefaults(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	userCount, err := h.Q.ResetAllNotificationPrefs(r.Context(), claims.GroupID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to reset user preferences")
		return
	}
	teamCount, err := h.Q.ResetAllTeamGruppkanalChannels(r.Context(), claims.GroupID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to reset team gruppkanal channels")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"reset_user_count": userCount,
		"reset_team_count": teamCount,
	})
}

// PUT /group-settings/notification-defaults — manager only, full replacement.
// Body: { "defaults": {event: {gruppkanal, personal_email_policy}}, "default_gruppkanal_channels": [...] }
func (h *NotificationPrefsHandler) PutGroupDefaults(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	var body struct {
		Defaults                  notifications.NotificationPrefs `json:"defaults"`
		DefaultGruppkanalChannels []string                        `json:"default_gruppkanal_channels"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	encoded, err := json.Marshal(body.Defaults)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to encode defaults")
		return
	}
	if body.DefaultGruppkanalChannels == nil {
		body.DefaultGruppkanalChannels = []string{}
	}
	if err := h.Q.SetGroupNotificationDefaults(r.Context(), db.SetGroupNotificationDefaultsParams{
		GroupID:                   claims.GroupID,
		NotificationDefaults:      encoded,
		DefaultGruppkanalChannels: body.DefaultGruppkanalChannels,
	}); err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to save defaults")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
