package handler

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/malarscouterna/scoutrustning/api/internal/auth"
	"github.com/malarscouterna/scoutrustning/api/internal/db"
	"github.com/malarscouterna/scoutrustning/api/internal/images"
)

// LogoHandler manages group logo upload/delete and the public serve endpoint.
type LogoHandler struct {
	Q        *db.Queries
	ImageDir string
}

func (h *LogoHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.With(auth.RequireRole("equipment_manager")).Post("/", h.Upload)
	r.With(auth.RequireRole("equipment_manager")).Delete("/", h.Delete)
	return r
}

// PublicLogoRoutes returns routes that are mounted without authentication.
// GET /public/groups/{groupId}/logo — serves WebP (web)
// GET /public/groups/{groupId}/logo.png — serves PNG (email)
func (h *LogoHandler) PublicLogoRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{groupId}/logo", h.ServeWebP)
	r.Get("/{groupId}/logo.png", h.ServePNG)
	return r
}

func (h *LogoHandler) Upload(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	r.Body = http.MaxBytesReader(w, r.Body, images.MaxUploadSize+1024)
	if err := r.ParseMultipartForm(images.MaxUploadSize); err != nil {
		WriteError(w, http.StatusBadRequest, "file_too_large")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "file_required")
		return
	}
	defer file.Close()

	result, err := images.ProcessLogoImage(file)
	if err != nil {
		slog.Error("logo processing failed", "group", claims.GroupID, "error", err)
		WriteError(w, http.StatusBadRequest, "image_processing_failed")
		return
	}

	// Delete previous logo files if one exists.
	if prev, err := h.Q.GetGroupLogoFileID(r.Context(), claims.GroupID); err == nil && prev.Valid {
		h.deleteLogoFiles(prev.Bytes[:])
	}

	if err := h.saveLogoFiles(result); err != nil {
		slog.Error("failed to save logo files", "group", claims.GroupID, "error", err)
		WriteError(w, http.StatusInternalServerError, "save_failed")
		return
	}

	var fileID pgtype.UUID
	fileID.Scan(result.ID)

	if err := h.Q.SetGroupLogo(r.Context(), db.SetGroupLogoParams{
		GroupID:    claims.GroupID,
		LogoFileID: fileID,
	}); err != nil {
		slog.Error("failed to store logo file_id", "group", claims.GroupID, "error", err)
		h.deleteLogoFiles(fileID.Bytes[:])
		WriteError(w, http.StatusInternalServerError, "db_update_failed")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{
		"logo_url": "/api/v0/public/groups/" + claims.GroupID + "/logo",
	})
}

func (h *LogoHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	fileID, err := h.Q.GetGroupLogoFileID(r.Context(), claims.GroupID)
	if err != nil || !fileID.Valid {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := h.Q.ClearGroupLogo(r.Context(), claims.GroupID); err != nil {
		WriteError(w, http.StatusInternalServerError, "db_update_failed")
		return
	}

	h.deleteLogoFiles(fileID.Bytes[:])
	w.WriteHeader(http.StatusNoContent)
}

func (h *LogoHandler) ServeWebP(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "groupId")
	fileID, err := h.Q.GetGroupLogoFileID(r.Context(), groupID)
	if err != nil || !fileID.Valid {
		http.NotFound(w, r)
		return
	}
	h.serveLogoFile(w, r, fileID.Bytes[:], ".webp", "image/webp")
}

func (h *LogoHandler) ServePNG(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "groupId")
	fileID, err := h.Q.GetGroupLogoFileID(r.Context(), groupID)
	if err != nil || !fileID.Valid {
		http.NotFound(w, r)
		return
	}
	h.serveLogoFile(w, r, fileID.Bytes[:], ".png", "image/png")
}

func (h *LogoHandler) serveLogoFile(w http.ResponseWriter, r *http.Request, fileID []byte, ext, contentType string) {
	id := fmt.Sprintf("%x-%x-%x-%x-%x", fileID[0:4], fileID[4:6], fileID[6:8], fileID[8:10], fileID[10:16])
	path := filepath.Join(h.logosDir(), id+ext)
	f, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	io.Copy(w, f)
}

func (h *LogoHandler) saveLogoFiles(result *images.LogoResult) error {
	dir := h.logosDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, result.ID+".webp"), result.WebP, 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, result.ID+".png"), result.PNG, 0644)
}

func (h *LogoHandler) deleteLogoFiles(fileIDBytes []byte) {
	if len(fileIDBytes) != 16 {
		return
	}
	id := fmt.Sprintf("%x-%x-%x-%x-%x", fileIDBytes[0:4], fileIDBytes[4:6], fileIDBytes[6:8], fileIDBytes[8:10], fileIDBytes[10:16])
	dir := h.logosDir()
	os.Remove(filepath.Join(dir, id+".webp"))
	os.Remove(filepath.Join(dir, id+".png"))
}

func (h *LogoHandler) logosDir() string {
	return filepath.Join(h.ImageDir, "logos")
}
