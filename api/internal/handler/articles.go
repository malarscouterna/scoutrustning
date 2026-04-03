package handler

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/malarscouterna/ms-utrustning/api/internal/auth"
	"github.com/malarscouterna/ms-utrustning/api/internal/db"
)

type ArticleHandler struct {
	Q *db.Queries
}

func (h *ArticleHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Get("/{id}", h.Get)
	r.With(auth.RequireRole("equipment_manager")).Post("/", h.Create)
	r.With(auth.RequireRole("equipment_manager")).Put("/{id}", h.Update)
	r.With(auth.RequireRole("equipment_manager")).Delete("/{id}", h.Delete)
	r.With(auth.RequireRole("equipment_manager")).Post("/import", h.Import)
	return r
}

func (h *ArticleHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	articles, err := h.Q.ListArticles(r.Context(), claims.GroupID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to list articles")
		return
	}
	WriteJSON(w, http.StatusOK, articles)
}

func (h *ArticleHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}
	article, err := h.Q.GetArticle(r.Context(), db.GetArticleParams{ID: id, GroupID: claims.GroupID})
	if err != nil {
		WriteError(w, http.StatusNotFound, "article not found")
		return
	}
	WriteJSON(w, http.StatusOK, article)
}

type articleRequest struct {
	CommercialName      string  `json:"commercial_name"`
	CommonName          string  `json:"common_name"`
	CategoryID          string  `json:"category_id"`
	LocationID          string  `json:"location_id"`
	Status              string  `json:"status"`
	IndividuallyTracked bool    `json:"individually_tracked"`
	RequiresApproval    bool    `json:"requires_approval"`
	Description         string  `json:"description"`
	Instructions        string  `json:"instructions"`
	Place               string  `json:"place"`
}

func (h *ArticleHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	var req articleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.CommonName == "" || req.CategoryID == "" || req.LocationID == "" {
		WriteError(w, http.StatusBadRequest, "common_name, category_id, and location_id are required")
		return
	}
	catID, err := parseUUID(req.CategoryID)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid category_id")
		return
	}
	locID, err := parseUUID(req.LocationID)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid location_id")
		return
	}
	status := req.Status
	if status == "" {
		status = "ok"
	}
	article, err := h.Q.CreateArticle(r.Context(), db.CreateArticleParams{
		GroupID:             claims.GroupID,
		CommercialName:      req.CommercialName,
		CommonName:          req.CommonName,
		CategoryID:          catID,
		LocationID:          locID,
		Status:              status,
		IndividuallyTracked: req.IndividuallyTracked,
		RequiresApproval:    req.RequiresApproval,
		Description:         req.Description,
		Instructions:        req.Instructions,
		Place:               req.Place,
	})
	if err != nil {
		slog.Error("failed to create article", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to create article")
		return
	}
	WriteJSON(w, http.StatusCreated, article)
}

func (h *ArticleHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req articleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	catID, err := parseUUID(req.CategoryID)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid category_id")
		return
	}
	locID, err := parseUUID(req.LocationID)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid location_id")
		return
	}
	article, err := h.Q.UpdateArticle(r.Context(), db.UpdateArticleParams{
		ID: id, GroupID: claims.GroupID,
		CommercialName:      req.CommercialName,
		CommonName:          req.CommonName,
		CategoryID:          catID,
		LocationID:          locID,
		Status:              req.Status,
		IndividuallyTracked: req.IndividuallyTracked,
		RequiresApproval:    req.RequiresApproval,
		Description:         req.Description,
		Instructions:        req.Instructions,
		Place:               req.Place,
	})
	if err != nil {
		WriteError(w, http.StatusNotFound, "article not found")
		return
	}
	WriteJSON(w, http.StatusOK, article)
}

