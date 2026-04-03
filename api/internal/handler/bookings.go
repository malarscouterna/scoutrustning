package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/malarscouterna/ms-utrustning/api/internal/auth"
	"github.com/malarscouterna/ms-utrustning/api/internal/db"
)

type BookingHandler struct {
	Q *db.Queries
}

func (h *BookingHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/{id}", h.Get)
	r.Put("/{id}", h.Update)
	r.Post("/{id}/items", h.AddItems)
	r.Delete("/{id}/items/{itemId}", h.RemoveItem)
	r.Post("/{id}/submit", h.Submit)
	r.Post("/{id}/cancel", h.Cancel)
	r.Post("/{id}/copy", h.Copy)
	r.Post("/{id}/pickup", h.Pickup)
	r.Put("/{id}/items/{itemId}/pickup", h.UpdateItemPickup)
	r.Post("/{id}/items/{itemId}/swap", h.SwapItem)
	r.Post("/{id}/return", h.Return)
	r.Put("/{id}/items/{itemId}/return", h.UpdateItemReturn)
	return r
}

// bookingAccess holds the fields needed for access checking.
type bookingAccess struct {
	CreatedBy    string
	UsedByUnitID pgtype.UUID
	Status       string
}

func accessFromGetBookingRow(b db.GetBookingRow) bookingAccess {
	return bookingAccess{CreatedBy: b.CreatedBy, UsedByUnitID: b.UsedByUnitID, Status: b.Status}
}

// canAccessBooking checks if the user can view/modify this booking.
func (h *BookingHandler) canAccessBooking(ctx context.Context, claims auth.Claims, b bookingAccess) bool {
	if claims.MemberID == b.CreatedBy {
		return true
	}
	if claims.HasRole("equipment_manager") {
		return true
	}
	if b.UsedByUnitID.Valid {
		unit, err := h.Q.GetUnitByID(ctx, db.GetUnitByIDParams{
			ID: b.UsedByUnitID, GroupID: claims.GroupID,
		})
		if err == nil {
			for _, u := range claims.Units {
				if u == unit.Name {
					return true
				}
			}
		}
	}
	return false
}

// isEditable returns true if the booking can be modified.
func isEditable(status string) bool {
	return status == "draft" || status == "confirmed" || status == "picked_up" ||
		status == "submitted" || status == "approved"
}

