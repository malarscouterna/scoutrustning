package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"slices"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/ms-utrustning/api/internal/auth"
	"github.com/malarscouterna/ms-utrustning/api/internal/crypto"
	"github.com/malarscouterna/ms-utrustning/api/internal/db"
	"github.com/malarscouterna/ms-utrustning/api/internal/notifications"
)

type GroupSettingsHandler struct {
	Q     *db.Queries
	Perms *PermissionCache
}

func (h *GroupSettingsHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/gchat-spaces", h.ListGChatSpaces) // any authenticated group member
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireRole("equipment_manager"))
		r.Get("/", h.Get)
		r.Put("/", h.Update)
		r.Post("/gchat-key", h.UploadGChatKey)
		r.Delete("/gchat-key", h.DeleteGChatKey)
	})
	return r
}

type groupSettingsResponse struct {
	NotificationEmailFrom string   `json:"notification_email_from"`
	SmtpHost              string   `json:"smtp_host"`
	SmtpPort              int32    `json:"smtp_port"`
	SmtpTls               string   `json:"smtp_tls"`
	SmtpUser              string   `json:"smtp_user"`
	SmtpKeySet            bool     `json:"smtp_key_set"`
	SmtpKeyMasked         string   `json:"smtp_key_masked"`
	SystemSmtpConfigured  bool     `json:"system_smtp_configured"`
	SystemSmtpFrom        string   `json:"system_smtp_from"`
	GchatConfigured       bool     `json:"gchat_configured"`
	GchatAdminEmail       string   `json:"gchat_admin_email"`
	DefaultApprovalLevel  string   `json:"default_approval_level"`
	DefaultAccessUnknown  string   `json:"default_access_unknown"`
	DefaultAccessTroop    string   `json:"default_access_troop"`
	DefaultAccessRole     string   `json:"default_access_role"`
	ImageUploadRole       string   `json:"image_upload_role"`
	BookingRole           string   `json:"booking_role"`
	ArticleEditRole       string   `json:"article_edit_role"`
	IssueResolveRole      string   `json:"issue_resolve_role"`
	ManagerNotesRole      string   `json:"manager_notes_role"`
	DefaultLanguage       string   `json:"default_language"`
	NotificationChannels  []string `json:"notification_channels"`
	LogoURL               string   `json:"logo_url"` // empty string when no logo uploaded
}

func (h *GroupSettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	settings, err := h.Q.GetGroupSettings(r.Context(), claims.GroupID)
	if err != nil {
		// No settings yet — return defaults
		WriteJSON(w, http.StatusOK, groupSettingsResponse{
			DefaultApprovalLevel: "none",
			DefaultAccessUnknown: "view",
			DefaultAccessTroop:   "book",
			DefaultAccessRole:    "book",
			ImageUploadRole:      "book",
			BookingRole:          "book",
			ArticleEditRole:      "manager",
			IssueResolveRole:     "manager",
			ManagerNotesRole:     "manager",
			DefaultLanguage:      "sv",
			NotificationChannels: []string{"email"},
			SystemSmtpConfigured: os.Getenv("SMTP_DEFAULT_HOST") != "",
			SystemSmtpFrom:       os.Getenv("SMTP_DEFAULT_FROM"),
		})
		return
	}

	WriteJSON(w, http.StatusOK, settingsToResponse(settings))
}

type groupSettingsRequest struct {
	NotificationEmailFrom string  `json:"notification_email_from"`
	SmtpHost              string  `json:"smtp_host"`
	SmtpPort              int32   `json:"smtp_port"`
	SmtpTls               string  `json:"smtp_tls"`
	SmtpUser              string  `json:"smtp_user"`
	SmtpKey               *string `json:"smtp_key"`
	DefaultApprovalLevel  string  `json:"default_approval_level"`
	DefaultAccessUnknown  string  `json:"default_access_unknown"`
	DefaultAccessTroop    string  `json:"default_access_troop"`
	DefaultAccessRole     string  `json:"default_access_role"`
	ImageUploadRole       string  `json:"image_upload_role"`
	BookingRole           string  `json:"booking_role"`
	ArticleEditRole       string  `json:"article_edit_role"`
	IssueResolveRole      string  `json:"issue_resolve_role"`
	ManagerNotesRole      string  `json:"manager_notes_role"`
	DefaultLanguage       string  `json:"default_language"`
}

