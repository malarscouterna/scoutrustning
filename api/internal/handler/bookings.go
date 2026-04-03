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
	return r
}

// canAccessBooking checks if the user can view/modify this booking.
// Access is granted to: the creator, any leader in the booking's unit, or equipment managers.
func (h *BookingHandler) canAccessBooking(ctx context.Context, claims auth.Claims, booking db.Booking) bool {
	if claims.MemberID == booking.CreatedBy {
		return true
	}
	if claims.HasRole("equipment_manager") {
		return true
	}
	if booking.UsedByUnitID.Valid {
		unit, err := h.Q.GetUnitByID(ctx, db.GetUnitByIDParams{
			ID: booking.UsedByUnitID, GroupID: claims.GroupID,
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
	if !h.canAccessBooking(r.Context(), claims, booking) {
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

		availSet := map[string]bool{}
		for _, a := range available {
			availSet[fmt.Sprintf("%x", a.ID.Bytes)] = true
		}

		for _, item := range items {
			if item.ReturnStatus.Valid && item.ReturnStatus.String != "pending" {
				continue // already returned, skip
			}
			if !availSet[fmt.Sprintf("%x", item.ArticleID.Bytes)] {
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
	if !h.canAccessBooking(r.Context(), claims, booking) {
		WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	if !isEditable(booking.Status) {
		WriteError(w, http.StatusBadRequest, "booking is not editable")
		return
	}

	var req struct {
		CommercialName string `json:"commercial_name"`
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
			matching = append(matching, a)
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
	if !h.canAccessBooking(r.Context(), claims, booking) {
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
	if booking.Status != "draft" {
		WriteError(w, http.StatusBadRequest, "can only submit draft bookings")
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
