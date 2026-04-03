package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/malarscouterna/ms-utrustning/api/internal/auth"
	"github.com/malarscouterna/ms-utrustning/api/internal/db"
)

type LocationHandler struct {
	Q *db.Queries
}

func (h *LocationHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.With(auth.RequireRole("equipment_manager")).Post("/", h.Create)
	r.With(auth.RequireRole("equipment_manager")).Put("/{id}", h.Update)
	r.With(auth.RequireRole("equipment_manager")).Delete("/{id}", h.Delete)
	return r
}

func (h *LocationHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	locations, err := h.Q.ListLocations(r.Context(), claims.GroupID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to list locations")
		return
	}
	WriteJSON(w, http.StatusOK, locations)
}

func (h *LocationHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	var req struct {
		Name      string `json:"name"`
		SortOrder int32  `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		WriteError(w, http.StatusBadRequest, "name is required")
		return
	}
	loc, err := h.Q.CreateLocation(r.Context(), db.CreateLocationParams{
		GroupID:   claims.GroupID,
		Name:      req.Name,
		SortOrder: req.SortOrder,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to create location")
		return
	}
	WriteJSON(w, http.StatusCreated, loc)
}

func (h *LocationHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Name      string `json:"name"`
		SortOrder int32  `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	loc, err := h.Q.UpdateLocation(r.Context(), db.UpdateLocationParams{
		ID: id, GroupID: claims.GroupID, Name: req.Name, SortOrder: req.SortOrder,
	})
	if err != nil {
		WriteError(w, http.StatusNotFound, "location not found")
		return
	}
	WriteJSON(w, http.StatusOK, loc)
}

func (h *LocationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.Q.DeleteLocation(r.Context(), db.DeleteLocationParams{ID: id, GroupID: claims.GroupID}); err != nil {
		WriteError(w, http.StatusNotFound, "location not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	return u, u.Scan(s)
}