func (h *GroupSettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	var req groupSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	approvalLevel := req.DefaultApprovalLevel
	if approvalLevel == "" {
		approvalLevel = "none"
	}
	validLevels := map[string]bool{"none": true, "low": true, "high": true}
	if !validLevels[approvalLevel] {
		WriteError(w, http.StatusBadRequest, "invalid default_approval_level")
		return
	}

	imageUploadRole := req.ImageUploadRole
	if imageUploadRole == "" {
		imageUploadRole = "book"
	}
	if !validAccessLevel(imageUploadRole) {
		WriteError(w, http.StatusBadRequest, "invalid image_upload_role")
		return
	}

	// Validate access defaults
	defaultAccessUnknown := req.DefaultAccessUnknown
	if defaultAccessUnknown == "" { defaultAccessUnknown = "view" }
	defaultAccessTroop := req.DefaultAccessTroop
	if defaultAccessTroop == "" { defaultAccessTroop = "book" }
	defaultAccessRole := req.DefaultAccessRole
	if defaultAccessRole == "" { defaultAccessRole = "book" }
	if !validAccessLevel(defaultAccessUnknown) || !validAccessLevel(defaultAccessTroop) || !validAccessLevel(defaultAccessRole) {
		WriteError(w, http.StatusBadRequest, "invalid default_access level")
		return
	}

	// Validate configurable permission levels with minimum bounds
	permFields := map[string]*string{
		"booking_role":       &req.BookingRole,
		"article_edit_role":  &req.ArticleEditRole,
		"issue_resolve_role": &req.IssueResolveRole,
		"manager_notes_role": &req.ManagerNotesRole,
	}
	permDefaults := map[string]string{
		"booking_role": "book", "article_edit_role": "manager",
		"issue_resolve_role": "manager", "manager_notes_role": "manager",
	}
	for key, val := range permFields {
		if *val == "" {
			*val = permDefaults[key]
		}
		if !ValidatePermissionLevel(key, *val) {
			WriteErrorWithParams(w, http.StatusBadRequest, "invalid_setting", map[string]string{"field": key})
			return
		}
	}

	defaultLanguage := req.DefaultLanguage
	if defaultLanguage == "" {
		defaultLanguage = "sv"
	}
	validLanguages := map[string]bool{"sv": true, "en": true}
	if !validLanguages[defaultLanguage] {
		WriteError(w, http.StatusBadRequest, "invalid default_language")
		return
	}

	// Handle SMTP key: nil = don't change, empty string = clear, non-empty = encrypt and store
	var smtpKeyEncrypted []byte
	if req.SmtpKey != nil {
		if *req.SmtpKey != "" {
			encrypted, err := crypto.Encrypt([]byte(*req.SmtpKey))
			if err != nil {
				WriteError(w, http.StatusInternalServerError, "failed to encrypt smtp key")
				return
			}
			smtpKeyEncrypted = encrypted
		}
		// empty string = clear (smtpKeyEncrypted stays nil)
	} else {
		// nil = preserve existing
		existing, err := h.Q.GetGroupSettings(r.Context(), claims.GroupID)
		if err == nil {
			smtpKeyEncrypted = existing.SmtpKeyEncrypted
		}
	}

	if _, err := h.Q.UpsertGroupSettings(r.Context(), db.UpsertGroupSettingsParams{
		GroupID:               claims.GroupID,
		NotificationEmailFrom: req.NotificationEmailFrom,
		SmtpKeyEncrypted:      smtpKeyEncrypted,
		DefaultApprovalLevel:  approvalLevel,
		DefaultAccessUnknown:  defaultAccessUnknown,
		DefaultAccessTroop:    defaultAccessTroop,
		DefaultAccessRole:     defaultAccessRole,
		ImageUploadRole:       imageUploadRole,
		BookingRole:           req.BookingRole,
		ArticleEditRole:       req.ArticleEditRole,
		IssueResolveRole:      req.IssueResolveRole,
		ManagerNotesRole:      req.ManagerNotesRole,
		DefaultLanguage:       defaultLanguage,
	}); err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to save settings")
		return
	}

	smtpPort := req.SmtpPort
	if smtpPort == 0 {
		smtpPort = 587
	}
	smtpTls := req.SmtpTls
	if smtpTls == "" {
		smtpTls = "starttls"
	}
	settings, err := h.Q.UpdateSmtpSettings(r.Context(), db.UpdateSmtpSettingsParams{
		GroupID:               claims.GroupID,
		NotificationEmailFrom: req.NotificationEmailFrom,
		SmtpHost:              req.SmtpHost,
		SmtpPort:              smtpPort,
		SmtpTls:               smtpTls,
		SmtpUser:              req.SmtpUser,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to save smtp settings")
		return
	}

	h.Perms.Invalidate(claims.GroupID)

	WriteJSON(w, http.StatusOK, settingsToResponse(settings))
}

