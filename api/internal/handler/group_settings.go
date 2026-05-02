package handler

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"

	"github.com/malarscouterna/ms-utrustning/api/internal/auth"
	"github.com/malarscouterna/ms-utrustning/api/internal/crypto"
	"github.com/malarscouterna/ms-utrustning/api/internal/db"
)

type GroupSettingsHandler struct {
	Q     *db.Queries
	Perms *PermissionCache
}

func (h *GroupSettingsHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(auth.RequireRole("equipment_manager"))
	r.Get("/", h.Get)
	r.Put("/", h.Update)
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
	GchatWebhookURL       string   `json:"gchat_webhook_url"`
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
	GchatWebhookURL       string  `json:"gchat_webhook_url"`
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
		GchatWebhookUrl:       req.GchatWebhookURL,
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
		GchatWebhookURL:       s.GchatWebhookUrl,
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
		id := s.LogoFileID.Bytes
		groupID := s.GroupID
		resp.LogoURL = "/api/v0/public/groups/" + groupID + "/logo"
		_ = id // file_id is resolved server-side by the public endpoint
	}
	if len(s.SmtpKeyEncrypted) > 0 {
		decrypted, err := crypto.Decrypt(s.SmtpKeyEncrypted)
		if err == nil {
			resp.SmtpKeyMasked = crypto.MaskKey(string(decrypted))
		}
	}
	return resp
}
