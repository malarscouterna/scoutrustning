package handler

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/scoutrustning/api/internal/auth"
	"github.com/malarscouterna/scoutrustning/api/internal/db"
)

type UserHandler struct {
	Q          *db.Queries
	Perms      *PermissionCache
	DemoMode   bool
	PersonaIDs map[string]bool // non-nil only in demo mode
}

func (h *UserHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Get("/{id}", h.Get)
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

	result := make([]userResponse, 0, len(users))
	for _, u := range users {
		if h.DemoMode && !h.PersonaIDs[u.ID] {
			continue
		}
		result = append(result, userResponse{
			ID:          u.ID,
			Name:        u.Name,
			Email:       u.Email,
			AccessLevel: u.MaxAccessLevel,
		})
	}

	WriteJSON(w, http.StatusOK, result)
}

var openBookingStatusesManager = []string{"draft", "submitted", "approved", "rejected", "confirmed", "picked_up"}
var openBookingStatusesOther = []string{"submitted", "approved", "confirmed", "picked_up"}

func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id := chi.URLParam(r, "id")

	user, err := h.Q.GetUser(r.Context(), db.GetUserParams{ID: id, GroupID: claims.GroupID})
	if err != nil {
		WriteError(w, http.StatusNotFound, "user not found")
		return
	}
	if h.DemoMode && !h.PersonaIDs[user.ID] {
		WriteError(w, http.StatusNotFound, "user not found")
		return
	}

	teams, err := h.Q.GetUserTeamAffiliations(r.Context(), db.GetUserTeamAffiliationsParams{
		ID:      id,
		GroupID: claims.GroupID,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	statuses := openBookingStatusesOther
	if claims.IsManager() {
		statuses = openBookingStatusesManager
	}
	bookings, err := h.Q.GetUserOpenBookings(r.Context(), db.GetUserOpenBookingsParams{
		ID:       id,
		GroupID:  claims.GroupID,
		Statuses: statuses,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}

	type teamAffiliation struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Type        string `json:"type"`
		AccessLevel string `json:"access_level"`
	}
	teamResults := make([]teamAffiliation, 0, len(teams))
	for _, t := range teams {
		teamResults = append(teamResults, teamAffiliation{
			ID:          formatUUID(t.ID),
			Name:        t.Name,
			Type:        t.Type,
			AccessLevel: t.AccessLevel,
		})
	}

	type openBooking struct {
		ID             string `json:"id"`
		Status         string `json:"status"`
		StartDate      string `json:"start_date"`
		EndDate        string `json:"end_date"`
		UsedByTeamID   string `json:"used_by_team_id,omitempty"`
		TeamName       string `json:"team_name,omitempty"`
		UsedByExternal string `json:"used_by_external,omitempty"`
		Notes          string `json:"notes,omitempty"`
	}
	bookingResults := make([]openBooking, 0, len(bookings))
	for _, b := range bookings {
		ob := openBooking{
			ID:        formatUUID(b.ID),
			Status:    b.Status,
			StartDate: b.StartDate.Time.Format("2006-01-02"),
			EndDate:   b.EndDate.Time.Format("2006-01-02"),
			Notes:     b.Notes,
		}
		if b.UsedByTeamID.Valid {
			ob.UsedByTeamID = formatUUID(b.UsedByTeamID)
		}
		if b.TeamName.Valid {
			ob.TeamName = b.TeamName.String
		}
		if b.UsedByExternal.Valid {
			ob.UsedByExternal = b.UsedByExternal.String
		}
		bookingResults = append(bookingResults, ob)
	}

	var picture *string
	if user.Picture.Valid {
		picture = &user.Picture.String
	}
	var notificationEmail *string
	if user.NotificationEmail.Valid {
		notificationEmail = &user.NotificationEmail.String
	}

	type issueSummary struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		Severity  string `json:"severity"`
		Status    string `json:"status"`
		CreatedAt string `json:"created_at"`
	}
	issueResults := []issueSummary{}
	perms := h.Perms.Get(r, claims.GroupID)
	if auth.AccessAtLeast(claims.MaxAccess, perms.IssueResolve) {
		issues, err := h.Q.GetUserIssues(r.Context(), db.GetUserIssuesParams{
			GroupID: claims.GroupID,
			UserID:  id,
		})
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "internal error")
			return
		}
		for _, issue := range issues {
			issueResults = append(issueResults, issueSummary{
				ID:        formatUUID(issue.ID),
				Title:     issue.Title,
				Severity:  issue.Severity,
				Status:    issue.Status,
				CreatedAt: issue.CreatedAt.Time.Format("2006-01-02"),
			})
		}
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"id":                 user.ID,
		"name":               user.Name,
		"picture":            picture,
		"notification_email": notificationEmail,
		"teams":              teamResults,
		"open_bookings":      bookingResults,
		"issues":             issueResults,
	})
}