// UploadGChatKey accepts the service account JSON as the request body, validates it by
// listing spaces, stores it encrypted, and appends "gchat" to enabled_channels.
func (h *GroupSettingsHandler) UploadGChatKey(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	// The body must be a JSON object with two fields: key_json and admin_email.
	var payload struct {
		KeyJSON    json.RawMessage `json:"key_json"`
		AdminEmail string          `json:"admin_email"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || len(payload.KeyJSON) == 0 {
		WriteError(w, http.StatusBadRequest, "invalid JSON key file")
		return
	}
	keyBytes, err := payload.KeyJSON.MarshalJSON()
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON key file")
		return
	}
	adminEmail := payload.AdminEmail

	// Validate by listing spaces.
	slog.Info("gchat: attempting connection", "admin_email", adminEmail, "key_bytes_len", len(keyBytes))
	spaces, err := notifications.ListGChatSpaces(r.Context(), keyBytes, adminEmail)
	if err != nil {
		slog.Error("gchat: connection failed", "err", err, "admin_email", adminEmail)
		WriteError(w, http.StatusBadRequest, "gchat_key_invalid: "+err.Error())
		return
	}

	encrypted, err := crypto.Encrypt(keyBytes)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to encrypt key")
		return
	}

	if err := h.Q.SetGchatCredentials(r.Context(), db.SetGchatCredentialsParams{
		GchatServiceAccountJsonEncrypted: encrypted,
		GchatAdminEmail:                  adminEmail,
		GroupID:                          claims.GroupID,
	}); err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to save credentials")
		return
	}

	// Append "gchat" to enabled_channels if not already present.
	gs, _ := h.Q.GetGroupSettings(r.Context(), claims.GroupID)
	channels := gs.EnabledChannels
	if !slices.Contains(channels, "gchat") {
		channels = append(channels, "gchat")
		_ = h.Q.UpdateEnabledChannels(r.Context(), db.UpdateEnabledChannelsParams{
			EnabledChannels: channels,
			GroupID:         claims.GroupID,
		})
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"gchat_configured": true,
		"gchat_admin_email": adminEmail,
		"spaces":           spaces,
	})
}

// DeleteGChatKey removes GChat credentials, removes "gchat" from enabled_channels,
// and clears all team space mappings for the group.
func (h *GroupSettingsHandler) DeleteGChatKey(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	_ = h.Q.ClearAllGchatSpacesForGroup(r.Context(), claims.GroupID)
	_ = h.Q.ClearGchatCredentials(r.Context(), claims.GroupID)

	gs, _ := h.Q.GetGroupSettings(r.Context(), claims.GroupID)
	channels := make([]string, 0, len(gs.EnabledChannels))
	for _, ch := range gs.EnabledChannels {
		if ch != "gchat" {
			channels = append(channels, ch)
		}
	}
	_ = h.Q.UpdateEnabledChannels(r.Context(), db.UpdateEnabledChannelsParams{
		EnabledChannels: channels,
		GroupID:         claims.GroupID,
	})

	w.WriteHeader(http.StatusNoContent)
}

// ListGChatSpaces returns spaces accessible to the stored service account.
func (h *GroupSettingsHandler) ListGChatSpaces(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	creds, err := h.Q.GetGchatCredentials(r.Context(), claims.GroupID)
	if err != nil || len(creds.GchatServiceAccountJsonEncrypted) == 0 {
		WriteError(w, http.StatusBadRequest, "gchat not configured")
		return
	}
	saJSON, err := crypto.Decrypt(creds.GchatServiceAccountJsonEncrypted)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to decrypt credentials")
		return
	}

	spaces, err := notifications.ListGChatSpaces(r.Context(), saJSON, creds.GchatAdminEmail)
	if err != nil {
		WriteError(w, http.StatusBadGateway, "failed to list spaces: "+err.Error())
		return
	}

	// Mark spaces already linked to a team as bot_is_member — the bot was added when the link
	// was created, so the DB is the authoritative source even if the bot token listing fails.
	if teams, dbErr := h.Q.ListTeamsWithGchatInfo(r.Context(), claims.GroupID); dbErr == nil {
		linked := make(map[string]bool, len(teams))
		for _, t := range teams {
			if t.GchatSpaceID.Valid && t.GchatSpaceID.String != "" {
				linked[t.GchatSpaceID.String] = true
			}
		}
		for i := range spaces {
			if linked[spaces[i].Name] {
				spaces[i].BotIsMember = true
			}
		}
	}

	WriteJSON(w, http.StatusOK, spaces)
}

func settingsToResponse(s db.GroupSetting) groupSettingsResponse {
	resp := groupSettingsResponse{
		NotificationEmailFrom: s.NotificationEmailFrom,
		SmtpHost:              s.SmtpHost,
		SmtpPort:              s.SmtpPort,
		SmtpTls:               s.SmtpTls,
		SmtpUser:              s.SmtpUser,
		SmtpKeySet:            len(s.SmtpKeyEncrypted) > 0,
		SystemSmtpConfigured:  os.Getenv("SMTP_DEFAULT_HOST") != "",
		SystemSmtpFrom:        os.Getenv("SMTP_DEFAULT_FROM"),
		GchatConfigured:       len(s.GchatServiceAccountJsonEncrypted) > 0,
		GchatAdminEmail:       s.GchatAdminEmail,
		DefaultApprovalLevel:  s.DefaultApprovalLevel,
		DefaultAccessUnknown:  s.DefaultAccessUnknown,
		DefaultAccessTroop:    s.DefaultAccessTroop,
		DefaultAccessRole:     s.DefaultAccessRole,
		ImageUploadRole:       s.ImageUploadRole,
		BookingRole:           s.BookingRole,
		ArticleEditRole:       s.ArticleEditRole,
		IssueResolveRole:      s.IssueResolveRole,
		ManagerNotesRole:      s.ManagerNotesRole,
		DefaultLanguage:       s.DefaultLanguage,
		NotificationChannels:  s.EnabledChannels,
	}
	if s.LogoFileID.Valid {
		resp.LogoURL = "/api/v0/public/groups/" + s.GroupID + "/logo"
	}
	if len(s.SmtpKeyEncrypted) > 0 {
		decrypted, err := crypto.Decrypt(s.SmtpKeyEncrypted)
		if err == nil {
			resp.SmtpKeyMasked = crypto.MaskKey(string(decrypted))
		}
	}
	return resp
}
