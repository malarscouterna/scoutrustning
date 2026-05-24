package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/malarscouterna/scoutrustning/api/internal/auth"
	"github.com/malarscouterna/scoutrustning/api/internal/crypto"
	"github.com/malarscouterna/scoutrustning/api/internal/db"
	"github.com/malarscouterna/scoutrustning/api/internal/notifications"
)

type TeamHandler struct {
	Q        *db.Queries
	DemoMode bool
	// AddBotFn is called to add the service account bot to a GChat space.
	// Defaults to notifications.AddBotToSpace when nil.
	AddBotFn func(ctx context.Context, saJSON []byte, adminEmail, spaceID, teamName string) error
}

func (h *TeamHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.With(auth.RequireRole("equipment_manager")).Post("/", h.Create)
	r.With(auth.RequireRole("equipment_manager")).Put("/{id}", h.Update)
	r.With(auth.RequireRole("equipment_manager")).Delete("/{id}", h.Delete)
	r.Get("/{id}/notification-settings", h.GetNotificationSettings)
	r.Put("/{id}/notification-settings", h.UpdateNotificationSettings)
	r.Put("/{id}/name", h.UpdateName)
	r.Put("/{id}/gchat-space", h.SetGChatSpace)
	r.Delete("/{id}/gchat-space", h.ClearGChatSpace)
	return r
}

// requireTeamMembership returns true if the requesting user is a member of the
// given team (via users.team_ids) OR is an equipment manager in the group.
func (h *TeamHandler) requireTeamMembership(ctx context.Context, claims auth.Claims, teamID pgtype.UUID) bool {
	if claims.IsManager() {
		return true
	}
	ok, err := h.Q.IsTeamMember(ctx, db.IsTeamMemberParams{
		UserID:  claims.MemberID,
		GroupID: claims.GroupID,
		TeamID:  teamID,
	})
	return err == nil && ok
}

