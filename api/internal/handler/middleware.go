package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

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

			teamIDs := make([]pgtype.UUID, 0, len(claims.Teams))
			for _, t := range claims.Teams {
				var uid pgtype.UUID
				if err := uid.Scan(t.TeamID); err == nil {
					teamIDs = append(teamIDs, uid)
				}
			}

			_, err := queries.UpsertUser(r.Context(), db.UpsertUserParams{
				ID:             claims.MemberID,
				GroupID:        claims.GroupID,
				Name:           claims.Name,
				Email:          claims.Email,
				MaxAccessLevel: claims.MaxAccess,
				TeamIds:        teamIDs,
			})
			if err != nil {
				if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23503" && strings.Contains(pgErr.ConstraintName, "group_id") {
					WriteError(w, http.StatusForbidden, "group_not_found")
					return
				}
				slog.Error("failed to upsert user", "error", err, "member_id", claims.MemberID)
				WriteError(w, http.StatusInternalServerError, "internal error")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
