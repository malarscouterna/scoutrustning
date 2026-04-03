package handler

import (
	"log/slog"
	"net/http"

	"github.com/malarscouterna/ms-utrustning/api/internal/auth"
	"github.com/malarscouterna/ms-utrustning/api/internal/db"
)

func UpsertUserMiddleware(queries *db.Queries) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.ClaimsFromContext(r.Context())
			if !ok {
				WriteError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			_, err := queries.UpsertUser(r.Context(), db.UpsertUserParams{
				ID:      claims.MemberID,
				GroupID: claims.GroupID,
				Name:    claims.Name,
				Email:   claims.Email,
			})
			if err != nil {
				slog.Error("failed to upsert user", "error", err, "member_id", claims.MemberID)
				WriteError(w, http.StatusInternalServerError, "internal error")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
