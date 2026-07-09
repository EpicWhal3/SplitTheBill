package room

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"splitthebill/backend/internal/calculation"
	"splitthebill/backend/internal/domain"
	"splitthebill/backend/internal/store"
)

type Handler struct {
	store store.Store
}

func NewHandler(store store.Store) *Handler {
	return &Handler{
		store: store,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")

	if r.URL.Path == "/rooms" && r.Method == http.MethodPost {
		h.createRoom(w, r)
		return
	}

	if len(parts) < 2 || parts[0] != "rooms" {
		writeError(w, http.StatusNotFound, "route not found")
		return
	}

	roomID := parts[1]

	if len(parts) == 2 {
		switch r.Method {
		case http.MethodGet:
			h.getRoom(w, roomID)
			return
		case http.MethodPatch:
			h.updateRoom(w, r, roomID)
			return
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
	}

	if len(parts) == 3 {
		resource := parts[2]

		switch resource {
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
		}
	}

	writeError(w, http.StatusNotFound, "route not found")
}

type createRoomRequest struct {
	Title       string `json:"title"`
	Currency    string `json:"currency"`
	ServiceFee  int64  `json:"service_fee"`
	TipAmount   int64  `json:"tip_amount"`
	Discount    int64  `json:"discount"`
	TotalAmount int64  `json:"total_amount"`
}

func (h *Handler) createRoom(w http.ResponseWriter, r *http.Request) {
	var req createRoomRequest

	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Currency = strings.TrimSpace(req.Currency)

	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	if req.Currency == "" {
		req.Currency = "EUR"
	}

	if req.ServiceFee < 0 || req.TipAmount < 0 || req.Discount < 0 {
		writeError(w, http.StatusBadRequest, "service_fee, tip_amount and discount must be non-negative")
		return
	}

	room := domain.Room{
		Title:       req.Title,
		Currency:    req.Currency,
		ServiceFee:  req.ServiceFee,
		TipAmount:   req.TipAmount,
		Discount:    req.Discount,
		TotalAmount: req.TotalAmount,
	}

	createdRoom, err := h.store.CreateRoom(room)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create room")
		return
	}

	writeJSON(w, http.StatusCreated, createdRoom)
}

type updateRoomRequest struct {
	Title       *string `json:"title"`
	Currency    *string `json:"currency"`
	ServiceFee  *int64  `json:"service_fee"`
	TipAmount   *int64  `json:"tip_amount"`
	Discount    *int64  `json:"discount"`
	TotalAmount *int64  `json:"total_amount"`
}

func (h *Handler) updateRoom(w http.ResponseWriter, r *http.Request, roomID string) {
	room, err := h.store.GetRoom(roomID)
	if err != nil {
		writeStoreError(w, err)
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
		room.Currency = strings.TrimSpace(*req.Currency)
	}

	if req.ServiceFee != nil {
		if *req.ServiceFee < 0 {
			writeError(w, http.StatusBadRequest, "service_fee must be non-negative")
			return
		}
		room.ServiceFee = *req.ServiceFee
	}

	if req.TipAmount != nil {
		if *req.TipAmount < 0 {
			writeError(w, http.StatusBadRequest, "tip_amount must be non-negative")
			return
		}
		room.TipAmount = *req.TipAmount
	}

	if req.Discount != nil {
		if *req.Discount < 0 {
			writeError(w, http.StatusBadRequest, "discount must be non-negative")
			return
		}
		room.Discount = *req.Discount
	}

	if req.TotalAmount != nil {
		room.TotalAmount = *req.TotalAmount
	}

	updatedRoom, err := h.store.UpdateRoom(room)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, updatedRoom)
}

