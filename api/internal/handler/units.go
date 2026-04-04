package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/ms-utrustning/api/internal/auth"
	"github.com/malarscouterna/ms-utrustning/api/internal/db"
)

type UnitHandler struct {
	Q *db.Queries
}

func (h *UnitHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.With(auth.RequireRole("equipment_manager")).Post("/", h.Create)
	return r
}

func (h *UnitHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	units, err := h.Q.ListUnits(r.Context(), claims.GroupID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to list units")
		return
	}
	WriteJSON(w, http.StatusOK, units)
}

func (h *UnitHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	var req struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		WriteError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Type == "" {
		req.Type = "unit"
	}
	if req.Type != "unit" && req.Type != "project" {
		WriteError(w, http.StatusBadRequest, "type must be unit or project")
		return
	}
	unit, err := h.Q.CreateUnit(r.Context(), db.CreateUnitParams{
		GroupID: claims.GroupID, Name: req.Name, Type: req.Type,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to create unit")
		return
	}
	WriteJSON(w, http.StatusCreated, unit)
}