func (h *ArticleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.Q.DeleteArticle(r.Context(), db.DeleteArticleParams{ID: id, GroupID: claims.GroupID}); err != nil {
		WriteError(w, http.StatusNotFound, "article not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Import handles CSV upload matching the Mälarscouterna inventory spreadsheet format.
// Auto-creates categories and locations that don't exist.
func (h *ArticleHandler) Import(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	ctx := r.Context()

	file, _, err := r.FormFile("file")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "file field required")
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	// Skip header
	if _, err := reader.Read(); err != nil {
		WriteError(w, http.StatusBadRequest, "failed to read CSV header")
		return
	}

	// Cache lookups for locations and categories
	locationCache := map[string]pgtype.UUID{}
	categoryCache := map[string]pgtype.UUID{}

	// Pre-load existing locations and categories
	locations, _ := h.Q.ListLocations(ctx, claims.GroupID)
	for _, l := range locations {
		locationCache[l.Name] = l.ID
	}
	categories, _ := h.Q.ListCategories(ctx, claims.GroupID)
	for _, c := range categories {
		categoryCache[c.Name] = c.ID
	}

	resolveLocation := func(name string) (pgtype.UUID, error) {
		if id, ok := locationCache[name]; ok {
			return id, nil
		}
		loc, err := h.Q.CreateLocation(ctx, db.CreateLocationParams{
			GroupID: claims.GroupID, Name: name, SortOrder: int32(len(locationCache) + 1),
		})
		if err != nil {
			return pgtype.UUID{}, err
		}
		locationCache[name] = loc.ID
		return loc.ID, nil
	}

	resolveCategory := func(name string) (pgtype.UUID, error) {
		if id, ok := categoryCache[name]; ok {
			return id, nil
		}
		cat, err := h.Q.CreateCategory(ctx, db.CreateCategoryParams{
			GroupID: claims.GroupID, Name: name, SortOrder: int32(len(categoryCache) + 1),
		})
		if err != nil {
			return pgtype.UUID{}, err
		}
		categoryCache[name] = cat.ID
		return cat.ID, nil
	}

	var imported, skipped int
	var errors []string

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errors = append(errors, fmt.Sprintf("row %d: %v", imported+skipped+2, err))
			skipped++
			continue
		}

		// CSV columns: 0=titelgrupp, 1=title, 2=description, 3=location, 4=plats, 5=rum, 6=lage, 7=tags, ...
		if len(record) < 8 {
			errors = append(errors, fmt.Sprintf("row %d: too few columns", imported+skipped+2))
			skipped++
			continue
		}

		commonName := strings.TrimSpace(record[1])
		if commonName == "" {
			skipped++
			continue
		}

		description := strings.TrimSpace(record[2])
		rawLocation := strings.TrimSpace(record[3])
		plats := strings.TrimSpace(record[4])
		rum := strings.TrimSpace(record[5])
		lage := strings.TrimSpace(record[6])
		tag := strings.TrimSpace(record[7])

		// Resolve location: Karsvik items use plats as the real location
		locationName := rawLocation
		if strings.EqualFold(rawLocation, "Karsvik") && plats != "" {
			locationName = normalizeKarsvikPlats(plats)
		}
		if locationName == "" {
			locationName = "Övrigt"
		}

		locID, err := resolveLocation(locationName)
		if err != nil {
			errors = append(errors, fmt.Sprintf("row %d: location %q: %v", imported+skipped+2, locationName, err))
			skipped++
			continue
		}

		// Resolve category from tag
		categoryName := normalizeCategory(tag)
		if categoryName == "" {
			categoryName = "Övrigt"
		}
		catID, err := resolveCategory(categoryName)
		if err != nil {
			errors = append(errors, fmt.Sprintf("row %d: category %q: %v", imported+skipped+2, categoryName, err))
			skipped++
			continue
		}

		// Build place from rum + lage
		var placeParts []string
		if rum != "" {
			placeParts = append(placeParts, rum)
		}
		if lage != "" {
			placeParts = append(placeParts, lage)
		}
		place := strings.Join(placeParts, ", ")

		commercialName := strings.TrimSpace(record[0])

		_, err = h.Q.CreateArticle(ctx, db.CreateArticleParams{
			GroupID:             claims.GroupID,
			CommercialName:      commercialName,
			CommonName:          commonName,
			CategoryID:          catID,
			LocationID:          locID,
			Status:              "ok",
			IndividuallyTracked: true,
			Description:         description,
			Place:               place,
		})
		if err != nil {
			errors = append(errors, fmt.Sprintf("row %d: %v", imported+skipped+2, err))
			skipped++
			continue
		}
		imported++
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"imported": imported,
		"skipped":  skipped,
		"errors":   errors,
	})
}

func normalizeKarsvikPlats(plats string) string {
	switch strings.ToLower(plats) {
	case "ladan":
		return "Ladan"
	case "ostergarden":
		return "Östergården"
	case "kallforradet":
		return "Kallförrådet"
	default:
		return plats
	}
}

func normalizeCategory(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return ""
	}
	// Capitalize first letter, lowercase rest
	return strings.ToUpper(tag[:1]) + strings.ToLower(tag[1:])
}
