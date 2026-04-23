package handler

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/ms-utrustning/api/internal/auth"
	"github.com/malarscouterna/ms-utrustning/api/internal/db"
)

type UserHandler struct {
	Q *db.Queries
}

func (h *UserHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	return r
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	if !claims.IsManager() {
		WriteError(w, http.StatusForbidden, "forbidden")
		return
	}

	accessLevels := []string{}
	if v := r.URL.Query().Get("access_levels"); v != "" {
		accessLevels = strings.Split(v, ",")
	}

	users, err := h.Q.ListUsersByGroup(r.Context(), db.ListUsersByGroupParams{
		GroupID:      claims.GroupID,
		AccessLevels: accessLevels,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	type userResponse struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Email       string `json:"email"`
		AccessLevel string `json:"access_level"`
	}

	result := make([]userResponse, len(users))
	for i, u := range users {
		result[i] = userResponse{
			ID:          u.ID,
			Name:        u.Name,
			Email:       u.Email,
			AccessLevel: u.MaxAccessLevel,
		}
	}

	WriteJSON(w, http.StatusOK, result)
}
