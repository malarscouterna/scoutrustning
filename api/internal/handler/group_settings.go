package handler

import (
	"encoding/json"
	"net/http"

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
	NotificationEmailFrom string `json:"notification_email_from"`
	SmtpKeySet            bool   `json:"smtp_key_set"`
	SmtpKeyMasked         string `json:"smtp_key_masked"`
	GchatWebhookURL       string `json:"gchat_webhook_url"`
	DefaultApprovalLevel  string `json:"default_approval_level"`
	DefaultAccessUnknown  string `json:"default_access_unknown"`
	DefaultAccessTroop    string `json:"default_access_troop"`
	DefaultAccessRole     string `json:"default_access_role"`
	ImageUploadRole       string `json:"image_upload_role"`
	BookingRole           string `json:"booking_role"`
	ArticleEditRole       string `json:"article_edit_role"`
	IssueResolveRole      string `json:"issue_resolve_role"`
	ManagerNotesRole      string `json:"manager_notes_role"`
	DefaultLanguage       string `json:"default_language"`
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
		})
		return
	}

	resp := settingsToResponse(settings)

	if len(settings.SmtpKeyEncrypted) > 0 {
		resp.SmtpKeySet = true
		decrypted, err := crypto.Decrypt(settings.SmtpKeyEncrypted)
		if err == nil {
			resp.SmtpKeyMasked = crypto.MaskKey(string(decrypted))
		}
	}

	WriteJSON(w, http.StatusOK, resp)
}

type groupSettingsRequest struct {
	NotificationEmailFrom string  `json:"notification_email_from"`
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

	settings, err := h.Q.UpsertGroupSettings(r.Context(), db.UpsertGroupSettingsParams{
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
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to save settings")
		return
	}

	h.Perms.Invalidate(claims.GroupID)

	resp := settingsToResponse(settings)
	if len(settings.SmtpKeyEncrypted) > 0 {
		decrypted, err := crypto.Decrypt(settings.SmtpKeyEncrypted)
		if err == nil {
			resp.SmtpKeyMasked = crypto.MaskKey(string(decrypted))
		}
	}

	WriteJSON(w, http.StatusOK, resp)
}

func settingsToResponse(s db.GroupSetting) groupSettingsResponse {
	resp := groupSettingsResponse{
		NotificationEmailFrom: s.NotificationEmailFrom,
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
		SmtpKeySet:            len(s.SmtpKeyEncrypted) > 0,
	}
	if len(s.SmtpKeyEncrypted) > 0 {
		decrypted, err := crypto.Decrypt(s.SmtpKeyEncrypted)
		if err == nil {
			resp.SmtpKeyMasked = crypto.MaskKey(string(decrypted))
		}
	}
	return resp
}
