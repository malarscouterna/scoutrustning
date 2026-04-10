package handler

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

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
	r.Get("/availability", h.Availability)
	r.Get("/availability/articles", h.AvailableArticlesList)
	r.Get("/{id}", h.Get)
	r.Get("/{id}/events", h.ListEvents)
	r.Get("/{id}/group-events", h.ListGroupEvents)
	r.Post("/{id}/events", h.AddNote)
	r.Put("/{id}/status", h.UpdateStatus)
	r.With(auth.RequireRole("equipment_manager")).Post("/", h.Create)
	r.With(auth.RequireRole("equipment_manager")).Put("/{id}", h.Update)
	r.With(auth.RequireRole("equipment_manager")).Delete("/{id}", h.Delete)
	r.With(auth.RequireRole("equipment_manager")).Post("/import", h.Import)
	r.With(auth.RequireRole("equipment_manager")).Put("/bulk", h.BulkUpdate)
	r.With(auth.RequireRole("equipment_manager")).Post("/group-count", h.GroupCount)
	return r
}

func (h *ArticleHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	// mine=true: only articles linked to user's bookings
	if r.URL.Query().Get("mine") == "true" {
		var statuses []string
		if v := r.URL.Query().Get("status"); v != "" {
			statuses = strings.Split(v, ",")
		}
		articles, err := h.Q.ListArticlesByUserBookings(r.Context(), db.ListArticlesByUserBookingsParams{
			GroupID:   claims.GroupID,
			Statuses:  statuses,
			UserID:    claims.MemberID,
			UnitNames: claims.Units,
		})
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to list articles")
			return
		}
		WriteJSON(w, http.StatusOK, articles)
		return
	}

	var statuses []string
	if v := r.URL.Query().Get("status"); v != "" {
		statuses = strings.Split(v, ",")
	}

	var categoryID pgtype.UUID
	if v := r.URL.Query().Get("category_id"); v != "" {
		id, err := parseUUID(v)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid category_id")
			return
		}
		categoryID = id
	}

	var locationID pgtype.UUID
	if v := r.URL.Query().Get("location_id"); v != "" {
		id, err := parseUUID(v)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid location_id")
			return
		}
		locationID = id
	}

	var search pgtype.Text
	if v := r.URL.Query().Get("search"); v != "" {
		search = pgtype.Text{String: v, Valid: true}
	}

	// with_availability=true: return articles enriched with current booking context
	if r.URL.Query().Get("with_availability") == "true" {
		asOfDate := pgtype.Date{Time: time.Now(), Valid: true}
		if v := r.URL.Query().Get("date"); v != "" {
			t, err := time.Parse("2006-01-02", v)
			if err != nil {
				WriteError(w, http.StatusBadRequest, "invalid date")
				return
			}
			asOfDate = pgtype.Date{Time: t, Valid: true}
		}
		articles, err := h.Q.ListArticlesWithAvailability(r.Context(), db.ListArticlesWithAvailabilityParams{
			GroupID:    claims.GroupID,
			AsOfDate:   asOfDate,
			CategoryID: categoryID,
			LocationID: locationID,
			Statuses:   statuses,
			Search:     search,
		})
		if err != nil {
			slog.Error("failed to list articles with availability", "error", err)
			WriteError(w, http.StatusInternalServerError, "failed to list articles")
			return
		}
		WriteJSON(w, http.StatusOK, articles)
		return
	}

	articles, err := h.Q.ListArticles(r.Context(), db.ListArticlesParams{
		GroupID:    claims.GroupID,
		Statuses:   statuses,
		CategoryID: categoryID,
		LocationID: locationID,
		Search:     search,
	})
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
	ApprovalLevel       string  `json:"approval_level"`
	Description         string  `json:"description"`
	Instructions        string  `json:"instructions"`
	Place               string  `json:"place"`
	PurchaseDate        *string `json:"purchase_date"`
	PurchasePrice       *string `json:"purchase_price"`
	ManagerNotes        string  `json:"manager_notes"`
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
	approvalLevel := req.ApprovalLevel
	if approvalLevel == "" {
		approvalLevel = "none"
	}
	var purchaseDate pgtype.Date
	if req.PurchaseDate != nil && *req.PurchaseDate != "" {
		t, err := time.Parse("2006-01-02", *req.PurchaseDate)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid purchase_date")
			return
		}
		purchaseDate = pgtype.Date{Time: t, Valid: true}
	}
	var purchasePrice pgtype.Numeric
	if req.PurchasePrice != nil && *req.PurchasePrice != "" {
		if err := purchasePrice.Scan(*req.PurchasePrice); err != nil {
			WriteError(w, http.StatusBadRequest, "invalid purchase_price")
			return
		}
	}
	article, err := h.Q.CreateArticle(r.Context(), db.CreateArticleParams{
		GroupID:             claims.GroupID,
		CommercialName:      req.CommercialName,
		CommonName:          req.CommonName,
		CategoryID:          catID,
		LocationID:          locID,
		Status:              status,
		IndividuallyTracked: req.IndividuallyTracked,
		ApprovalLevel:       approvalLevel,
		Description:         req.Description,
		Instructions:        req.Instructions,
		Place:               req.Place,
		PurchaseDate:        purchaseDate,
		PurchasePrice:       purchasePrice,
		ManagerNotes:        req.ManagerNotes,
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
	var purchaseDate pgtype.Date
	if req.PurchaseDate != nil && *req.PurchaseDate != "" {
		t, err := time.Parse("2006-01-02", *req.PurchaseDate)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid purchase_date")
			return
		}
		purchaseDate = pgtype.Date{Time: t, Valid: true}
	}
	var purchasePrice pgtype.Numeric
	if req.PurchasePrice != nil && *req.PurchasePrice != "" {
		if err := purchasePrice.Scan(*req.PurchasePrice); err != nil {
			WriteError(w, http.StatusBadRequest, "invalid purchase_price")
			return
		}
	}

	// group=true: apply shared fields to all articles in the quantity tracked group
	if r.URL.Query().Get("group") == "true" {
		existing, err := h.Q.GetArticle(r.Context(), db.GetArticleParams{ID: id, GroupID: claims.GroupID})
		if err != nil {
			WriteError(w, http.StatusNotFound, "article not found")
			return
		}
		_, err = h.Q.UpdateArticleGroupFields(r.Context(), db.UpdateArticleGroupFieldsParams{
			GroupID:            claims.GroupID,
			OldCommercialName:  existing.CommercialName,
			OldLocationID:      existing.LocationID,
			NewCommercialName:  req.CommercialName,
			NewCommonName:      req.CommonName,
			CategoryID:         catID,
			NewLocationID:      locID,
			ApprovalLevel:      req.ApprovalLevel,
			Description:        req.Description,
			Instructions:       req.Instructions,
			Place:              req.Place,
			ManagerNotes:       req.ManagerNotes,
			IndividuallyTracked: req.IndividuallyTracked,
		})
		if err != nil {
			slog.Error("failed to update article group", "error", err)
			WriteError(w, http.StatusInternalServerError, "failed to update group")
			return
		}
		updated, err := h.Q.GetArticle(r.Context(), db.GetArticleParams{ID: id, GroupID: claims.GroupID})
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to read updated article")
			return
		}
		WriteJSON(w, http.StatusOK, updated)
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
		ApprovalLevel:       req.ApprovalLevel,
		Description:         req.Description,
		Instructions:        req.Instructions,
		Place:               req.Place,
		PurchaseDate:        purchaseDate,
		PurchasePrice:       purchasePrice,
		ManagerNotes:        req.ManagerNotes,
	})
	if err != nil {
		WriteError(w, http.StatusNotFound, "article not found")
		return
	}

	// Auto-propagate shared fields to siblings (same commercial_name across all locations)
	if article.IndividuallyTracked && article.CommercialName != "" {
		h.Q.PropagateSharedFields(r.Context(), db.PropagateSharedFieldsParams{
			GroupID:        claims.GroupID,
			CommercialName: article.CommercialName,
			ExcludeID:      article.ID,
			Description:    article.Description,
			Instructions:   article.Instructions,
			ManagerNotes:   article.ManagerNotes,
			CategoryID:     article.CategoryID,
		})
	}

	WriteJSON(w, http.StatusOK, article)
}

