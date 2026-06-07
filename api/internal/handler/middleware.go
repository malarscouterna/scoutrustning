package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/malarscouterna/scoutrustning/api/internal/auth"
	"github.com/malarscouterna/scoutrustning/api/internal/db"
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

			// Upsert a row for each group the user belongs to so settings persist
			// even before the user switches to that group for the first time.
			// Fall back to the active group alone when AvailableGroups is unpopulated (dev/test claims).
			groups := claims.AvailableGroups
			if len(groups) == 0 {
				groups = []auth.GroupSummary{{ID: claims.GroupID}}
			}
			for _, g := range groups {
				ids := teamIDs
				if g.ID != claims.GroupID {
					ids = nil // team memberships are only known for the active group
				}
				_, err := queries.UpsertUser(r.Context(), db.UpsertUserParams{
					ID:             claims.MemberID,
					GroupID:        g.ID,
					Name:           claims.Name,
					Email:          claims.Email,
					MaxAccessLevel: claims.MaxAccess,
					TeamIds:        ids,
				})
				if err != nil {
					if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23503" && strings.Contains(pgErr.ConstraintName, "group_id") {
						WriteError(w, http.StatusForbidden, "group_not_found")
						return
					}
					slog.Error("failed to upsert user", "error", err, "member_id", claims.MemberID, "group_id", g.ID)
					WriteError(w, http.StatusInternalServerError, "internal error")
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
