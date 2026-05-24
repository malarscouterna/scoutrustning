package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/scoutrustning/api/internal/auth"
	"github.com/malarscouterna/scoutrustning/api/internal/db"
)

type CategoryHandler struct {
	Q *db.Queries
}

func (h *CategoryHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.With(auth.RequireRole("equipment_manager")).Post("/", h.Create)
	r.With(auth.RequireRole("equipment_manager")).Put("/{id}", h.Update)
	r.With(auth.RequireRole("equipment_manager")).Delete("/{id}", h.Delete)
	return r
}

func (h *CategoryHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	categories, err := h.Q.ListCategories(r.Context(), claims.GroupID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to list categories")
		return
	}
	WriteJSON(w, http.StatusOK, categories)
}

func (h *CategoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	var req struct {
		Name      string  `json:"name"`
		ParentID  *string `json:"parent_id"`
		SortOrder int32   `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		WriteError(w, http.StatusBadRequest, "name is required")
		return
	}
	params := db.CreateCategoryParams{
		GroupID:   claims.GroupID,
		Name:      req.Name,
		SortOrder: req.SortOrder,
	}
	if req.ParentID != nil {
		pid, err := parseUUID(*req.ParentID)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid parent_id")
			return
		}
		params.ParentID = pid
	}
	cat, err := h.Q.CreateCategory(r.Context(), params)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to create category")
		return
	}
	WriteJSON(w, http.StatusCreated, cat)
}

func (h *CategoryHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Name      string  `json:"name"`
		ParentID  *string `json:"parent_id"`
		SortOrder int32   `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	params := db.UpdateCategoryParams{
		ID: id, GroupID: claims.GroupID, Name: req.Name, SortOrder: req.SortOrder,
	}
	if req.ParentID != nil {
		pid, err := parseUUID(*req.ParentID)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid parent_id")
			return
		}
		params.ParentID = pid
	}
	cat, err := h.Q.UpdateCategory(r.Context(), params)
	if err != nil {
		WriteError(w, http.StatusNotFound, "category not found")
		return
	}
	WriteJSON(w, http.StatusOK, cat)
}

func (h *CategoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}
	count, err := h.Q.CountArticlesForCategory(r.Context(), db.CountArticlesForCategoryParams{GroupID: claims.GroupID, CategoryID: id})
	if err == nil && count > 0 {
		WriteJSON(w, http.StatusConflict, map[string]any{"error": "has_articles", "count": count})
		return
	}
	if err := h.Q.DeleteCategory(r.Context(), db.DeleteCategoryParams{ID: id, GroupID: claims.GroupID}); err != nil {
		WriteError(w, http.StatusNotFound, "category not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
