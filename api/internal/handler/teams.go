package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/malarscouterna/ms-utrustning/api/internal/auth"
	"github.com/malarscouterna/ms-utrustning/api/internal/db"
)

type TeamHandler struct {
	Q *db.Queries
}

func (h *TeamHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.With(auth.RequireRole("equipment_manager")).Post("/", h.Create)
	r.With(auth.RequireRole("equipment_manager")).Put("/{id}", h.Update)
	r.With(auth.RequireRole("equipment_manager")).Delete("/{id}", h.Delete)
	return r
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