// Availability returns available article counts grouped by commercial_name for a date range.
func (h *ArticleHandler) Availability(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	startStr := r.URL.Query().Get("start_date")
	endStr := r.URL.Query().Get("end_date")
	if startStr == "" || endStr == "" {
		WriteError(w, http.StatusBadRequest, "start_date and end_date required")
		return
	}
	startDate, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid start_date")
		return
	}
	endDate, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid end_date")
		return
	}

	available, err := h.Q.AvailableArticles(r.Context(), db.AvailableArticlesParams{
		GroupID:   claims.GroupID,
		StartDate: pgtype.Date{Time: startDate, Valid: true},
		EndDate:   pgtype.Date{Time: endDate, Valid: true},
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to check availability")
		return
	}

	// Optional filters
	categoryFilter := r.URL.Query().Get("category_id")
	locationFilter := r.URL.Query().Get("location_id")
	bookableOnly := r.URL.Query().Get("bookable_only") != "false" // default true

	// Group by commercial_name + location
	type availGroup struct {
		CommercialName      string `json:"commercial_name"`
		AvailableCount      int    `json:"available_count"`
		ReportedUsableCount int    `json:"reported_usable_count"`
		IncomingCount       int    `json:"incoming_count"`
		UnderRepairCount    int    `json:"under_repair_count"`
		ApprovalLevel       string `json:"approval_level"`
		CategoryName        string `json:"category_name"`
		LocationName        string `json:"location_name"`
	}
	type groupKey struct {
		name     string
		location string
	}
	groups := map[groupKey]*availGroup{}
	for _, a := range available {
		if bookableOnly && a.ApprovalLevel != "none" {
			continue
		}
		if categoryFilter != "" {
			var catUUID pgtype.UUID
			catUUID.Scan(categoryFilter)
			if a.CategoryID != catUUID {
				continue
			}
		}
		if locationFilter != "" {
			var locUUID pgtype.UUID
			locUUID.Scan(locationFilter)
			if a.LocationID != locUUID {
				continue
			}
		}
		key := groupKey{a.CommercialName, a.LocationName}
		g, ok := groups[key]
		if !ok {
			g = &availGroup{
				CommercialName:   a.CommercialName,
				ApprovalLevel:   a.ApprovalLevel,
				CategoryName:     a.CategoryName,
				LocationName:     a.LocationName,
			}
			groups[key] = g
		}
		g.AvailableCount++
		switch a.Status {
		case "reported_usable":
			g.ReportedUsableCount++
		case "incoming":
			g.IncomingCount++
		case "under_repair":
			g.UnderRepairCount++
		}
	}

	result := make([]availGroup, 0, len(groups))
	for _, g := range groups {
		result = append(result, *g)
	}
	// Sort alphabetically
	sort.Slice(result, func(i, j int) bool {
		if result[i].CommercialName != result[j].CommercialName {
			return result[i].CommercialName < result[j].CommercialName
		}
		return result[i].LocationName < result[j].LocationName
	})
	WriteJSON(w, http.StatusOK, result)
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

func (h *ArticleHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 1 {
			WriteError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		// Fetch limit+1 to detect if there are more
		events, err := h.Q.ListArticleEventsLimited(r.Context(), db.ListArticleEventsLimitedParams{
			ArticleID: id, GroupID: claims.GroupID, MaxResults: int32(limit + 1),
		})
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to list events")
			return
		}
		hasMore := len(events) > limit
		if hasMore {
			events = events[:limit]
		}
		WriteJSON(w, http.StatusOK, map[string]any{"events": events, "has_more": hasMore})
		return
	}

	events, err := h.Q.ListArticleEvents(r.Context(), db.ListArticleEventsParams{ArticleID: id, GroupID: claims.GroupID})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to list events")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"events": events, "has_more": false})
}

