package images

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/malarscouterna/scoutrustning/api/internal/auth"
	"github.com/malarscouterna/scoutrustning/api/internal/db"
)

var validImageID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type Handler struct {
	Q        *db.Queries
	ImageDir string
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/product", h.UploadProduct)
	r.Post("/product/from-shared", h.AddFromShared)
	r.Get("/product", h.ListProductImages)
	r.Get("/product/{imageId}", h.GetProductImageMeta)
	r.Put("/product/{imageId}", h.UpdateProductImageMeta)
	r.Delete("/product/{imageId}", h.DeleteProductImage)
	r.Delete("/my/{imageId}", h.DeleteMyImage)
	r.With(auth.RequireRole("equipment_manager")).Put("/product/reorder", h.ReorderProductImages)
	r.Get("/shared", h.ListShared)
	r.Get("/my", h.ListMyImages)
	r.Get("/my/{imageId}/articles", h.ListArticlesUsingImage)
	r.Post("/issue", h.UploadIssue)
	r.Get("/{filename}", h.Serve)
	return r
}

// canUpload checks if the user meets the image_upload_role threshold.
// image_upload_role is now an access level (view/book/trusted/manager).
func (h *Handler) canUpload(r *http.Request, claims auth.Claims) bool {
	required, err := h.Q.GetImageUploadRole(r.Context(), claims.GroupID)
	if err != nil {
		required = auth.AccessBook
	}
	return auth.AccessAtLeast(claims.MaxAccess, required)
}

func (h *Handler) UploadProduct(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	if !h.canUpload(r, claims) {
		writeError(w, http.StatusForbidden, "insufficient_upload_permission")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadSize+1024)
	if err := r.ParseMultipartForm(MaxUploadSize); err != nil {
		writeError(w, http.StatusBadRequest, "file_too_large")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file_required")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_failed")
		return
	}
	if !detectMIME(data) {
		writeError(w, http.StatusBadRequest, "invalid_file_type")
		return
	}

	commercialName := r.FormValue("commercial_name")
	locationID := r.FormValue("location_id")
	if commercialName == "" || locationID == "" {
		writeError(w, http.StatusBadRequest, "commercial_name_and_location_id_required")
		return
	}

	var locID pgtype.UUID
	if err := locID.Scan(locationID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_location_id")
		return
	}

	// Read metadata fields
	title := r.FormValue("title")
	description := r.FormValue("description")
	format := r.FormValue("format")
	if format == "" {
		format = "landscape"
	}
	if format != "landscape" && format != "portrait" && format != "square" {
		writeError(w, http.StatusBadRequest, "invalid_format")
		return
	}
	shared := r.FormValue("shared") == "true"
	attribution := r.FormValue("attribution")

	result, err := ProcessProductImage(bytes.NewReader(data), format)
	if err != nil {
		slog.Error("failed to process product image", "error", err)
		writeError(w, http.StatusBadRequest, "image_processing_failed")
		return
	}

	if err := h.saveFiles(result); err != nil {
		slog.Error("failed to save image files", "error", err)
		writeError(w, http.StatusInternalServerError, "save_failed")
		return
	}

	// Insert product_images row with id = file_id so image_ids URLs work
	var fileID pgtype.UUID
	fileID.Scan(result.ID)

	piRow, err := h.Q.InsertProductImage(r.Context(), db.InsertProductImageParams{
		ID:              fileID,
		FileID:          fileID,
		GroupID:         claims.GroupID,
		UploadedBy:      claims.MemberID,
		Title:           title,
		Description:     description,
		Format:          format,
		Shared:          shared,
		Attribution:     attribution,
		IsReference:     false,
	})
	if err != nil {
		slog.Error("failed to insert product_images row", "error", err)
		h.deleteFiles(result.ID)
		writeError(w, http.StatusInternalServerError, "db_insert_failed")
		return
	}

	// Append the file_id to article image_ids (used for thumbnail URLs)
	fileIDStr := uuidToString(fileID)
	ids := h.getImageIds(r, claims.GroupID, commercialName, locID)
	ids = append(ids, fileIDStr)

	if err := h.setImageIds(r, claims.GroupID, commercialName, locID, ids); err != nil {
		slog.Error("failed to update image_ids", "error", err)
		h.Q.DeleteProductImage(r.Context(), db.DeleteProductImageParams{ID: piRow.ID, GroupID: claims.GroupID})
		h.deleteFiles(result.ID)
		writeError(w, http.StatusInternalServerError, "db_update_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"image":     imageResponseBasic(piRow),
		"image_ids": ids,
	})
}

