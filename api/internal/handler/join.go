package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/scoutrustning/api/internal/auth"
	"github.com/malarscouterna/scoutrustning/api/internal/i18n"
	"github.com/malarscouterna/scoutrustning/api/internal/notifications"
)

// JoinHandler handles group signup submissions. It is mounted behind
// auth.Middleware with AllowUnmapped: true, since applicants by definition
// have no registered group yet - see docs/implementation/scout-group-signup.md.
type JoinHandler struct {
	Notifier  notifications.Notifier
	AdminTo   string
}

func (h *JoinHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.Submit)
	return r
}

type joinRequest struct {
	OrgID              string `json:"org_id"`
	GroupName          string `json:"group_name"`
	RoleName           string `json:"role_name"`
	RoleKey            string `json:"role_key"`
	TeamName           string `json:"team_name"`
	ContactEmail       string `json:"contact_email"`
	GroupSize          string `json:"group_size"`
	InterestedInDomain bool   `json:"interested_in_custom_domain"`
}

func (h *JoinHandler) Submit(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req joinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.GroupName = strings.TrimSpace(req.GroupName)
	req.RoleName = strings.TrimSpace(req.RoleName)
	req.RoleKey = strings.TrimSpace(req.RoleKey)
	req.TeamName = strings.TrimSpace(req.TeamName)
	req.ContactEmail = strings.TrimSpace(req.ContactEmail)
	req.GroupSize = strings.TrimSpace(req.GroupSize)
	if req.OrgID == "" || req.GroupName == "" || req.RoleName == "" || req.TeamName == "" || req.ContactEmail == "" {
		WriteError(w, http.StatusBadRequest, "missing required fields")
		return
	}

	// Cross-check the submitted org against the token's own memberships -
	// don't trust org_id from the request body alone, to stop an
	// authenticated-but-unmapped user from claiming an org they don't belong to.
	var roleKeyOK bool
	for _, o := range claims.Orgs {
		if o.ID == req.OrgID {
			roleKeyOK = true
			break
		}
	}
	if !roleKeyOK {
		WriteError(w, http.StatusForbidden, "org not in token")
		return
	}

	if h.Notifier == nil {
		WriteError(w, http.StatusServiceUnavailable, "no smtp config")
		return
	}
	if h.AdminTo == "" {
		slog.Error("ADMIN_EMAIL not configured, refusing group signup")
		WriteError(w, http.StatusServiceUnavailable, "signup is not available right now")
		return
	}

	roleKey := firstNonEmptyStr(req.RoleKey, "<role-key-not-provided>")
	initCmd := fmt.Sprintf(
		"go run ./cmd/server init-group --group-id=%s --group-name=%q --role-key=%s --team-name=%q",
		req.OrgID, req.GroupName, roleKey, req.TeamName,
	)

	adminHTML, adminText := notifications.RenderJoinAdminEmail(notifications.JoinAdminEmailData{
		ApplicantName:      claims.Name,
		ApplicantEmail:     claims.Email,
		OrgID:              req.OrgID,
		GroupName:          req.GroupName,
		RoleName:           req.RoleName,
		TeamName:           req.TeamName,
		ContactEmail:       req.ContactEmail,
		GroupSize:          req.GroupSize,
		InterestedInDomain: req.InterestedInDomain,
		InitGroupCmd:       initCmd,
	})
	adminMsg := notifications.Message{
		To:       h.AdminTo,
		Subject:  notifications.JoinAdminSubject(req.GroupName),
		Body:     adminHTML,
		TextBody: adminText,
	}
	if err := h.Notifier.Send(r.Context(), adminMsg); err != nil {
		slog.Error("join admin email failed", "err", err)
		WriteError(w, http.StatusServiceUnavailable, "failed to send application")
		return
	}

	lang := "sv"
	applicantHTML, applicantText := notifications.RenderJoinApplicantEmail(notifications.JoinApplicantEmailData{
		Lang:               lang,
		Name:               claims.Name,
		OrgID:              req.OrgID,
		GroupName:          req.GroupName,
		RoleName:           req.RoleName,
		ContactEmail:       req.ContactEmail,
		GroupSize:          req.GroupSize,
		InterestedInDomain: req.InterestedInDomain,
	})
	applicantMsg := notifications.Message{
		To:       req.ContactEmail,
		Subject:  i18n.T(lang, "email_join_applicant_subject", map[string]string{"group_name": req.GroupName}),
		Body:     applicantHTML,
		TextBody: applicantText,
	}
	if err := h.Notifier.Send(r.Context(), applicantMsg); err != nil {
		slog.Error("join applicant email failed", "err", err)
	}

	WriteJSON(w, http.StatusOK, map[string]any{"sent": true})
}

func firstNonEmptyStr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