// ListGroupEvents returns events for all articles in a quantity tracked group.
func (h *ArticleHandler) ListGroupEvents(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}

	// Look up the article to get commercial_name + location_id
	article, err := h.Q.GetArticle(r.Context(), db.GetArticleParams{ID: id, GroupID: claims.GroupID})
	if err != nil {
		WriteError(w, http.StatusNotFound, "article not found")
		return
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 1 {
			WriteError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		events, err := h.Q.ListArticleEventsByGroupLimited(r.Context(), db.ListArticleEventsByGroupLimitedParams{
			GroupID: claims.GroupID, CommercialName: article.CommercialName,
			LocationID: article.LocationID, MaxResults: int32(limit + 1),
		})
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to list events")
			return
		}
		hasMore := len(events) > limit
		if hasMore {
			events = events[:limit]
		}
		WriteJSON(w, http.StatusOK, map[string]any{"events": events, "has_more": hasMore})
		return
	}

	events, err := h.Q.ListArticleEventsByGroup(r.Context(), db.ListArticleEventsByGroupParams{
		GroupID: claims.GroupID, CommercialName: article.CommercialName,
		LocationID: article.LocationID,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to list events")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"events": events, "has_more": false})
}

// AddNote adds a note event to an article's history.
func (h *ArticleHandler) AddNote(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Message == "" {
		WriteError(w, http.StatusBadRequest, "message required")
		return
	}
	// Verify article exists in group
	if _, err := h.Q.GetArticle(r.Context(), db.GetArticleParams{ID: id, GroupID: claims.GroupID}); err != nil {
		WriteError(w, http.StatusNotFound, "article not found")
		return
	}
	LogArticleEvent(r.Context(), h.Q, claims, id, "note", req.Message, nil)
	w.WriteHeader(http.StatusNoContent)
}

// UpdateStatus changes article status with an optional comment.
// Any user can set reported statuses (reported_usable, reported_unusable, lost).
// Manager-only statuses (ok, under_repair, archived, etc.) require equipment_manager role.
func (h *ArticleHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req struct {
		Status                string  `json:"status"`
		Comment               string  `json:"comment"`
		ExpectedAvailableDate *string `json:"expected_available_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Status == "" {
		WriteError(w, http.StatusBadRequest, "status required")
		return
	}

	validStatuses := map[string]bool{
		"ok": true, "reported_usable": true, "incoming": true,
		"reported_unusable": true, "under_repair": true, "lost": true, "archived": true,
	}
	if !validStatuses[req.Status] {
		WriteError(w, http.StatusBadRequest, "invalid status")
		return
	}

	// Anyone can report issues; other statuses require manager
	userStatuses := map[string]bool{"reported_usable": true, "reported_unusable": true, "lost": true}
	if !userStatuses[req.Status] && !claims.HasRole("equipment_manager") {
		WriteError(w, http.StatusForbidden, "forbidden")
		return
	}

	// Reporting requires a comment
	if userStatuses[req.Status] && req.Comment == "" {
		WriteError(w, http.StatusBadRequest, "comment required when reporting an issue")
		return
	}

	// expected_available_date only valid for incoming and under_repair
	var expectedDate pgtype.Date
	if req.ExpectedAvailableDate != nil {
		if req.Status != "incoming" && req.Status != "under_repair" {
			WriteError(w, http.StatusBadRequest, "expected_available_date only valid for incoming and under_repair")
			return
		}
		t, err := time.Parse("2006-01-02", *req.ExpectedAvailableDate)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid expected_available_date")
			return
		}
		expectedDate = pgtype.Date{Time: t, Valid: true}
	}

	article, err := h.Q.GetArticle(r.Context(), db.GetArticleParams{ID: id, GroupID: claims.GroupID})
	if err != nil {
		WriteError(w, http.StatusNotFound, "article not found")
		return
	}

	updated, err := h.Q.UpdateArticleStatus(r.Context(), db.UpdateArticleStatusParams{
		ID: id, GroupID: claims.GroupID, Status: req.Status,
		ExpectedAvailableDate: expectedDate,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to update article status")
		return
	}

	eventType := "status_change"
	if userStatuses[req.Status] {
		eventType = "issue_reported"
	} else if req.Status == "ok" && article.Status != "ok" {
		eventType = "issue_resolved"
	}

	LogArticleEvent(r.Context(), h.Q, claims, id, eventType, req.Comment, map[string]string{
		"old_status": article.Status,
		"new_status": req.Status,
	})

	WriteJSON(w, http.StatusOK, updated)
}

// AvailableArticlesList returns individual available articles for a date range,
// optionally excluding a specific booking (for swap scenarios).
func (h *ArticleHandler) AvailableArticlesList(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	startStr := r.URL.Query().Get("start_date")
	endStr := r.URL.Query().Get("end_date")
	if startStr == "" || endStr == "" {
		WriteError(w, http.StatusBadRequest, "start_date and end_date required")
		return
	}
	startDate, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid start_date")
		return
	}
	endDate, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid end_date")
		return
	}

	excludeBooking := r.URL.Query().Get("exclude_booking_id")
	commercialName := r.URL.Query().Get("commercial_name")

	if excludeBooking != "" {
		bid, err := parseUUID(excludeBooking)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid exclude_booking_id")
			return
		}
		articles, err := h.Q.AvailableArticlesExcludingBooking(r.Context(), db.AvailableArticlesExcludingBookingParams{
			GroupID:          claims.GroupID,
			ExcludeBookingID: bid,
			StartDate:        pgtype.Date{Time: startDate, Valid: true},
			EndDate:          pgtype.Date{Time: endDate, Valid: true},
		})
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to check availability")
			return
		}
		if commercialName != "" {
			var filtered []db.AvailableArticlesExcludingBookingRow
			for _, a := range articles {
				if a.CommercialName == commercialName {
					filtered = append(filtered, a)
				}
			}
			articles = filtered
		}
		WriteJSON(w, http.StatusOK, articles)
		return
	}

	articles, err := h.Q.AvailableArticles(r.Context(), db.AvailableArticlesParams{
		GroupID:   claims.GroupID,
		StartDate: pgtype.Date{Time: startDate, Valid: true},
		EndDate:   pgtype.Date{Time: endDate, Valid: true},
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to check availability")
		return
	}
	if commercialName != "" {
		var filtered []db.AvailableArticlesRow
		for _, a := range articles {
			if a.CommercialName == commercialName {
				filtered = append(filtered, a)
			}
		}
		articles = filtered
	}
	WriteJSON(w, http.StatusOK, articles)
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
	header, err := reader.Read()
	if err != nil {
		WriteError(w, http.StatusBadRequest, "failed to read CSV header")
		return
	}
	// Resolve column indices from header
	colIdx := map[string]int{}
	for i, col := range header {
		key := strings.ToLower(strings.TrimSpace(col))
		// Strip Haikuniq-style wrapping like `"cf: ...,type:text"`
		if strings.HasPrefix(key, "cf: ") {
			key = strings.TrimPrefix(key, "cf: ")
			if j := strings.Index(key, ","); j >= 0 {
				key = key[:j]
			}
		}
		colIdx[key] = i
	}
	col := func(record []string, name string) string {
		if i, ok := colIdx[name]; ok && i < len(record) {
			return strings.TrimSpace(record[i])
		}
		return ""
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

		commonName := col(record, "title")
		if commonName == "" {
			skipped++
			continue
		}

		description := col(record, "description")
		instructions := col(record, "instructions")
		managerNotes := col(record, "manager_notes")
		rawLocation := col(record, "location")
		plats := col(record, "plats")
		rum := col(record, "rum")
		lage := col(record, "lage")
		tag := col(record, "tags")

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

		commercialName := col(record, "titelgrupp")

		// Determine approval level from CSV column, default to 'none'
		approvalLevel := "none"
		if v := col(record, "requires_approval"); v != "" {
			switch strings.ToLower(v) {
			case "high":
				approvalLevel = "high"
			case "low", "true", "yes", "1":
				approvalLevel = "low"
			case "none", "false", "no", "0":
				approvalLevel = "none"
			}
		}

		// count column: if >1, create multiple quantity-tracked articles
		count := 1
		if c, err := strconv.Atoi(col(record, "count")); err == nil && c > 1 {
			count = c
		}
		individuallyTracked := count <= 1

		rowErr := false
		for range count {
			_, err = h.Q.CreateArticle(ctx, db.CreateArticleParams{
				GroupID:             claims.GroupID,
				CommercialName:      commercialName,
				CommonName:          commonName,
				CategoryID:          catID,
				LocationID:          locID,
				Status:              "ok",
				IndividuallyTracked: individuallyTracked,
				ApprovalLevel:       approvalLevel,
				Description:         description,
				Instructions:        instructions,
				Place:               place,
				ManagerNotes:        managerNotes,
			})
			if err != nil {
				errors = append(errors, fmt.Sprintf("row %d: %v", imported+skipped+2, err))
				rowErr = true
				break
			}
		}
		if rowErr {
			skipped++
			continue
		}
		imported += count
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"imported": imported,
		"skipped":  skipped,
		"errors":   errors,
	})
}

// BulkUpdate handles bulk status change, location move, and archive with conflict detection.
func (h *ArticleHandler) BulkUpdate(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	ctx := r.Context()

	var req struct {
		ArticleIDs    []string `json:"article_ids"`
		Status        string   `json:"status"`
		LocationID    string   `json:"location_id"`
		ApprovalLevel string   `json:"approval_level"`
		Comment       string   `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.ArticleIDs) == 0 {
		WriteError(w, http.StatusBadRequest, "article_ids required")
		return
	}
	if req.Status == "" && req.LocationID == "" && req.ApprovalLevel == "" {
		WriteError(w, http.StatusBadRequest, "status, location_id, or approval_level required")
		return
	}

	ids := make([]pgtype.UUID, len(req.ArticleIDs))
	for i, s := range req.ArticleIDs {
		id, err := parseUUID(s)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid article_id")
			return
		}
		ids[i] = id
	}

	type conflict struct {
		ArticleID    string `json:"article_id"`
		ArticleName  string `json:"article_name"`
		BookingID    string `json:"booking_id"`
		BookingDates string `json:"booking_dates"`
		BookingUnit  string `json:"booking_unit"`
	}

	// For archive: check active booking conflicts and attempt auto-replacement
	if req.Status == "archived" {
		conflicts, err := h.Q.ListActiveBookingConflicts(ctx, db.ListActiveBookingConflictsParams{
			Ids: ids, GroupID: claims.GroupID,
		})
		if err != nil {
			slog.Error("failed to check booking conflicts", "error", err)
			WriteError(w, http.StatusInternalServerError, "failed to check conflicts")
			return
		}

		var unresolvedConflicts []conflict
		replacedArticles := map[string]bool{} // article IDs that were auto-replaced

		for _, c := range conflicts {
			artIDStr := formatUUID(c.ArticleID)
			if replacedArticles[artIDStr] {
				continue
			}

			// Get article info for replacement search
			article, err := h.Q.GetArticle(ctx, db.GetArticleParams{ID: c.ArticleID, GroupID: claims.GroupID})
			if err != nil {
				continue
			}

			replacement, err := h.Q.FindReplacementArticle(ctx, db.FindReplacementArticleParams{
				GroupID:        claims.GroupID,
				CommercialName: article.CommercialName,
				LocationID:     article.LocationID,
				ExcludeIds:     ids,
				StartDate:      c.BookingStartDate,
				EndDate:        c.BookingEndDate,
			})
			if err != nil {
				// No replacement found
				startStr := c.BookingStartDate.Time.Format("2006-01-02")
				endStr := c.BookingEndDate.Time.Format("2006-01-02")
				unresolvedConflicts = append(unresolvedConflicts, conflict{
					ArticleID:    artIDStr,
					ArticleName:  c.ArticleName,
					BookingID:    formatUUID(c.BookingID),
					BookingDates: startStr + " — " + endStr,
					BookingUnit:  c.BookingUnit,
				})
				continue
			}

			// Auto-swap in the booking
			_, err = h.Q.SwapBookingItemArticleByArticle(ctx, db.SwapBookingItemArticleByArticleParams{
				NewArticleID: replacement,
				OldArticleID: c.ArticleID,
				BookingID:    c.BookingID,
				GroupID:      claims.GroupID,
			})
			if err != nil {
				slog.Error("failed to swap article in booking", "error", err)
				continue
			}
			replacedArticles[artIDStr] = true
		}

		if len(unresolvedConflicts) > 0 {
			WriteJSON(w, http.StatusOK, map[string]any{
				"updated":   0,
				"conflicts": unresolvedConflicts,
			})
			return
		}
	}

	var updated int64
	if req.Status != "" {
		n, err := h.Q.BulkUpdateArticleStatus(ctx, db.BulkUpdateArticleStatusParams{
			Status: req.Status, Ids: ids, GroupID: claims.GroupID,
		})
		if err != nil {
			slog.Error("failed to bulk update status", "error", err)
			WriteError(w, http.StatusInternalServerError, "failed to update")
			return
		}
		updated = n

		for _, id := range ids {
			LogArticleEvent(ctx, h.Q, claims, id, "status_change", req.Comment, map[string]string{
				"new_status": req.Status,
				"bulk":       "true",
			})
		}
	}

	if req.LocationID != "" {
		locID, err := parseUUID(req.LocationID)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid location_id")
			return
		}
		n, err := h.Q.BulkUpdateArticleLocation(ctx, db.BulkUpdateArticleLocationParams{
			LocationID: locID, Ids: ids, GroupID: claims.GroupID,
		})
		if err != nil {
			slog.Error("failed to bulk update location", "error", err)
			WriteError(w, http.StatusInternalServerError, "failed to update")
			return
		}
		if updated == 0 {
			updated = n
		}
		for _, id := range ids {
			LogArticleEvent(ctx, h.Q, claims, id, "status_change", req.Comment, map[string]string{
				"new_location_id": req.LocationID,
				"bulk":            "true",
			})
		}
	}

	if req.ApprovalLevel != "" {
		n, err := h.Q.BulkUpdateArticleApproval(ctx, db.BulkUpdateArticleApprovalParams{
			ApprovalLevel: req.ApprovalLevel, Ids: ids, GroupID: claims.GroupID,
		})
		if err != nil {
			slog.Error("failed to bulk update approval", "error", err)
			WriteError(w, http.StatusInternalServerError, "failed to update")
			return
		}
		if updated == 0 {
			updated = n
		}
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"updated":   updated,
		"conflicts": []any{},
	})
}