func (h *Handler) DeleteProductImage(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	imageId := chi.URLParam(r, "imageId")
	if !validImageID.MatchString(imageId) {
		writeError(w, http.StatusBadRequest, "invalid_image_id")
		return
	}

	commercialName := r.URL.Query().Get("commercial_name")
	locationID := r.URL.Query().Get("location_id")
	if commercialName == "" || locationID == "" {
		writeError(w, http.StatusBadRequest, "commercial_name_and_location_id_required")
		return
	}

	var locID pgtype.UUID
	if err := locID.Scan(locationID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_location_id")
		return
	}

	// Get the product_images row to find file_id and check ownership
	var piID pgtype.UUID
	piID.Scan(imageId)
	piRow, err := h.Q.GetProductImage(r.Context(), piID)
	if err != nil {
		writeError(w, http.StatusNotFound, "image_not_found")
		return
	}

	// Only the uploader or an equipment manager can delete
	if piRow.UploadedBy != claims.MemberID && !claims.IsManager() {
		writeError(w, http.StatusForbidden, "can_only_delete_own_images")
		return
	}

	// Remove from article image_ids (image_ids stores file_id values)
	fileIDStr := uuidToString(piRow.FileID)
	ids := h.getImageIds(r, claims.GroupID, commercialName, locID)
	var newIds []string
	found := false
	for _, id := range ids {
		if id == fileIDStr {
			found = true
		} else {
			newIds = append(newIds, id)
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "image_not_found")
		return
	}
	if newIds == nil {
		newIds = []string{}
	}

	if err := h.setImageIds(r, claims.GroupID, commercialName, locID, newIds); err != nil {
		writeError(w, http.StatusInternalServerError, "db_update_failed")
		return
	}

	// Delete the product_images row
	h.Q.DeleteProductImage(r.Context(), db.DeleteProductImageParams{ID: piID, GroupID: claims.GroupID})

	// Only delete files if no other rows reference the same file_id
	refCount, err := h.Q.CountProductImagesByFileId(r.Context(), piRow.FileID)
	if err != nil || refCount == 0 {
		h.deleteFiles(uuidToString(piRow.FileID))
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ReorderProductImages(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	var req struct {
		CommercialName string   `json:"commercial_name"`
		LocationID     string   `json:"location_id"`
		ImageIds       []string `json:"image_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_body")
		return
	}
	if req.CommercialName == "" || req.LocationID == "" {
		writeError(w, http.StatusBadRequest, "commercial_name_and_location_id_required")
		return
	}

	var locID pgtype.UUID
	if err := locID.Scan(req.LocationID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_location_id")
		return
	}

	current := h.getImageIds(r, claims.GroupID, req.CommercialName, locID)
	currentSet := map[string]bool{}
	for _, id := range current {
		currentSet[id] = true
	}
	if len(req.ImageIds) != len(current) {
		writeError(w, http.StatusBadRequest, "image_count_mismatch")
		return
	}
	for _, id := range req.ImageIds {
		if !currentSet[id] {
			writeError(w, http.StatusBadRequest, "image_not_found")
			return
		}
	}

	if err := h.setImageIds(r, claims.GroupID, req.CommercialName, locID, req.ImageIds); err != nil {
		writeError(w, http.StatusInternalServerError, "db_update_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"image_ids": req.ImageIds})
}

func (h *Handler) AddFromShared(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	if !h.canUpload(r, claims) {
		writeError(w, http.StatusForbidden, "insufficient_upload_permission")
		return
	}

	var req struct {
		SourceImageID  string `json:"source_image_id"`
		CommercialName string `json:"commercial_name"`
		LocationID     string `json:"location_id"`
		Title          string `json:"title"`
		Description    string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_body")
		return
	}
	if req.SourceImageID == "" || req.CommercialName == "" || req.LocationID == "" {
		writeError(w, http.StatusBadRequest, "source_image_id_commercial_name_and_location_id_required")
		return
	}

	var locID pgtype.UUID
	if err := locID.Scan(req.LocationID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_location_id")
		return
	}

	// Look up the source image
	var sourceID pgtype.UUID
	sourceID.Scan(req.SourceImageID)
	source, err := h.Q.GetProductImage(r.Context(), sourceID)
	if err != nil {
		writeError(w, http.StatusNotFound, "source_image_not_found")
		return
	}

	// Source must be shared or from the same group
	if !source.Shared && source.GroupID != claims.GroupID {
		writeError(w, http.StatusForbidden, "image_not_shared")
		return
	}

	// Create a new row in the current group referencing the same file_id
	var newID pgtype.UUID
	newID.Scan(uuid.New().String())
	piRow, err := h.Q.InsertProductImage(r.Context(), db.InsertProductImageParams{
		ID:              newID,
		FileID:          source.FileID,
		GroupID:         claims.GroupID,
		UploadedBy:      claims.MemberID,
		Title:           req.Title,
		Description:     req.Description,
		Format:          source.Format,
		Shared:          false,
		Attribution:     source.Attribution,
		IsReference:     true,
	})
	if err != nil {
		slog.Error("failed to insert product_images row from shared", "error", err)
		writeError(w, http.StatusInternalServerError, "db_insert_failed")
		return
	}

	fileIDStr := uuidToString(source.FileID)
	ids := h.getImageIds(r, claims.GroupID, req.CommercialName, locID)
	ids = append(ids, fileIDStr)

	if err := h.setImageIds(r, claims.GroupID, req.CommercialName, locID, ids); err != nil {
		h.Q.DeleteProductImage(r.Context(), db.DeleteProductImageParams{ID: piRow.ID, GroupID: claims.GroupID})
		writeError(w, http.StatusInternalServerError, "db_update_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"image":     imageResponseBasic(piRow),
		"image_ids": ids,
	})
}

func (h *Handler) ListShared(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	search := pgtype.Text{}
	if s := r.URL.Query().Get("search"); s != "" {
		search = pgtype.Text{String: s, Valid: true}
	}

	rows, err := h.Q.ListSharedImages(r.Context(), db.ListSharedImagesParams{
		GroupID: claims.GroupID,
		Search:  search,
	})
	if err != nil {
		slog.Error("failed to list shared images", "error", err)
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}

	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		result = append(result, map[string]any{
			"id":          uuidToString(row.ID),
			"file_id":     uuidToString(row.FileID),
			"title":       row.Title,
			"description": row.Description,
			"format":      row.Format,
			"shared":      row.Shared,
			"attribution": row.Attribution,
			"created_at":  row.CreatedAt.Time,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) ListProductImages(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	commercialName := r.URL.Query().Get("commercial_name")
	locationID := r.URL.Query().Get("location_id")
	if commercialName == "" || locationID == "" {
		writeError(w, http.StatusBadRequest, "commercial_name_and_location_id_required")
		return
	}

	var locID pgtype.UUID
	if err := locID.Scan(locationID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_location_id")
		return
	}

	ids := h.getImageIds(r, claims.GroupID, commercialName, locID)
	if len(ids) == 0 {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	uuids := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		uuids[i].Scan(id)
	}

	rows, err := h.Q.ListProductImagesByIds(r.Context(), uuids)
	if err != nil {
		slog.Error("failed to list product images by ids", "error", err)
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}

	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		result = append(result, map[string]any{
			"id":          uuidToString(row.ID),
			"file_id":     uuidToString(row.FileID),
			"title":       row.Title,
			"description": row.Description,
			"format":      row.Format,
			"shared":      row.Shared,
			"attribution": row.Attribution,
			"uploaded_by": row.UploadedBy,
			"created_at":  row.CreatedAt.Time,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) GetProductImageMeta(w http.ResponseWriter, r *http.Request) {
	imageId := chi.URLParam(r, "imageId")
	if !validImageID.MatchString(imageId) {
		writeError(w, http.StatusBadRequest, "invalid_image_id")
		return
	}

	var piID pgtype.UUID
	piID.Scan(imageId)
	row, err := h.Q.GetProductImage(r.Context(), piID)
	if err != nil {
		writeError(w, http.StatusNotFound, "image_not_found")
		return
	}

	refCount, _ := h.Q.CountProductImagesByFileId(r.Context(), row.FileID)
	writeJSON(w, http.StatusOK, map[string]any{
		"id":          uuidToString(row.ID),
		"file_id":     uuidToString(row.FileID),
		"title":       row.Title,
		"description": row.Description,
		"format":      row.Format,
		"shared":      row.Shared,
		"attribution": row.Attribution,
		"created_at":  row.CreatedAt.Time,
		"ref_count":   refCount,
	})
}

func (h *Handler) UpdateProductImageMeta(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	imageId := chi.URLParam(r, "imageId")
	if !validImageID.MatchString(imageId) {
		writeError(w, http.StatusBadRequest, "invalid_image_id")
		return
	}

	var piID pgtype.UUID
	piID.Scan(imageId)
	existing, err := h.Q.GetProductImage(r.Context(), piID)
	if err != nil {
		writeError(w, http.StatusNotFound, "image_not_found")
		return
	}

	if existing.UploadedBy != claims.MemberID && !claims.IsManager() {
		writeError(w, http.StatusForbidden, "can_only_edit_own_images")
		return
	}

	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Shared      bool   `json:"shared"`
		Attribution string `json:"attribution"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_body")
		return
	}

	updated, err := h.Q.UpdateProductImage(r.Context(), db.UpdateProductImageParams{
		Title:       req.Title,
		Description: req.Description,
		Shared:      req.Shared,
		Attribution: req.Attribution,
		ID:          piID,
		GroupID:     claims.GroupID,
	})
	if err != nil {
		slog.Error("failed to update product image", "error", err)
		writeError(w, http.StatusInternalServerError, "update_failed")
		return
	}

	writeJSON(w, http.StatusOK, imageResponseBasic(updated))
}

func (h *Handler) ListMyImages(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	rows, err := h.Q.ListProductImagesByUploader(r.Context(), db.ListProductImagesByUploaderParams{
		UserID:  claims.MemberID,
		GroupID: claims.GroupID,
	})
	if err != nil {
		slog.Error("failed to list user images", "error", err)
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}

	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		result = append(result, map[string]any{
			"id":                uuidToString(row.ID),
			"file_id":           uuidToString(row.FileID),
			"title":             row.Title,
			"description":       row.Description,
			"format":            row.Format,
			"shared":            row.Shared,
			"attribution":       row.Attribution,
			"created_at":        row.CreatedAt.Time,
			"own_group_count":   row.OwnGroupCount,
			"other_group_count": row.OtherGroupCount,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) ListArticlesUsingImage(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	imageId := chi.URLParam(r, "imageId")
	if !validImageID.MatchString(imageId) {
		writeError(w, http.StatusBadRequest, "invalid_image_id")
		return
	}

	// Look up file_id since image_ids stores file_id values
	var piID pgtype.UUID
	piID.Scan(imageId)
	piRow, err := h.Q.GetProductImage(r.Context(), piID)
	if err != nil {
		writeError(w, http.StatusNotFound, "image_not_found")
		return
	}

	rows, err := h.Q.ListArticlesUsingImage(r.Context(), db.ListArticlesUsingImageParams{
		GroupID:    claims.GroupID,
		ImageIDStr: uuidToString(piRow.FileID),
	})
	if err != nil {
		slog.Error("failed to list articles using image", "error", err)
		writeError(w, http.StatusInternalServerError, "list_failed")
		return
	}

	// Deduplicate by commercial_name + location to show article groups, not individual articles
	type group struct {
		CommercialName string `json:"commercial_name"`
		LocationName   string `json:"location_name"`
		ArticleID      string `json:"article_id"`
	}
	seen := map[string]bool{}
	result := []group{}
	for _, row := range rows {
		key := row.CommercialName + "|" + row.LocationName
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, group{
			CommercialName: row.CommercialName,
			LocationName:   row.LocationName,
			ArticleID:      uuidToString(row.ID),
		})
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) DeleteMyImage(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	imageId := chi.URLParam(r, "imageId")
	if !validImageID.MatchString(imageId) {
		writeError(w, http.StatusBadRequest, "invalid_image_id")
		return
	}

	var piID pgtype.UUID
	piID.Scan(imageId)
	piRow, err := h.Q.GetProductImage(r.Context(), piID)
	if err != nil {
		writeError(w, http.StatusNotFound, "image_not_found")
		return
	}

	if piRow.UploadedBy != claims.MemberID && !claims.IsManager() {
		writeError(w, http.StatusForbidden, "can_only_delete_own_images")
		return
	}

	// Remove from all articles' image_ids in this group (image_ids stores file_id)
	fileIDStr, _ := json.Marshal(uuidToString(piRow.FileID))
	h.Q.RemoveImageIdFromAllArticles(r.Context(), db.RemoveImageIdFromAllArticlesParams{
		ImageIDStr: fileIDStr,
		GroupID:    claims.GroupID,
	})

	// Delete the product_images row
	h.Q.DeleteProductImage(r.Context(), db.DeleteProductImageParams{ID: piID, GroupID: claims.GroupID})

	// Only delete files if no other rows reference the same file_id
	refCount, err := h.Q.CountProductImagesByFileId(r.Context(), piRow.FileID)
	if err != nil || refCount == 0 {
		h.deleteFiles(uuidToString(piRow.FileID))
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UploadIssue(w http.ResponseWriter, r *http.Request) {

	r.Body = http.MaxBytesReader(w, r.Body, MaxUploadSize+1024)
	if err := r.ParseMultipartForm(MaxUploadSize); err != nil {
		writeError(w, http.StatusBadRequest, "file_too_large")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file_required")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_failed")
		return
	}
	if !detectMIME(data) {
		writeError(w, http.StatusBadRequest, "invalid_file_type")
		return
	}

	result, err := ProcessIssueImage(bytes.NewReader(data))
	if err != nil {
		slog.Error("failed to process issue image", "error", err)
		writeError(w, http.StatusBadRequest, "image_processing_failed")
		return
	}

	if err := h.saveFiles(result); err != nil {
		slog.Error("failed to save image files", "error", err)
		writeError(w, http.StatusInternalServerError, "save_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"image_id": result.ID})
}

func (h *Handler) Serve(w http.ResponseWriter, r *http.Request) {
	filename := chi.URLParam(r, "filename")

	name := strings.TrimSuffix(filename, ".webp")
	if name == filename {
		http.NotFound(w, r)
		return
	}

	id := strings.TrimSuffix(name, "_thumb")
	if !validImageID.MatchString(id) {
		http.NotFound(w, r)
		return
	}

	path := filepath.Join(h.ImageDir, filename)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		http.NotFound(w, r)
		return
	}

	if r.URL.Query().Get("format") == "jpeg" {
		data, err := os.ReadFile(path)
		if err != nil {
			http.Error(w, "read error", http.StatusInternalServerError)
			return
		}
		jpegData, err := ConvertToJPEG(data)
		if err != nil {
			slog.Error("failed to convert to jpeg", "error", err)
			http.Error(w, "conversion error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+id+".jpg\"")
		w.Write(jpegData)
		return
	}

	w.Header().Set("Content-Type", "image/webp")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, path)
}

// getImageIds reads the current image_ids array for an article group.
func (h *Handler) getImageIds(r *http.Request, groupID, commercialName string, locationID pgtype.UUID) []string {
	raw, err := h.Q.GetArticleGroupImageIds(r.Context(), db.GetArticleGroupImageIdsParams{
		GroupID: groupID, CommercialName: commercialName, LocationID: locationID,
	})
	if err != nil {
		return []string{}
	}
	var ids []string
	if err := json.Unmarshal(raw, &ids); err != nil {
		return []string{}
	}
	return ids
}

// setImageIds writes the image_ids array to all articles in the group.
func (h *Handler) setImageIds(r *http.Request, groupID, commercialName string, locationID pgtype.UUID, ids []string) error {
	data, _ := json.Marshal(ids)
	_, err := h.Q.UpdateArticleGroupImageIds(r.Context(), db.UpdateArticleGroupImageIdsParams{
		ImageIds: data, GroupID: groupID, CommercialName: commercialName, LocationID: locationID,
	})
	return err
}

func (h *Handler) saveFiles(result *ProcessResult) error {
	if err := os.MkdirAll(h.ImageDir, 0755); err != nil {
		return err
	}
	sourcePath := filepath.Join(h.ImageDir, result.ID+".webp")
	if err := os.WriteFile(sourcePath, result.Source, 0644); err != nil {
		return err
	}
	thumbPath := filepath.Join(h.ImageDir, result.ID+"_thumb.webp")
	if err := os.WriteFile(thumbPath, result.Thumbnail, 0644); err != nil {
		os.Remove(sourcePath)
		return err
	}
	return nil
}

func (h *Handler) deleteFiles(fileID string) {
	os.Remove(filepath.Join(h.ImageDir, fileID+".webp"))
	os.Remove(filepath.Join(h.ImageDir, fileID+"_thumb.webp"))
}

func detectMIME(data []byte) bool {
	ct := http.DetectContentType(data)
	switch {
	case strings.HasPrefix(ct, "image/jpeg"),
		strings.HasPrefix(ct, "image/png"),
		strings.HasPrefix(ct, "image/webp"):
		return true
	}
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return true
	}
	if len(data) >= 12 && string(data[4:8]) == "ftyp" {
		brand := string(data[8:12])
		switch brand {
		case "heic", "heix", "mif1":
			return true
		}
	}
	return false
}

// imageResponseBasic returns image metadata without resolved attribution (no group name join available).
func imageResponseBasic(pi db.ProductImage) map[string]any {
	return map[string]any{
		"id":          uuidToString(pi.ID),
		"file_id":     uuidToString(pi.FileID),
		"title":       pi.Title,
		"description": pi.Description,
		"format":      pi.Format,
		"shared":      pi.Shared,
		"created_at":  pi.CreatedAt.Time,
	}
}

func uuidToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuid.UUID(u.Bytes).String()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
