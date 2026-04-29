package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/ms-utrustning/api/internal/auth"
	"github.com/malarscouterna/ms-utrustning/api/internal/db"
	"github.com/malarscouterna/ms-utrustning/api/internal/notifications"
)

// activeNotificationChannels lists the channels active for all groups in phase 3.
var activeNotificationChannels = []string{"email"}

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

// GET /me/notification-prefs — returns merged effective prefs with source info.
func (h *NotificationPrefsHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	prefs, err := notifications.ResolvePrefs(r.Context(), h.Q, claims.MemberID, claims.GroupID, activeNotificationChannels, claims.IsManager())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to load preferences")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"prefs": prefs})
}

// PUT /me/notification-prefs — partial update; only keys present are changed.
// Redundant overrides (value matches group/system default) are pruned so that
// source correctly reverts to "group_default" or "system_default".
func (h *NotificationPrefsHandler) PutMe(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	var incoming map[string]map[string]bool
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
			existing[event][ch] = enabled
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
// Returns effective defaults per role (user/manager), merged with system defaults.
func (h *NotificationPrefsHandler) GetGroupDefaults(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	raw, _ := h.Q.GetGroupNotificationDefaults(r.Context(), claims.GroupID)
	groupDefaults := notifications.ParseGroupDefaults(raw)

	mergeRole := func(isManager bool) map[string]map[string]bool {
		sys := notifications.SystemDefaults(isManager)
		out := make(map[string]map[string]bool, len(notifications.AllEvents))
		for _, event := range notifications.AllEvents {
			out[event] = make(map[string]bool)
			for ch, sv := range sys[event] {
				if gv, ok := groupDefaults.Lookup(event, ch, isManager); ok {
					out[event][ch] = gv
				} else {
					out[event][ch] = sv
				}
			}
		}
		return out
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"user":                   mergeRole(false),
		"manager":                mergeRole(true),
		"system_defaults_user":   notifications.SystemDefaults(false),
		"system_defaults_manager": notifications.SystemDefaults(true),
	})
}

// PUT /group-settings/notification-defaults — manager only, full replacement.
// Body: { "user": {event: {ch: bool}}, "manager": {event: {ch: bool}} }
func (h *NotificationPrefsHandler) PutGroupDefaults(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	var incoming notifications.GroupNotificationDefaults
	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	encoded, err := json.Marshal(incoming)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to encode defaults")
		return
	}
	if err := h.Q.SetGroupNotificationDefaults(r.Context(), db.SetGroupNotificationDefaultsParams{
		GroupID: claims.GroupID, NotificationDefaults: encoded,
	}); err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to save defaults")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
