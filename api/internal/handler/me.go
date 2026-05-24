package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/mail"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/malarscouterna/scoutrustning/api/internal/auth"
	"github.com/malarscouterna/scoutrustning/api/internal/db"
	"github.com/malarscouterna/scoutrustning/api/internal/i18n"
	"github.com/malarscouterna/scoutrustning/api/internal/notifications"
)

type MeHandler struct {
	Q     *db.Queries
	Perms *PermissionCache
	// NotifPrefs handles /me/notification-prefs sub-routes.
	NotifPrefs *NotificationPrefsHandler
	// Notifier is used exclusively by the test-email endpoint.
	Notifier   notifications.Notifier
	// PersonaIDs is non-nil in demo mode. Persona users are skipped by test-email.
	PersonaIDs map[string]bool
	DemoMode   bool
	BaseURL    string
}

func (h *MeHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.Get)
	r.Put("/language", h.PutLanguage)
	r.Put("/notification-email", h.PutNotificationEmail)
	r.Post("/test-email", h.PostTestEmail)
	r.Mount("/notification-prefs", h.NotifPrefs.MeRoutes())
	return r
}

func (h *MeHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	groupName := ""
	if g, err := h.Q.GetGroup(r.Context(), claims.GroupID); err == nil {
		groupName = g.Name
	}
	perms := h.Perms.Get(r, claims.GroupID)

	lang := "sv"
	var notificationEmail *string
	if settings, err := h.Q.GetGroupSettings(r.Context(), claims.GroupID); err == nil {
		lang = settings.DefaultLanguage
	}
	if user, err := h.Q.GetUser(r.Context(), db.GetUserParams{
		ID:      claims.MemberID,
		GroupID: claims.GroupID,
	}); err == nil {
		if user.Language.Valid && i18n.Supported(user.Language.String) {
			lang = user.Language.String
		}
		if user.NotificationEmail.Valid {
			notificationEmail = &user.NotificationEmail.String
		}
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"member_id":          claims.MemberID,
		"group_id":           claims.GroupID,
		"group_name":         groupName,
		"name":               claims.Name,
		"email":              claims.Email,
		"notification_email": notificationEmail,
		"teams":              claims.Teams,
		"max_access":         claims.MaxAccess,
		"language":           lang,
		"permissions": map[string]string{
			"image_upload":  perms.ImageUpload,
			"booking":       perms.Booking,
			"article_edit":  perms.ArticleEdit,
			"issue_resolve": perms.IssueResolve,
			"manager_notes": perms.ManagerNotes,
		},
	})
}

func (h *MeHandler) PostTestEmail(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.DemoMode || (h.PersonaIDs != nil && h.PersonaIDs[claims.MemberID]) {
		WriteJSON(w, http.StatusOK, map[string]any{"skipped": true})
		return
	}
	if h.Notifier == nil {
		WriteError(w, http.StatusServiceUnavailable, "no smtp config")
		return
	}
	lang := "sv"
	to := claims.Email
	if user, err := h.Q.GetUser(r.Context(), db.GetUserParams{ID: claims.MemberID, GroupID: claims.GroupID}); err == nil {
		if user.Language.Valid && i18n.Supported(user.Language.String) {
			lang = user.Language.String
		}
		if user.NotificationEmail.Valid {
			to = user.NotificationEmail.String
		}
	}
	baseURL := h.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:5173"
	}
	group, _ := h.Q.GetGroup(r.Context(), claims.GroupID)
	logoURL := notifications.GroupLogoURL(r.Context(), h.Q, claims.GroupID, baseURL)
	htmlBody, textBody := notifications.RenderTestEmail(lang, claims.Name, group.Name, logoURL, baseURL)
	msg := notifications.Message{
		GroupID:  claims.GroupID,
		To:       to,
		Subject:  i18n.T(lang, "email_subject_test_email"),
		Body:     htmlBody,
		TextBody: textBody,
	}
	if err := h.Notifier.Send(r.Context(), msg); err != nil {
		slog.Error("test email failed", "to", to, "group", claims.GroupID, "err", err)
		WriteError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"sent": true})
}

func (h *MeHandler) PutNotificationEmail(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if h.DemoMode {
		WriteJSON(w, http.StatusOK, map[string]any{"skipped": true})
		return
	}
	var req struct {
		Email *string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == nil || *req.Email == "" {
		// Clear override — revert to ScoutID email.
		if err := h.Q.ClearUserNotificationEmail(r.Context(), db.ClearUserNotificationEmailParams{
			ID:      claims.MemberID,
			GroupID: claims.GroupID,
		}); err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to clear notification email")
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"cleared": true})
		return
	}

	addr, err := mail.ParseAddress(*req.Email)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid email address")
		return
	}
	email := addr.Address

	if err := h.Q.SetUserNotificationEmail(r.Context(), db.SetUserNotificationEmailParams{
		NotificationEmail: pgtype.Text{String: email, Valid: true},
		ID:                claims.MemberID,
		GroupID:           claims.GroupID,
	}); err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to save notification email")
		return
	}

	// Send a test email so the user can verify delivery.
	if h.Notifier != nil {
		lang := "sv"
		if user, err := h.Q.GetUser(r.Context(), db.GetUserParams{ID: claims.MemberID, GroupID: claims.GroupID}); err == nil {
			if user.Language.Valid && i18n.Supported(user.Language.String) {
				lang = user.Language.String
			}
		}
		baseURL := h.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:5173"
		}
		group, _ := h.Q.GetGroup(r.Context(), claims.GroupID)
		logoURL := notifications.GroupLogoURL(r.Context(), h.Q, claims.GroupID, baseURL)
		htmlBody, textBody := notifications.RenderTestEmail(lang, claims.Name, group.Name, logoURL, baseURL)
		msg := notifications.Message{
			GroupID:  claims.GroupID,
			To:       email,
			Subject:  i18n.T(lang, "email_subject_test_email"),
			Body:     htmlBody,
			TextBody: textBody,
		}
		if sendErr := h.Notifier.Send(r.Context(), msg); sendErr != nil {
			slog.Error("notification email test failed", "to", email, "group", claims.GroupID, "err", sendErr)
			WriteError(w, http.StatusServiceUnavailable, sendErr.Error())
			return
		}
	}

	WriteJSON(w, http.StatusOK, map[string]any{"sent": true})
}

func (h *MeHandler) PutLanguage(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req struct {
		Language *string `json:"language"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var lang pgtype.Text
	if req.Language != nil {
		if *req.Language != "" && !i18n.Supported(*req.Language) {
			WriteError(w, http.StatusBadRequest, "unsupported language")
			return
		}
		lang = pgtype.Text{String: *req.Language, Valid: *req.Language != ""}
	}
	if err := h.Q.UpdateUserLanguage(r.Context(), db.UpdateUserLanguageParams{
		Language: lang,
		ID:       claims.MemberID,
	}); err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to update language")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
