package room

import (
	"crypto/subtle"
	"encoding/json"
	"errors"

	"io"
	"net/http"
	"strings"

	"time"
	"unicode/utf8"

	"splitthebill/backend/internal/calculation"
	"splitthebill/backend/internal/domain"
	"splitthebill/backend/internal/store"
)

const (
	maxTitleLength       = 120
	maxParticipantLength = 80
	maxItemNameLength    = 160
	maxAssignmentWeight  = 1000
)

type Handler struct {
	store store.Store
}

func NewHandler(appStore store.Store) *Handler {
	return &Handler{store: appStore}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")

	if path == "rooms" {
		if r.Method == http.MethodPost {
			h.createRoom(w, r)
			return
		}

		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if len(parts) < 2 || parts[0] != "rooms" || parts[1] == "" {
		writeError(w, http.StatusNotFound, "route not found")
		return
	}

	roomID := parts[1]

	if len(parts) == 2 {
		switch r.Method {
		case http.MethodGet:
			h.getRoom(w, roomID)
		case http.MethodPatch:
			h.updateRoom(w, r, roomID)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	if len(parts) == 3 {
		switch parts[2] {
		case "join":
			if r.Method == http.MethodPost {
				h.joinParticipant(w, r, roomID)
				return
			}
		case "participants":
			if r.Method == http.MethodPost {
				h.addParticipant(w, r, roomID)
				return
			}
		case "items":
			if r.Method == http.MethodPost {
				h.addItem(w, r, roomID)
				return
			}
		case "assignments":
			if r.Method == http.MethodPost {
				h.addAssignment(w, r, roomID)
				return
			}
		case "calculate":
			if r.Method == http.MethodPost {
				h.calculate(w, roomID)
				return
			}
		case "open":
			if r.Method == http.MethodPost {
				h.openClaiming(w, r, roomID)
				return
			}
		case "finalize":
			if r.Method == http.MethodPost {
				h.finalize(w, r, roomID)
				return
			}
		case "reopen":
			if r.Method == http.MethodPost {
				h.reopen(w, r, roomID)
				return
			}
		default:
			writeError(w, http.StatusNotFound, "route not found")
			return
		}

		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if len(parts) == 4 {
		resourceID := parts[3]

		switch parts[2] {
		case "participants":
			switch r.Method {
			case http.MethodPatch:
				h.updateParticipant(w, r, roomID, resourceID)
			case http.MethodDelete:
				h.deleteParticipant(w, r, roomID, resourceID)
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			}
			return

		case "items":
			switch r.Method {
			case http.MethodPatch:
				h.updateItem(w, r, roomID, resourceID)
			case http.MethodDelete:
				h.deleteItem(w, r, roomID, resourceID)
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			}
			return

		case "selections":
			switch r.Method {
			case http.MethodPut:
				h.selectItem(w, r, roomID, resourceID)
			case http.MethodDelete:
				h.unselectItem(w, r, roomID, resourceID)
			default:
				writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			}
			return
		}
	}

	if len(parts) == 5 && parts[2] == "assignments" {
		if r.Method == http.MethodDelete {
			h.deleteAssignment(w, r, roomID, parts[3], parts[4])
			return
		}

		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeError(w, http.StatusNotFound, "route not found")
}

type createRoomRequest struct {
	Title         string `json:"title"`
	Currency      string `json:"currency"`
	ServiceFee    int64  `json:"service_fee"`
	TipAmount     int64  `json:"tip_amount"`
	Discount      int64  `json:"discount"`
	DiscountMode  string `json:"discount_mode"`
	ExpectedTotal int64  `json:"expected_total"`
}

func (h *Handler) createRoom(w http.ResponseWriter, r *http.Request) {
	var req createRoomRequest

	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Currency = strings.ToUpper(strings.TrimSpace(req.Currency))

	if req.Currency == "" {
		req.Currency = "EUR"
	}

	discountMode := domain.DiscountMode(strings.TrimSpace(req.DiscountMode))
	if discountMode == "" {
		discountMode = domain.DiscountModeProportional
	}

	if err := validateTitle(req.Title); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := validateCurrency(req.Currency); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateCharges(req.ServiceFee, req.TipAmount, req.Discount, req.ExpectedTotal); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateDiscountMode(discountMode); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	created, err := h.store.CreateRoom(domain.Room{
		Title:         req.Title,
		Currency:      req.Currency,
		ServiceFee:    req.ServiceFee,
		TipAmount:     req.TipAmount,
		Discount:      req.Discount,
		DiscountMode:  discountMode,
		ExpectedTotal: req.ExpectedTotal,
		Status:        domain.RoomStatusDraft,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create room")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"room":        created,
		"admin_token": created.AdminToken,
	})
}

type updateRoomRequest struct {
	Title              *string `json:"title"`
	Currency           *string `json:"currency"`
	ServiceFee         *int64  `json:"service_fee"`
	TipAmount          *int64  `json:"tip_amount"`
	Discount           *int64  `json:"discount"`
	DiscountMode       *string `json:"discount_mode"`
	ExpectedTotal      *int64  `json:"expected_total"`
	PayerParticipantID *string `json:"payer_participant_id"`
}

func (h *Handler) updateRoom(w http.ResponseWriter, r *http.Request, roomID string) {
	room, ok := h.authorizeAdmin(w, r, roomID)
	if !ok {
		return
	}
	if !ensureMutable(w, room) {
		return
	}

	var req updateRoomRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if req.Title != nil {
		room.Title = strings.TrimSpace(*req.Title)
	}
	if req.Currency != nil {
		room.Currency = strings.ToUpper(strings.TrimSpace(*req.Currency))
	}
	if req.ServiceFee != nil {
		room.ServiceFee = *req.ServiceFee
	}
	if req.TipAmount != nil {
		room.TipAmount = *req.TipAmount
	}
	if req.Discount != nil {
		room.Discount = *req.Discount
	}
	if req.DiscountMode != nil {
		room.DiscountMode = domain.DiscountMode(strings.TrimSpace(*req.DiscountMode))
	}
	if req.ExpectedTotal != nil {
		room.ExpectedTotal = *req.ExpectedTotal
	}
	if req.PayerParticipantID != nil {
		room.PayerParticipantID = strings.TrimSpace(*req.PayerParticipantID)
	}

	if err := validateTitle(room.Title); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateCurrency(room.Currency); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateCharges(room.ServiceFee, room.TipAmount, room.Discount, room.ExpectedTotal); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateDiscountMode(domain.DiscountMode(room.DiscountMode)); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if room.PayerParticipantID != "" {
		if _, err := h.findParticipant(roomID, room.PayerParticipantID); err != nil {
			writeError(w, http.StatusBadRequest, "payer must be a participant of this room")
			return
		}
	}

	updated, err := h.store.UpdateRoom(room)
	if err != nil {
		writeStoreError(w, err, "room not found")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) getRoom(w http.ResponseWriter, roomID string) {
	room, err := h.store.GetRoom(roomID)
	if err != nil {
		writeStoreError(w, err, "room not found")
		return
	}
	participants, err := h.store.ListParticipants(roomID)
	if err != nil {
		writeStoreError(w, err, "room not found")
		return
	}
	items, err := h.store.ListItems(roomID)
	if err != nil {
		writeStoreError(w, err, "room not found")
		return
	}
	assignments, err := h.store.ListAssignments(roomID)
	if err != nil {
		writeStoreError(w, err, "room not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"room":                room,
		"participants":        nonNilParticipants(participants),
		"items":               nonNilItems(items),
		"assignments":         nonNilAssignments(assignments),
		"subtotal":            calculateSubtotal(items),
		"unassigned_item_ids": unassignedItemIDs(items, assignments),
	})
}

type participantRequest struct {
	Name string `json:"name"`
}

func (h *Handler) addParticipant(w http.ResponseWriter, r *http.Request, roomID string) {
	room, ok := h.authorizeAdmin(w, r, roomID)
	if !ok || !ensureMutable(w, room) {
		return
	}

	var req participantRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if err := validateParticipantName(req.Name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	created, err := h.store.AddParticipant(roomID, domain.Participant{Name: req.Name})
	if err != nil {
		writeStoreError(w, err, "room not found")
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) joinParticipant(w http.ResponseWriter, r *http.Request, roomID string) {
	room, err := h.store.GetRoom(roomID)
	if err != nil {
		writeStoreError(w, err, "room not found")
		return
	}
	if room.Status == domain.RoomStatusFinalized {
		writeError(w, http.StatusConflict, "room is finalized")
		return
	}

	var req participantRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if err := validateParticipantName(req.Name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	participant, err := h.store.JoinParticipant(roomID, req.Name)
	if err != nil {
		writeStoreError(w, err, "room not found")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"participant":       participant,
		"participant_token": participant.AccessToken,
	})
}

func (h *Handler) updateParticipant(w http.ResponseWriter, r *http.Request, roomID, participantID string) {
	room, ok := h.authorizeAdmin(w, r, roomID)
	if !ok || !ensureMutable(w, room) {
		return
	}

	var req participantRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if err := validateParticipantName(req.Name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	existing, err := h.findParticipant(roomID, participantID)
	if err != nil {
		writeStoreError(w, err, "participant not found")
		return
	}
	existing.Name = req.Name

	updated, err := h.store.UpdateParticipant(roomID, existing)
	if err != nil {
		writeStoreError(w, err, "participant not found")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) deleteParticipant(w http.ResponseWriter, r *http.Request, roomID, participantID string) {
	room, ok := h.authorizeAdmin(w, r, roomID)
	if !ok || !ensureMutable(w, room) {
		return
	}
	if err := h.store.DeleteParticipant(roomID, participantID); err != nil {
		writeStoreError(w, err, "participant not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type addItemRequest struct {
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	UnitPrice int64  `json:"unit_price"`
}

func (h *Handler) addItem(w http.ResponseWriter, r *http.Request, roomID string) {
	room, ok := h.authorizeAdmin(w, r, roomID)
	if !ok || !ensureMutable(w, room) {
		return
	}

	var req addItemRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Quantity == 0 {
		req.Quantity = 1
	}
	if err := validateItem(req.Name, req.Quantity, req.UnitPrice); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	created, err := h.store.AddItem(roomID, domain.ReceiptItem{
		Name:      req.Name,
		Quantity:  req.Quantity,
		UnitPrice: req.UnitPrice,
		Total:     int64(req.Quantity) * req.UnitPrice,
	})
	if err != nil {
		writeStoreError(w, err, "room not found")
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

type updateItemRequest struct {
	Name      *string `json:"name"`
	Quantity  *int    `json:"quantity"`
	UnitPrice *int64  `json:"unit_price"`
}

func (h *Handler) updateItem(w http.ResponseWriter, r *http.Request, roomID, itemID string) {
	room, ok := h.authorizeAdmin(w, r, roomID)
	if !ok || !ensureMutable(w, room) {
		return
	}

	item, err := h.findItem(roomID, itemID)
	if err != nil {
		writeStoreError(w, err, "item not found")
		return
	}

	var req updateItemRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Name != nil {
		item.Name = strings.TrimSpace(*req.Name)
	}
	if req.Quantity != nil {
		item.Quantity = *req.Quantity
	}
	if req.UnitPrice != nil {
		item.UnitPrice = *req.UnitPrice
	}
	if err := validateItem(item.Name, item.Quantity, item.UnitPrice); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	item.Total = int64(item.Quantity) * item.UnitPrice
	updated, err := h.store.UpdateItem(roomID, item)
	if err != nil {
		writeStoreError(w, err, "item not found")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) deleteItem(w http.ResponseWriter, r *http.Request, roomID, itemID string) {
	room, ok := h.authorizeAdmin(w, r, roomID)
	if !ok || !ensureMutable(w, room) {
		return
	}
	if err := h.store.DeleteItem(roomID, itemID); err != nil {
		writeStoreError(w, err, "item not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type addAssignmentRequest struct {
	ItemID        string `json:"item_id"`
	ParticipantID string `json:"participant_id"`
	Weight        int64  `json:"weight"`
}

func (h *Handler) addAssignment(w http.ResponseWriter, r *http.Request, roomID string) {
	room, ok := h.authorizeAdmin(w, r, roomID)
	if !ok || !ensureMutable(w, room) {
		return
	}

	var req addAssignmentRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.ItemID = strings.TrimSpace(req.ItemID)
	req.ParticipantID = strings.TrimSpace(req.ParticipantID)
	if req.ItemID == "" || req.ParticipantID == "" {
		writeError(w, http.StatusBadRequest, "item_id and participant_id are required")
		return
	}
	if err := validateWeight(req.Weight); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	created, err := h.store.AddAssignment(roomID, domain.ItemAssignment{
		ItemID:        req.ItemID,
		ParticipantID: req.ParticipantID,
		Weight:        req.Weight,
	})
	if err != nil {
		writeAssignmentStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) deleteAssignment(w http.ResponseWriter, r *http.Request, roomID, itemID, participantID string) {
	room, ok := h.authorizeAdmin(w, r, roomID)
	if !ok || !ensureMutable(w, room) {
		return
	}
	if err := h.store.DeleteAssignment(roomID, itemID, participantID); err != nil {
		writeStoreError(w, err, "assignment not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type selectionRequest struct {
	Weight int64 `json:"weight"`
}

func (h *Handler) selectItem(w http.ResponseWriter, r *http.Request, roomID, itemID string) {
	participant, ok := h.authorizeParticipant(w, r, roomID)
	if !ok {
		return
	}

	room, err := h.store.GetRoom(roomID)
	if err != nil {
		writeStoreError(w, err, "room not found")
		return
	}
	if room.Status != domain.RoomStatusClaiming {
		writeError(w, http.StatusConflict, "room is not open for selections")
		return
	}

	req := selectionRequest{Weight: 1}
	if err := readOptionalJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := validateWeight(req.Weight); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	created, err := h.store.AddAssignment(roomID, domain.ItemAssignment{
		ItemID:        itemID,
		ParticipantID: participant.ID,
		Weight:        req.Weight,
	})
	if err != nil {
		writeAssignmentStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, created)
}

func (h *Handler) unselectItem(w http.ResponseWriter, r *http.Request, roomID, itemID string) {
	participant, ok := h.authorizeParticipant(w, r, roomID)
	if !ok {
		return
	}

	room, err := h.store.GetRoom(roomID)
	if err != nil {
		writeStoreError(w, err, "room not found")
		return
	}
	if room.Status != domain.RoomStatusClaiming {
		writeError(w, http.StatusConflict, "room is not open for selections")
		return
	}

	if err := h.store.DeleteAssignment(roomID, itemID, participant.ID); err != nil {
		if !errors.Is(err, store.ErrorNotFound) {
			writeStoreError(w, err, "selection not found")
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) openClaiming(w http.ResponseWriter, r *http.Request, roomID string) {
	room, ok := h.authorizeAdmin(w, r, roomID)
	if !ok {
		return
	}
	if room.Status == domain.RoomStatusFinalized {
		writeError(w, http.StatusConflict, "room is finalized")
		return
	}

	items, err := h.store.ListItems(roomID)
	if err != nil {
		writeStoreError(w, err, "room not found")
		return
	}
	if len(items) == 0 {
		writeError(w, http.StatusConflict, "add at least one item before opening selections")
		return
	}

	room.Status = domain.RoomStatusClaiming
	room.FinalizedAt = nil
	updated, err := h.store.UpdateRoom(room)
	if err != nil {
		writeStoreError(w, err, "room not found")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) finalize(w http.ResponseWriter, r *http.Request, roomID string) {
	room, ok := h.authorizeAdmin(w, r, roomID)
	if !ok {
		return
	}
	if room.Status != domain.RoomStatusClaiming {
		writeError(w, http.StatusConflict, "room must be open for selections before finalization")
		return
	}
	if room.PayerParticipantID == "" {
		writeError(w, http.StatusConflict, "payer is required before finalization")
		return
	}

	response, err := h.buildCalculationResponse(roomID, room)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if room.ExpectedTotal > 0 && !response.MatchesExpectedTotal {
		writeError(w, http.StatusConflict, "calculated total does not match expected total")
		return
	}

	now := time.Now().UTC()
	room.Status = domain.RoomStatusFinalized
	room.FinalizedAt = &now
	updated, err := h.store.UpdateRoom(room)
	if err != nil {
		writeStoreError(w, err, "room not found")
		return
	}
	response.Room = updated
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) reopen(w http.ResponseWriter, r *http.Request, roomID string) {
	room, ok := h.authorizeAdmin(w, r, roomID)
	if !ok {
		return
	}
	if room.Status != domain.RoomStatusFinalized {
		writeError(w, http.StatusConflict, "room is not finalized")
		return
	}

	room.Status = domain.RoomStatusClaiming
	room.FinalizedAt = nil
	updated, err := h.store.UpdateRoom(room)
	if err != nil {
		writeStoreError(w, err, "room not found")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) calculate(w http.ResponseWriter, roomID string) {
	room, err := h.store.GetRoom(roomID)
	if err != nil {
		writeStoreError(w, err, "room not found")
		return
	}

	response, err := h.buildCalculationResponse(roomID, room)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, response)
}

type calculationResponse struct {
	Room                 domain.Room                `json:"room"`
	Results              []domain.ParticipantResult `json:"results"`
	Debts                []domain.Debt              `json:"debts"`
	Subtotal             int64                      `json:"subtotal"`
	CalculatedTotal      int64                      `json:"calculated_total"`
	Difference           int64                      `json:"difference"`
	MatchesExpectedTotal bool                       `json:"matches_expected_total"`
}

func (h *Handler) buildCalculationResponse(roomID string, room domain.Room) (calculationResponse, error) {
	participants, err := h.store.ListParticipants(roomID)
	if err != nil {
		return calculationResponse{}, err
	}
	items, err := h.store.ListItems(roomID)
	if err != nil {
		return calculationResponse{}, err
	}
	assignments, err := h.store.ListAssignments(roomID)
	if err != nil {
		return calculationResponse{}, err
	}

	if unassigned := unassignedItemIDs(items, assignments); len(unassigned) > 0 {
		return calculationResponse{}, errors.New("all items must have at least one participant")
	}

	results, err := calculation.Calculate(calculation.BillInput{
		Participants: participants,
		Items:        items,
		Assignments:  assignments,
		ServiceFee:   room.ServiceFee,
		TipAmount:    room.TipAmount,
		Discount:     room.Discount,
		DiscountMode: room.DiscountMode,
	})
	if err != nil {
		return calculationResponse{}, err
	}

	var calculatedTotal int64
	for _, result := range results {
		calculatedTotal += result.TotalAmount
	}

	difference := int64(0)
	matchesExpected := true
	if room.ExpectedTotal > 0 {
		difference = calculatedTotal - room.ExpectedTotal
		matchesExpected = difference == 0
	}

	debts := []domain.Debt{}
	if room.PayerParticipantID != "" {
		debts, err = calculation.BuildDebts(results, room.PayerParticipantID)
		if err != nil {
			return calculationResponse{}, err
		}
	}

	return calculationResponse{
		Room:                 room,
		Results:              results,
		Debts:                debts,
		Subtotal:             calculateSubtotal(items),
		CalculatedTotal:      calculatedTotal,
		Difference:           difference,
		MatchesExpectedTotal: matchesExpected,
	}, nil
}

func (h *Handler) authorizeAdmin(w http.ResponseWriter, r *http.Request, roomID string) (domain.Room, bool) {
	room, err := h.store.GetRoom(roomID)
	if err != nil {
		writeStoreError(w, err, "room not found")
		return domain.Room{}, false
	}

	provided := strings.TrimSpace(r.Header.Get("X-Admin-Token"))
	if provided == "" || !tokensEqual(provided, room.AdminToken) {
		writeError(w, http.StatusUnauthorized, "organizer access required")
		return domain.Room{}, false
	}
	return room, true
}

func (h *Handler) authorizeParticipant(w http.ResponseWriter, r *http.Request, roomID string) (domain.Participant, bool) {
	token := strings.TrimSpace(r.Header.Get("X-Participant-Token"))
	if token == "" {
		writeError(w, http.StatusUnauthorized, "participant session is invalid")
		return domain.Participant{}, false
	}

	participant, err := h.store.FindParticipantByToken(roomID, token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "participant session is invalid")
		return domain.Participant{}, false
	}
	return participant, true
}

func tokensEqual(left, right string) bool {
	if len(left) != len(right) || left == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func ensureMutable(w http.ResponseWriter, room domain.Room) bool {
	if room.Status == domain.RoomStatusFinalized {
		writeError(w, http.StatusConflict, "room is finalized")
		return false
	}
	return true
}

func (h *Handler) findParticipant(roomID, participantID string) (domain.Participant, error) {
	participants, err := h.store.ListParticipants(roomID)
	if err != nil {
		return domain.Participant{}, err
	}
	for _, participant := range participants {
		if participant.ID == participantID {
			return participant, nil
		}
	}
	return domain.Participant{}, store.ErrorParticipantNotFound
}

func (h *Handler) findItem(roomID, itemID string) (domain.ReceiptItem, error) {
	items, err := h.store.ListItems(roomID)
	if err != nil {
		return domain.ReceiptItem{}, err
	}
	for _, item := range items {
		if item.ID == itemID {
			return item, nil
		}
	}
	return domain.ReceiptItem{}, store.ErrorItemNotFound
}

func unassignedItemIDs(items []domain.ReceiptItem, assignments []domain.ItemAssignment) []string {
	assigned := make(map[string]bool, len(assignments))
	for _, assignment := range assignments {
		assigned[assignment.ItemID] = true
	}

	result := make([]string, 0)
	for _, item := range items {
		if !assigned[item.ID] {
			result = append(result, item.ID)
		}
	}
	return result
}

func validateTitle(value string) error {
	if value == "" {
		return errors.New("title is required")
	}
	if utf8.RuneCountInString(value) > maxTitleLength {
		return errors.New("title is too long")
	}
	return nil
}

func validateCurrency(value string) error {
	if len(value) != 3 {
		return errors.New("currency must be a three-letter code")
	}
	for _, char := range value {
		if char < 'A' || char > 'Z' {
			return errors.New("currency must contain only latin letters")
		}
	}
	return nil
}

func validateCharges(serviceFee, tipAmount, discount, expectedTotal int64) error {
	if serviceFee < 0 || tipAmount < 0 || discount < 0 || expectedTotal < 0 {
		return errors.New("service_fee, tip_amount, discount and expected_total must be non-negative")
	}
	return nil
}

func validateDiscountMode(value domain.DiscountMode) error {
	if value != domain.DiscountModeProportional && value != domain.DiscountModeEqual {
		return errors.New("discount_mode must be proportional or equal")
	}
	return nil
}

func validateParticipantName(value string) error {
	if value == "" {
		return errors.New("name is required")
	}
	if utf8.RuneCountInString(value) > maxParticipantLength {
		return errors.New("participant name is too long")
	}
	return nil
}

func validateItem(name string, quantity int, unitPrice int64) error {
	if name == "" {
		return errors.New("name is required")
	}
	if utf8.RuneCountInString(name) > maxItemNameLength {
		return errors.New("item name is too long")
	}
	if quantity <= 0 {
		return errors.New("quantity must be positive")
	}
	if unitPrice <= 0 {
		return errors.New("unit_price must be positive")
	}
	return nil
}

func validateWeight(weight int64) error {
	if weight <= 0 || weight > maxAssignmentWeight {
		return errors.New("weight must be between 1 and 1000")
	}
	return nil
}

func calculateSubtotal(items []domain.ReceiptItem) int64 {
	var subtotal int64
	for _, item := range items {
		subtotal += item.Total
	}
	return subtotal
}

func nonNilParticipants(value []domain.Participant) []domain.Participant {
	if value == nil {
		return []domain.Participant{}
	}
	return value
}

func nonNilItems(value []domain.ReceiptItem) []domain.ReceiptItem {
	if value == nil {
		return []domain.ReceiptItem{}
	}
	return value
}

func nonNilAssignments(value []domain.ItemAssignment) []domain.ItemAssignment {
	if value == nil {
		return []domain.ItemAssignment{}
	}
	return value
}

func readJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("body must contain a single json value")
	}
	return nil
}

func readOptionalJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return nil
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("body must contain a single json value")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeAssignmentStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrorNotFound):
		writeError(w, http.StatusNotFound, "room not found")
	case errors.Is(err, store.ErrorItemNotFound):
		writeError(w, http.StatusBadRequest, "item does not exist in this room")
	case errors.Is(err, store.ErrorParticipantNotFound):
		writeError(w, http.StatusBadRequest, "participant does not exist in this room")
	default:
		writeError(w, http.StatusInternalServerError, "failed to save assignment")
	}
}

func writeStoreError(w http.ResponseWriter, err error, notFoundMessage string) {
	switch {
	case errors.Is(err, store.ErrorNameTaken):
		writeError(w, http.StatusConflict, "participant name is already taken")
	case errors.Is(err, store.ErrorNotFound),
		errors.Is(err, store.ErrorItemNotFound),
		errors.Is(err, store.ErrorParticipantNotFound):
		writeError(w, http.StatusNotFound, notFoundMessage)
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}