// GroupCount adjusts the count of a quantity tracked article group.
func (h *ArticleHandler) GroupCount(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	ctx := r.Context()

	var req struct {
		CommercialName string `json:"commercial_name"`
		LocationID     string `json:"location_id"`
		NewCount       int    `json:"new_count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.CommercialName == "" || req.LocationID == "" {
		WriteError(w, http.StatusBadRequest, "commercial_name and location_id required")
		return
	}
	if req.NewCount < 0 {
		WriteError(w, http.StatusBadRequest, "new_count must be >= 0")
		return
	}

	locID, err := parseUUID(req.LocationID)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid location_id")
		return
	}

	currentCount, err := h.Q.CountArticlesInGroup(ctx, db.CountArticlesInGroupParams{
		GroupID: claims.GroupID, CommercialName: req.CommercialName, LocationID: locID,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to count articles")
		return
	}

	diff := int(req.NewCount) - int(currentCount)
	if diff == 0 {
		WriteJSON(w, http.StatusOK, map[string]any{"count": currentCount})
		return
	}

	// Get representative article for event logging and as template for new articles
	representative, err := h.Q.GetArticleGroupInfo(ctx, db.GetArticleGroupInfoParams{
		GroupID: claims.GroupID, CommercialName: req.CommercialName, LocationID: locID,
	})
	if err != nil {
		WriteError(w, http.StatusNotFound, "article group not found")
		return
	}

	if diff > 0 {
		// Create new articles using representative as template
		for range diff {
			_, err := h.Q.CreateArticle(ctx, db.CreateArticleParams{
				GroupID:             claims.GroupID,
				CommercialName:      representative.CommercialName,
				CommonName:          representative.CommonName,
				CategoryID:          representative.CategoryID,
				LocationID:          representative.LocationID,
				Status:              "ok",
				IndividuallyTracked: false,
				ApprovalLevel:       representative.ApprovalLevel,
				Description:         representative.Description,
				Instructions:        representative.Instructions,
				Place:               representative.Place,
				ManagerNotes:        representative.ManagerNotes,
			})
			if err != nil {
				slog.Error("failed to create article for count increase", "error", err)
				WriteError(w, http.StatusInternalServerError, "failed to create articles")
				return
			}
		}
	} else {
		// Archive newest articles not in active bookings
		toArchive := -diff
		candidates, err := h.Q.ListNewestInGroup(ctx, db.ListNewestInGroupParams{
			GroupID: claims.GroupID, CommercialName: req.CommercialName, LocationID: locID,
		})
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to list articles")
			return
		}

		// Never archive the representative (oldest)
		var archiveIDs []pgtype.UUID
		for _, cid := range candidates {
			if formatUUID(cid) == formatUUID(representative.ID) {
				continue
			}
			archiveIDs = append(archiveIDs, cid)
			if len(archiveIDs) >= toArchive {
				break
			}
		}

		if len(archiveIDs) < toArchive {
			WriteError(w, http.StatusConflict, "cannot_reduce_count")
			return
		}

		_, err = h.Q.BulkUpdateArticleStatus(ctx, db.BulkUpdateArticleStatusParams{
			Status: "archived", Ids: archiveIDs, GroupID: claims.GroupID,
		})
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to archive articles")
			return
		}
	}

	// Log single count_changed event on the representative
	LogArticleEvent(ctx, h.Q, claims, representative.ID, "count_changed", "", map[string]string{
		"old_count": strconv.FormatInt(currentCount, 10),
		"new_count": strconv.Itoa(req.NewCount),
	})

	WriteJSON(w, http.StatusOK, map[string]any{"count": req.NewCount})
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
	runes := []rune(tag)
	runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
	for i := 1; i < len(runes); i++ {
		runes[i] = []rune(strings.ToLower(string(runes[i])))[0]
	}
	return string(runes)
}