// PUT /teams/{id}/name — update team name; accessible to team members and managers.
func (h *TeamHandler) UpdateName(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if !h.requireTeamMembership(r.Context(), claims, id) {
		WriteError(w, http.StatusForbidden, "you are not a member of this team")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		WriteError(w, http.StatusBadRequest, "name required")
		return
	}
	team, err := h.Q.UpdateTeamName(r.Context(), db.UpdateTeamNameParams{
		ID: id, GroupID: claims.GroupID, Name: req.Name,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to update team name")
		return
	}
	WriteJSON(w, http.StatusOK, team)
}

func (h *TeamHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	teams, err := h.Q.ListTeams(r.Context(), claims.GroupID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to list teams")
		return
	}
	WriteJSON(w, http.StatusOK, teams)
}

func (h *TeamHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	var req struct {
		Name         string `json:"name"`
		Type         string `json:"type"`
		AccessLevel  string `json:"access_level"`
		ClaimScope   string `json:"claim_scope"`
		ClaimID      string `json:"claim_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		WriteError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Type == "" {
		req.Type = "troop"
	}
	if req.Type != "troop" && req.Type != "role" {
		WriteError(w, http.StatusBadRequest, "type must be troop or role")
		return
	}
	if req.AccessLevel == "" {
		req.AccessLevel = "book"
	}
	if !validAccessLevel(req.AccessLevel) {
		WriteError(w, http.StatusBadRequest, "access_level must be view, book, trusted, or manager")
		return
	}

	team, err := h.Q.CreateTeam(r.Context(), db.CreateTeamParams{
		GroupID: claims.GroupID, Name: req.Name, Type: req.Type, AccessLevel: req.AccessLevel,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to create team")
		return
	}

	// Create claim mapping if provided
	if req.ClaimScope != "" && req.ClaimID != "" {
		if req.ClaimScope != "group" && req.ClaimScope != "troop" {
			WriteError(w, http.StatusBadRequest, "claim_scope must be group or troop")
			return
		}
		h.Q.CreateTeamClaimMapping(r.Context(), db.CreateTeamClaimMappingParams{
			GroupID: claims.GroupID, TeamID: team.ID, ClaimScope: req.ClaimScope, ClaimID: req.ClaimID,
		})
	}

	WriteJSON(w, http.StatusCreated, team)
}

func (h *TeamHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req struct {
		Name        string `json:"name"`
		Type        string `json:"type"`
		AccessLevel string `json:"access_level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	existing, err := h.Q.GetTeam(r.Context(), db.GetTeamParams{ID: id, GroupID: claims.GroupID})
	if err != nil {
		WriteError(w, http.StatusNotFound, "team not found")
		return
	}

	if req.Name == "" {
		req.Name = existing.Name
	}
	if req.Type == "" {
		req.Type = existing.Type
	}
	if req.AccessLevel == "" {
		req.AccessLevel = existing.AccessLevel
	}
	if req.Type != "troop" && req.Type != "role" {
		WriteError(w, http.StatusBadRequest, "type must be troop or role")
		return
	}
	if !validAccessLevel(req.AccessLevel) {
		WriteError(w, http.StatusBadRequest, "access_level must be view, book, trusted, or manager")
		return
	}

	// Protect last manager team from demotion
	if existing.AccessLevel == "manager" && req.AccessLevel != "manager" {
		count, err := h.Q.CountManagerTeams(r.Context(), claims.GroupID)
		if err == nil && count <= 1 {
			WriteError(w, http.StatusConflict, "cannot demote the last manager team")
			return
		}
	}

	team, err := h.Q.UpdateTeam(r.Context(), db.UpdateTeamParams{
		ID: id, GroupID: claims.GroupID, Name: req.Name, Type: req.Type, AccessLevel: req.AccessLevel,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to update team")
		return
	}
	WriteJSON(w, http.StatusOK, team)
}

func (h *TeamHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}

	count, err := h.Q.CountActiveBookingsForTeam(r.Context(), db.CountActiveBookingsForTeamParams{
		TeamID: pgtype.UUID{Bytes: id.Bytes, Valid: true}, GroupID: claims.GroupID,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to check bookings")
		return
	}
	if count > 0 {
		WriteError(w, http.StatusConflict, "team has active bookings")
		return
	}

	// Protect last manager team from deletion
	team, err := h.Q.GetTeam(r.Context(), db.GetTeamParams{ID: id, GroupID: claims.GroupID})
	if err != nil {
		WriteError(w, http.StatusNotFound, "team not found")
		return
	}
	if team.AccessLevel == "manager" {
		mgrCount, err := h.Q.CountManagerTeams(r.Context(), claims.GroupID)
		if err == nil && mgrCount <= 1 {
			WriteError(w, http.StatusConflict, "cannot delete the last manager team")
			return
		}
	}

	if err := h.Q.DeleteTeam(r.Context(), db.DeleteTeamParams{ID: id, GroupID: claims.GroupID}); err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to delete team")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validAccessLevel(level string) bool {
	return level == "view" || level == "book" || level == "trusted" || level == "manager"
}

// GET /teams/{id}/notification-settings — accessible to team members and managers.
func (h *TeamHandler) GetNotificationSettings(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if !h.requireTeamMembership(r.Context(), claims, id) {
		WriteError(w, http.StatusForbidden, "you are not a member of this team")
		return
	}
	row, err := h.Q.GetTeamNotificationSettings(r.Context(), db.GetTeamNotificationSettingsParams{
		ID: id, GroupID: claims.GroupID,
	})
	if err != nil {
		WriteError(w, http.StatusNotFound, "team not found")
		return
	}
	groupDefaultsRow, _ := h.Q.GetGroupNotificationDefaults(r.Context(), claims.GroupID)
	WriteJSON(w, http.StatusOK, map[string]any{
		"notification_email":          row.NotificationEmail.String,
		"notification_prefs":          row.NotificationPrefs,
		"gchat_space_id":              row.GchatSpaceID.String,
		"gruppkanal_channels":         row.GruppkanalChannels,
		"default_gruppkanal_channels": groupDefaultsRow.DefaultGruppkanalChannels,
	})
}

// PUT /teams/{id}/notification-settings — accessible to team members and managers.
func (h *TeamHandler) UpdateNotificationSettings(w http.ResponseWriter, r *http.Request) {
	if h.DemoMode {
		WriteError(w, http.StatusForbidden, "not_allowed_in_demo")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if !h.requireTeamMembership(r.Context(), claims, id) {
		WriteError(w, http.StatusForbidden, "you are not a member of this team")
		return
	}

	var req struct {
		NotificationEmail  *string                              `json:"notification_email"`
		NotificationPrefs  notifications.NotificationPrefs      `json:"notification_prefs"`
		GruppkanalChannels *[]string                            `json:"gruppkanal_channels"` // null = reset to inherit
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Load existing to merge partial updates.
	existing, err := h.Q.GetTeamNotificationSettings(r.Context(), db.GetTeamNotificationSettingsParams{
		ID: id, GroupID: claims.GroupID,
	})
	if err != nil {
		WriteError(w, http.StatusNotFound, "team not found")
		return
	}

	email := existing.NotificationEmail
	if req.NotificationEmail != nil {
		email = pgtype.Text{String: *req.NotificationEmail, Valid: *req.NotificationEmail != ""}
	}

	prefsJSON := existing.NotificationPrefs
	if req.NotificationPrefs != nil {
		// Validate that any Gruppkanal channels are actually available for this team.
		encoded, err := json.Marshal(req.NotificationPrefs)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to encode prefs")
			return
		}
		prefsJSON = encoded
	}

	// Resolve Gruppkanal channels: keep existing if not provided.
	gruppkanalChannels := existing.GruppkanalChannels
	if req.GruppkanalChannels != nil {
		// Validate that requested channels are available (notification_email or gchat_space_id must be set).
		requested := *req.GruppkanalChannels
		var validated []string
		for _, ch := range requested {
			switch ch {
			case "email":
				if email.Valid && email.String != "" {
					validated = append(validated, ch)
				}
			case "gchat":
				if existing.GchatSpaceID.Valid && existing.GchatSpaceID.String != "" {
					validated = append(validated, ch)
				}
			}
		}
		if validated == nil {
			validated = []string{}
		}
		gruppkanalChannels = validated
	}

	_, err = h.Q.UpdateTeamNotificationSettings(r.Context(), db.UpdateTeamNotificationSettingsParams{
		ID:                 id,
		GroupID:            claims.GroupID,
		NotificationEmail:  email,
		NotificationPrefs:  prefsJSON,
		GruppkanalChannels: gruppkanalChannels,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to update notification settings")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SetGChatSpace links a team to a Google Chat Space, adds the bot, and posts a welcome card.
// Accessible to team members and managers. Auto-add requires the stored admin account to be
// a member of the space; if not, the caller gets a clear error with manual-add instructions.
func (h *TeamHandler) SetGChatSpace(w http.ResponseWriter, r *http.Request) {
	if h.DemoMode {
		WriteError(w, http.StatusForbidden, "not_allowed_in_demo")
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if !h.requireTeamMembership(r.Context(), claims, id) {
		WriteError(w, http.StatusForbidden, "not a member of this team")
		return
	}

	var req struct {
		GchatSpaceID string `json:"gchat_space_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.GchatSpaceID == "" {
		WriteError(w, http.StatusBadRequest, "gchat_space_id required")
		return
	}

	team, err := h.Q.GetTeam(r.Context(), db.GetTeamParams{ID: id, GroupID: claims.GroupID})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to look up team")
		return
	}

	creds, err := h.Q.GetGchatCredentials(r.Context(), claims.GroupID)
	if err != nil || len(creds.GchatServiceAccountJsonEncrypted) == 0 {
		WriteError(w, http.StatusBadRequest, "gchat not configured for this group")
		return
	}
	saJSON, err := crypto.Decrypt(creds.GchatServiceAccountJsonEncrypted)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to decrypt credentials")
		return
	}

	addBot := h.AddBotFn
	if addBot == nil {
		addBot = notifications.AddBotToSpace
	}
	if err := addBot(r.Context(), saJSON, creds.GchatAdminEmail, req.GchatSpaceID, team.Name); err != nil {
		slog.Error("gchat add bot to space failed", "err", err, "space", req.GchatSpaceID)
		WriteError(w, http.StatusBadGateway, gchatAddBotError(err))
		return
	}

	if err := h.Q.SetTeamGchatSpace(r.Context(), db.SetTeamGchatSpaceParams{
		GchatSpaceID: pgtype.Text{String: req.GchatSpaceID, Valid: true},
		ID:           id,
		GroupID:      claims.GroupID,
	}); err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to set gchat space")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// gchatAddBotError converts a raw AddBotToSpace error into a user-readable message.
func gchatAddBotError(err error) string {
	msg := err.Error()
	if strings.Contains(msg, "PERMISSION_DENIED") || strings.Contains(msg, "403") {
		return "Kunde inte lägga till boten automatiskt — administratörskontot är inte medlem i det här utrymmet. " +
			"Lägg till appen manuellt i Google Chat-utrymmet och försök igen."
	}
	return "could not add bot to space: " + msg
}

// ClearGChatSpace unlinks a team from its Google Chat Space.
// Accessible to team members and managers.
func (h *TeamHandler) ClearGChatSpace(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if !h.requireTeamMembership(r.Context(), claims, id) {
		WriteError(w, http.StatusForbidden, "not a member of this team")
		return
	}

	if err := h.Q.ClearTeamGchatSpace(r.Context(), db.ClearTeamGchatSpaceParams{
		ID: id, GroupID: claims.GroupID,
	}); err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to clear gchat space")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