func (h *Handler) getRoom(w http.ResponseWriter, roomID string) {
	room, err := h.store.GetRoom(roomID)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	participants, err := h.store.ListParticipants(roomID)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	items, err := h.store.ListItems(roomID)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	assignments, err := h.store.ListAssignments(roomID)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	if participants == nil {
		participants = []domain.Participant{}
	}

	if items == nil {
		items = []domain.ReceiptItem{}
	}

	if assignments == nil {
		assignments = []domain.ItemAssignment{}
	}

	response := map[string]any{
		"room":         room,
		"participants": participants,
		"items":        items,
		"assignments":  assignments,
	}

	writeJSON(w, http.StatusOK, response)
}

type addParticipantRequest struct {
	Name string `json:"name"`
}

func (h *Handler) addParticipant(w http.ResponseWriter, r *http.Request, roomID string) {
	var req addParticipantRequest

	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	req.Name = strings.TrimSpace(req.Name)

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	participant := domain.Participant{
		Name: req.Name,
	}

	createdParticipant, err := h.store.AddParticipant(roomID, participant)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, createdParticipant)
}

type addItemRequest struct {
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	UnitPrice int64  `json:"unit_price"`
	Total     int64  `json:"total"`
}

func (h *Handler) addItem(w http.ResponseWriter, r *http.Request, roomID string) {
	var req addItemRequest

	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	req.Name = strings.TrimSpace(req.Name)

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	if req.Quantity <= 0 {
		req.Quantity = 1
	}

	if req.UnitPrice < 0 || req.Total < 0 {
		writeError(w, http.StatusBadRequest, "unit_price and total must be non-negative")
		return
	}

	if req.Total == 0 {
		req.Total = int64(req.Quantity) * req.UnitPrice
	}

	if req.Total <= 0 {
		writeError(w, http.StatusBadRequest, "total must be positive")
		return
	}

	item := domain.ReceiptItem{
		Name:      req.Name,
		Quantity:  req.Quantity,
		UnitPrice: req.UnitPrice,
		Total:     req.Total,
	}

	createdItem, err := h.store.AddItem(roomID, item)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, createdItem)
}

type addAssignmentRequest struct {
	ItemID        string `json:"item_id"`
	ParticipantID string `json:"participant_id"`
	Weight        int64  `json:"weight"`
}

func (h *Handler) addAssignment(w http.ResponseWriter, r *http.Request, roomID string) {
	var req addAssignmentRequest

	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	req.ItemID = strings.TrimSpace(req.ItemID)
	req.ParticipantID = strings.TrimSpace(req.ParticipantID)

	if req.ItemID == "" {
		writeError(w, http.StatusBadRequest, "item_id is required")
		return
	}

	if req.ParticipantID == "" {
		writeError(w, http.StatusBadRequest, "participant_id is required")
		return
	}

	if req.Weight <= 0 {
		req.Weight = 1
	}

	assignment := domain.ItemAssignment{
		ItemID:        req.ItemID,
		ParticipantID: req.ParticipantID,
		Weight:        req.Weight,
	}

	createdAssignment, err := h.store.AddAssignment(roomID, assignment)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, createdAssignment)
}

func (h *Handler) calculate(w http.ResponseWriter, roomID string) {
	room, err := h.store.GetRoom(roomID)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	participants, err := h.store.ListParticipants(roomID)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	items, err := h.store.ListItems(roomID)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	assignments, err := h.store.ListAssignments(roomID)
	if err != nil {
		writeStoreError(w, err)
		return
	}

	results, err := calculation.Calculate(calculation.BillInput{
		Participants: participants,
		Items:        items,
		Assignments:  assignments,
		ServiceFee:   room.ServiceFee,
		TipAmount:    room.TipAmount,
		Discount:     room.Discount,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var calculatedTotal int64

	for _, result := range results {
		calculatedTotal += result.TotalAmount
	}

	response := map[string]any{
		"room":             room,
		"results":          results,
		"calculated_total": calculatedTotal,
	}

	writeJSON(w, http.StatusOK, response)
}

func readJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	return decoder.Decode(dst)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error": message,
	})
}

func writeStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrorNotFound) {
		writeError(w, http.StatusNotFound, "room not found")
		return
	}

	writeError(w, http.StatusInternalServerError, err.Error())
}