func (h *BookingHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	var req struct {
		StartDate             string  `json:"start_date"`
		EndDate               string  `json:"end_date"`
		UsedByUnitID          *string `json:"used_by_unit_id"`
		UsedByExternal        *string `json:"used_by_external"`
		UsedByExternalContact *string `json:"used_by_external_contact"`
		Notes                 string  `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid start_date")
		return
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid end_date")
		return
	}
	if endDate.Before(startDate) {
		WriteError(w, http.StatusBadRequest, "end_date must be after start_date")
		return
	}

	params := db.CreateBookingParams{
		GroupID:   claims.GroupID,
		CreatedBy: claims.MemberID,
		StartDate: pgtype.Date{Time: startDate, Valid: true},
		EndDate:   pgtype.Date{Time: endDate, Valid: true},
		Notes:     req.Notes,
	}
	if req.UsedByUnitID != nil {
		id, err := parseUUID(*req.UsedByUnitID)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid used_by_unit_id")
			return
		}
		params.UsedByUnitID = id
	}
	if req.UsedByExternal != nil {
		params.UsedByExternal = pgtype.Text{String: *req.UsedByExternal, Valid: true}
	}
	if req.UsedByExternalContact != nil {
		params.UsedByExternalContact = pgtype.Text{String: *req.UsedByExternalContact, Valid: true}
	}

	booking, err := h.Q.CreateBooking(r.Context(), params)
	if err != nil {
		slog.Error("failed to create booking", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to create booking")
		return
	}
	WriteJSON(w, http.StatusCreated, booking)
}

func (h *BookingHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}

	booking, err := h.Q.GetBooking(r.Context(), db.GetBookingParams{ID: id, GroupID: claims.GroupID})
	if err != nil {
		WriteError(w, http.StatusNotFound, "booking not found")
		return
	}

	items, err := h.Q.ListBookingItems(r.Context(), db.ListBookingItemsParams{
		BookingID: id, GroupID: claims.GroupID,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to list booking items")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"booking": booking,
		"items":   items,
	})
}

func (h *BookingHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())

	bookings, err := h.Q.ListBookingsByUser(r.Context(), db.ListBookingsByUserParams{
		GroupID:   claims.GroupID,
		UserID:    claims.MemberID,
		UnitNames: claims.Units,
	})
	if err != nil {
		slog.Error("failed to list bookings", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to list bookings")
		return
	}
	WriteJSON(w, http.StatusOK, bookings)
}

func (h *BookingHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	bookingID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}

	booking, err := h.Q.GetBooking(r.Context(), db.GetBookingParams{ID: bookingID, GroupID: claims.GroupID})
	if err != nil {
		WriteError(w, http.StatusNotFound, "booking not found")
		return
	}
	if !h.canAccessBooking(r.Context(), claims, accessFromGetBookingRow(booking)) {
		WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	if !isEditable(booking.Status) {
		WriteError(w, http.StatusBadRequest, "booking is not editable")
		return
	}

	var req struct {
		StartDate             *string `json:"start_date"`
		EndDate               *string `json:"end_date"`
		UsedByUnitID          *string `json:"used_by_unit_id"`
		UsedByExternal        *string `json:"used_by_external"`
		UsedByExternalContact *string `json:"used_by_external_contact"`
		Notes                 *string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	params := db.UpdateBookingParams{
		ID:                    bookingID,
		GroupID:               claims.GroupID,
		StartDate:             booking.StartDate,
		EndDate:               booking.EndDate,
		UsedByUnitID:          booking.UsedByUnitID,
		UsedByExternal:        booking.UsedByExternal,
		UsedByExternalContact: booking.UsedByExternalContact,
		Notes:                 booking.Notes,
	}

	if req.StartDate != nil {
		t, err := time.Parse("2006-01-02", *req.StartDate)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid start_date")
			return
		}
		params.StartDate = pgtype.Date{Time: t, Valid: true}
	}
	if req.EndDate != nil {
		t, err := time.Parse("2006-01-02", *req.EndDate)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid end_date")
			return
		}
		params.EndDate = pgtype.Date{Time: t, Valid: true}
	}
	if params.EndDate.Time.Before(params.StartDate.Time) {
		WriteError(w, http.StatusBadRequest, "end_date must be after start_date")
		return
	}
	if req.Notes != nil {
		params.Notes = *req.Notes
	}
	if req.UsedByUnitID != nil {
		id, err := parseUUID(*req.UsedByUnitID)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid used_by_unit_id")
			return
		}
		params.UsedByUnitID = id
	}
	if req.UsedByExternal != nil {
		params.UsedByExternal = pgtype.Text{String: *req.UsedByExternal, Valid: true}
	}
	if req.UsedByExternalContact != nil {
		params.UsedByExternalContact = pgtype.Text{String: *req.UsedByExternalContact, Valid: true}
	}

	// If dates changed, re-validate availability for all existing items
	datesChanged := params.StartDate != booking.StartDate || params.EndDate != booking.EndDate
	if datesChanged {
		items, err := h.Q.ListBookingItems(r.Context(), db.ListBookingItemsParams{
			BookingID: bookingID, GroupID: claims.GroupID,
		})
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to check items")
			return
		}

		available, err := h.Q.AvailableArticlesExcludingBooking(r.Context(), db.AvailableArticlesExcludingBookingParams{
			GroupID:          claims.GroupID,
			ExcludeBookingID: bookingID,
			StartDate:        params.StartDate,
			EndDate:          params.EndDate,
		})
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to check availability")
			return
		}

		availSet := map[pgtype.UUID]bool{}
		for _, a := range available {
			availSet[a.ID] = true
		}

		for _, item := range items {
			if item.ReturnStatus.Valid && item.ReturnStatus.String != "pending" {
				continue // already returned, skip
			}
			if !availSet[item.ArticleID] {
				WriteError(w, http.StatusConflict, fmt.Sprintf("article %s not available for new dates", item.CommonName))
				return
			}
		}
	}

	updated, err := h.Q.UpdateBooking(r.Context(), params)
	if err != nil {
		slog.Error("failed to update booking", "error", err)
		WriteError(w, http.StatusInternalServerError, "failed to update booking")
		return
	}
	WriteJSON(w, http.StatusOK, updated)
}

// AddItems assigns specific available articles to a booking.
func (h *BookingHandler) AddItems(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	bookingID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}

	booking, err := h.Q.GetBooking(r.Context(), db.GetBookingParams{ID: bookingID, GroupID: claims.GroupID})
	if err != nil {
		WriteError(w, http.StatusNotFound, "booking not found")
		return
	}
	if !h.canAccessBooking(r.Context(), claims, accessFromGetBookingRow(booking)) {
		WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	if !isEditable(booking.Status) {
		WriteError(w, http.StatusBadRequest, "booking is not editable")
		return
	}

	var req struct {
		CommercialName string `json:"commercial_name"`
		LocationName   string `json:"location_name"`
		Quantity       int    `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.CommercialName == "" || req.Quantity < 1 {
		WriteError(w, http.StatusBadRequest, "commercial_name and quantity >= 1 required")
		return
	}

	available, err := h.Q.AvailableArticlesExcludingBooking(r.Context(), db.AvailableArticlesExcludingBookingParams{
		GroupID:          claims.GroupID,
		ExcludeBookingID: bookingID,
		StartDate:        booking.StartDate,
		EndDate:          booking.EndDate,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to check availability")
		return
	}

	var matching []db.AvailableArticlesExcludingBookingRow
	for _, a := range available {
		if a.CommercialName == req.CommercialName {
			if req.LocationName == "" || a.LocationName == req.LocationName {
				matching = append(matching, a)
			}
		}
	}

	if len(matching) < req.Quantity {
		WriteError(w, http.StatusConflict, fmt.Sprintf("only %d available, requested %d", len(matching), req.Quantity))
		return
	}

	var added []db.BookingItem
	for i := range req.Quantity {
		item, err := h.Q.AddBookingItem(r.Context(), db.AddBookingItemParams{
			GroupID:   claims.GroupID,
			BookingID: bookingID,
			ArticleID: matching[i].ID,
		})
		if err != nil {
			slog.Error("failed to add booking item", "error", err)
			WriteError(w, http.StatusInternalServerError, "failed to add item")
			return
		}
		added = append(added, item)
	}

	WriteJSON(w, http.StatusCreated, added)

	// Auto-transition: if confirmed booking now has approval-required items, go to submitted
	if booking.Status == "confirmed" {
		needsApproval, err := h.Q.BookingHasApprovalRequired(r.Context(), db.BookingHasApprovalRequiredParams{
			BookingID: bookingID, GroupID: claims.GroupID,
		})
		if err == nil && needsApproval && !claims.HasRole("project_leader") {
			h.Q.UpdateBookingStatus(r.Context(), db.UpdateBookingStatusParams{
				ID: bookingID, GroupID: claims.GroupID, Status: "submitted",
			})
		}
	}
}

func (h *BookingHandler) RemoveItem(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	bookingID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid booking id")
		return
	}
	itemID, err := parseUUID(chi.URLParam(r, "itemId"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid item id")
		return
	}

	booking, err := h.Q.GetBooking(r.Context(), db.GetBookingParams{ID: bookingID, GroupID: claims.GroupID})
	if err != nil {
		WriteError(w, http.StatusNotFound, "booking not found")
		return
	}
	if !h.canAccessBooking(r.Context(), claims, accessFromGetBookingRow(booking)) {
		WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	if !isEditable(booking.Status) {
		WriteError(w, http.StatusBadRequest, "booking is not editable")
		return
	}

	err = h.Q.RemoveBookingItem(r.Context(), db.RemoveBookingItemParams{
		ID: itemID, GroupID: claims.GroupID, BookingID: bookingID,
	})
	if err != nil {
		WriteError(w, http.StatusNotFound, "item not found")
		return
	}

	// Auto-transition: if submitted booking no longer needs approval, auto-confirm
	if booking.Status == "submitted" {
		needsApproval, err := h.Q.BookingHasApprovalRequired(r.Context(), db.BookingHasApprovalRequiredParams{
			BookingID: bookingID, GroupID: claims.GroupID,
		})
		if err == nil && !needsApproval {
			h.Q.UpdateBookingStatus(r.Context(), db.UpdateBookingStatusParams{
				ID: bookingID, GroupID: claims.GroupID, Status: "confirmed",
			})
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *BookingHandler) Submit(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	bookingID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}

	booking, err := h.Q.GetBooking(r.Context(), db.GetBookingParams{ID: bookingID, GroupID: claims.GroupID})
	if err != nil {
		WriteError(w, http.StatusNotFound, "booking not found")
		return
	}
	if !isEditable(booking.Status) {
		WriteError(w, http.StatusBadRequest, "booking is not editable")
		return
	}

	needsApproval, err := h.Q.BookingHasApprovalRequired(r.Context(), db.BookingHasApprovalRequiredParams{
		BookingID: bookingID, GroupID: claims.GroupID,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to check approval")
		return
	}

	newStatus := "confirmed"
	if needsApproval && !claims.HasRole("project_leader") {
		newStatus = "submitted"
	}

	updated, err := h.Q.UpdateBookingStatus(r.Context(), db.UpdateBookingStatusParams{
		ID: bookingID, GroupID: claims.GroupID, Status: newStatus,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to update booking")
		return
	}

	WriteJSON(w, http.StatusOK, updated)
}

// Cancel transitions a booking to cancelled. Drafts are deleted entirely.
func (h *BookingHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	bookingID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}

	booking, err := h.Q.GetBooking(r.Context(), db.GetBookingParams{ID: bookingID, GroupID: claims.GroupID})
	if err != nil {
		WriteError(w, http.StatusNotFound, "booking not found")
		return
	}
	if !h.canAccessBooking(r.Context(), claims, accessFromGetBookingRow(booking)) {
		WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	if booking.Status == "returned" || booking.Status == "cancelled" {
		WriteError(w, http.StatusBadRequest, "booking already completed")
		return
	}

	if booking.Status == "draft" {
		// Delete drafts entirely
		err = h.Q.DeleteBooking(r.Context(), db.DeleteBookingParams{ID: bookingID, GroupID: claims.GroupID})
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "failed to delete draft")
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	updated, err := h.Q.UpdateBookingStatus(r.Context(), db.UpdateBookingStatusParams{
		ID: bookingID, GroupID: claims.GroupID, Status: "cancelled",
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to cancel booking")
		return
	}
	WriteJSON(w, http.StatusOK, updated)
}

// Pickup transitions a confirmed/approved booking to picked_up.
func (h *BookingHandler) Pickup(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	bookingID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}

	booking, err := h.Q.GetBooking(r.Context(), db.GetBookingParams{ID: bookingID, GroupID: claims.GroupID})
	if err != nil {
		WriteError(w, http.StatusNotFound, "booking not found")
		return
	}
	if !h.canAccessBooking(r.Context(), claims, accessFromGetBookingRow(booking)) {
		WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	if booking.Status != "confirmed" && booking.Status != "approved" {
		WriteError(w, http.StatusBadRequest, "booking must be confirmed or approved")
		return
	}

	updated, err := h.Q.UpdateBookingStatus(r.Context(), db.UpdateBookingStatusParams{
		ID: bookingID, GroupID: claims.GroupID, Status: "picked_up",
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to update booking")
		return
	}
	WriteJSON(w, http.StatusOK, updated)
}

// UpdateItemPickup sets the pickup_status for a single booking item.
func (h *BookingHandler) UpdateItemPickup(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	bookingID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid booking id")
		return
	}
	itemID, err := parseUUID(chi.URLParam(r, "itemId"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid item id")
		return
	}

	booking, err := h.Q.GetBooking(r.Context(), db.GetBookingParams{ID: bookingID, GroupID: claims.GroupID})
	if err != nil {
		WriteError(w, http.StatusNotFound, "booking not found")
		return
	}
	if !h.canAccessBooking(r.Context(), claims, accessFromGetBookingRow(booking)) {
		WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	if booking.Status != "picked_up" {
		WriteError(w, http.StatusBadRequest, "booking must be in picked_up status")
		return
	}

	var req struct {
		PickupStatus string `json:"pickup_status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	switch req.PickupStatus {
	case "picked_up", "not_available", "":
		// "" clears the status (undo)
	default:
		WriteError(w, http.StatusBadRequest, "pickup_status must be picked_up, not_available, or empty")
		return
	}

	var pickupStatus pgtype.Text
	if req.PickupStatus != "" {
		pickupStatus = pgtype.Text{String: req.PickupStatus, Valid: true}
	}

	item, err := h.Q.UpdateBookingItemPickupStatus(r.Context(), db.UpdateBookingItemPickupStatusParams{
		ID: itemID, GroupID: claims.GroupID, BookingID: bookingID,
		PickupStatus: pickupStatus,
	})
	if err != nil {
		WriteError(w, http.StatusNotFound, "item not found")
		return
	}
	WriteJSON(w, http.StatusOK, item)
}

// SwapItem replaces the article on a booking item during pickup.
func (h *BookingHandler) SwapItem(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	bookingID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid booking id")
		return
	}
	itemID, err := parseUUID(chi.URLParam(r, "itemId"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid item id")
		return
	}

	booking, err := h.Q.GetBooking(r.Context(), db.GetBookingParams{ID: bookingID, GroupID: claims.GroupID})
	if err != nil {
		WriteError(w, http.StatusNotFound, "booking not found")
		return
	}
	if !h.canAccessBooking(r.Context(), claims, accessFromGetBookingRow(booking)) {
		WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	if booking.Status != "picked_up" {
		WriteError(w, http.StatusBadRequest, "booking must be in picked_up status")
		return
	}

	var req struct {
		NewArticleID string `json:"new_article_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	newArticleID, err := parseUUID(req.NewArticleID)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid new_article_id")
		return
	}

	// Verify the new article is available for this booking's dates
	available, err := h.Q.AvailableArticlesExcludingBooking(r.Context(), db.AvailableArticlesExcludingBookingParams{
		GroupID:          claims.GroupID,
		ExcludeBookingID: bookingID,
		StartDate:        booking.StartDate,
		EndDate:          booking.EndDate,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to check availability")
		return
	}

	found := false
	for _, a := range available {
		if a.ID == newArticleID {
			found = true
			break
		}
	}
	if !found {
		WriteError(w, http.StatusConflict, "article not available")
		return
	}

	item, err := h.Q.SwapBookingItemArticle(r.Context(), db.SwapBookingItemArticleParams{
		ID: itemID, GroupID: claims.GroupID, BookingID: bookingID,
		NewArticleID: newArticleID,
	})
	if err != nil {
		WriteError(w, http.StatusNotFound, "item not found")
		return
	}
	WriteJSON(w, http.StatusOK, item)
}

// Return transitions a picked_up booking to returned.
func (h *BookingHandler) Return(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	bookingID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}

	booking, err := h.Q.GetBooking(r.Context(), db.GetBookingParams{ID: bookingID, GroupID: claims.GroupID})
	if err != nil {
		WriteError(w, http.StatusNotFound, "booking not found")
		return
	}
	if !h.canAccessBooking(r.Context(), claims, accessFromGetBookingRow(booking)) {
		WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	if booking.Status != "picked_up" {
		WriteError(w, http.StatusBadRequest, "booking must be in picked_up status")
		return
	}

	allReturned, err := h.Q.AllItemsReturned(r.Context(), db.AllItemsReturnedParams{
		BookingID: bookingID, GroupID: claims.GroupID,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to check return status")
		return
	}
	if !allReturned {
		WriteError(w, http.StatusBadRequest, "not all items have been returned")
		return
	}

	updated, err := h.Q.UpdateBookingStatus(r.Context(), db.UpdateBookingStatusParams{
		ID: bookingID, GroupID: claims.GroupID, Status: "returned",
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to update booking")
		return
	}
	WriteJSON(w, http.StatusOK, updated)
}

// UpdateItemReturn sets the return status for a single booking item.
// Side effects: broken/lost creates an issue report and updates article status.
func (h *BookingHandler) UpdateItemReturn(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	bookingID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid booking id")
		return
	}
	itemID, err := parseUUID(chi.URLParam(r, "itemId"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid item id")
		return
	}

	booking, err := h.Q.GetBooking(r.Context(), db.GetBookingParams{ID: bookingID, GroupID: claims.GroupID})
	if err != nil {
		WriteError(w, http.StatusNotFound, "booking not found")
		return
	}
	if !h.canAccessBooking(r.Context(), claims, accessFromGetBookingRow(booking)) {
		WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	if booking.Status != "picked_up" && booking.Status != "returned" {
		WriteError(w, http.StatusBadRequest, "booking must be in picked_up or returned status")
		return
	}

	// Reopen if already returned
	if booking.Status == "returned" {
		h.Q.UpdateBookingStatus(r.Context(), db.UpdateBookingStatusParams{
			ID: bookingID, GroupID: claims.GroupID, Status: "picked_up",
		})
	}

	var req struct {
		ReturnStatus       string  `json:"return_status"`
		ExpectedReturnDate *string `json:"expected_return_date"`
		Notes              string  `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	validStatuses := map[string]bool{"returned_ok": true, "delayed": true, "broken": true, "lost": true, "": true}
	if !validStatuses[req.ReturnStatus] {
		WriteError(w, http.StatusBadRequest, "return_status must be returned_ok, delayed, broken, lost, or empty")
		return
	}

	if req.ReturnStatus == "delayed" && req.ExpectedReturnDate == nil {
		WriteError(w, http.StatusBadRequest, "expected_return_date required when return_status is delayed")
		return
	}

	var returnStatus pgtype.Text
	if req.ReturnStatus != "" {
		returnStatus = pgtype.Text{String: req.ReturnStatus, Valid: true}
	}

	item, err := h.Q.UpdateBookingItemReturnStatus(r.Context(), db.UpdateBookingItemReturnStatusParams{
		ID: itemID, GroupID: claims.GroupID, BookingID: bookingID,
		ReturnStatus: returnStatus,
	})
	if err != nil {
		WriteError(w, http.StatusNotFound, "item not found")
		return
	}

	// Side effects based on return status
	switch req.ReturnStatus {
	case "":
		// Undo: reset article to ok
		h.Q.UpdateArticleStatus(r.Context(), db.UpdateArticleStatusParams{
			ID: item.ArticleID, GroupID: claims.GroupID,
			Status: "ok",
		})
	case "delayed":
		// No article status change — item is still out on loan
	case "broken":
		h.Q.UpdateArticleStatus(r.Context(), db.UpdateArticleStatusParams{
			ID: item.ArticleID, GroupID: claims.GroupID,
			Status: "reported_unusable",
		})
		h.Q.CreateIssueReport(r.Context(), db.CreateIssueReportParams{
			GroupID:     claims.GroupID,
			ArticleID:   item.ArticleID,
			ReporterID:  claims.MemberID,
			Description: req.Notes,
			Severity:    "unusable",
		})
	case "lost":
		h.Q.UpdateArticleStatus(r.Context(), db.UpdateArticleStatusParams{
			ID: item.ArticleID, GroupID: claims.GroupID,
			Status: "archived",
		})
		h.Q.CreateIssueReport(r.Context(), db.CreateIssueReportParams{
			GroupID:     claims.GroupID,
			ArticleID:   item.ArticleID,
			ReporterID:  claims.MemberID,
			Description: req.Notes,
			Severity:    "unusable",
		})
	case "returned_ok":
		h.Q.UpdateArticleStatus(r.Context(), db.UpdateArticleStatusParams{
			ID: item.ArticleID, GroupID: claims.GroupID,
			Status: "ok",
		})
	}

	WriteJSON(w, http.StatusOK, item)
}

// Copy creates a new draft booking with the same unit and items as the source.
// Dates are left as a placeholder (today + 7 days) since the user needs to pick new dates.
// Items that no longer exist are silently skipped.
func (h *BookingHandler) Copy(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	sourceID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid id")
		return
	}

	source, err := h.Q.GetBooking(r.Context(), db.GetBookingParams{ID: sourceID, GroupID: claims.GroupID})
	if err != nil {
		WriteError(w, http.StatusNotFound, "booking not found")
		return
	}

	sourceItems, err := h.Q.ListBookingItems(r.Context(), db.ListBookingItemsParams{
		BookingID: sourceID, GroupID: claims.GroupID,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to list source items")
		return
	}

	// Create new draft with same unit, placeholder dates
	now := time.Now()
	newBooking, err := h.Q.CreateBooking(r.Context(), db.CreateBookingParams{
		GroupID:               claims.GroupID,
		CreatedBy:             claims.MemberID,
		UsedByUnitID:          source.UsedByUnitID,
		UsedByExternal:        source.UsedByExternal,
		UsedByExternalContact: source.UsedByExternalContact,
		StartDate:             pgtype.Date{Time: now, Valid: true},
		EndDate:               pgtype.Date{Time: now.AddDate(0, 0, 7), Valid: true},
		Notes:                 source.Notes,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to create copy")
		return
	}

	// Copy items, skip any that fail (article may have been deleted)
	var copied int
	for _, item := range sourceItems {
		_, err := h.Q.AddBookingItem(r.Context(), db.AddBookingItemParams{
			GroupID:   claims.GroupID,
			BookingID: newBooking.ID,
			ArticleID: item.ArticleID,
		})
		if err == nil {
			copied++
		}
	}

	WriteJSON(w, http.StatusCreated, map[string]any{
		"booking":      newBooking,
		"items_copied": copied,
		"items_total":  len(sourceItems),
	})
}
