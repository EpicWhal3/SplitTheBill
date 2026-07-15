package room

import (
	"encoding/json"
	"errors"
	"io"

	"net/http"
	"strings"
	"unicode/utf8"

	"splitthebill/backend/internal/calculation"
	"splitthebill/backend/internal/domain"
	"splitthebill/backend/internal/store"
)

const (
	maxTitleLength       = 120
	maxParticipantLength = 80
	maxItemNameLength    = 160
)

type Handler struct {
	store store.Store
}

func NewHandler(store store.Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")

	if path == "rooms" {
		if r.Method == http.MethodPost {
			h.createRoom(w, r)
			return
		}

		writeError(
			w,
			http.StatusMethodNotAllowed,
			"method not allowed",
		)
		return
	}

	if len(parts) < 2 ||
		parts[0] != "rooms" ||
		parts[1] == "" {
		writeError(
			w,
			http.StatusNotFound,
			"route not found",
		)
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
			writeError(
				w,
				http.StatusMethodNotAllowed,
				"method not allowed",
			)
		}

		return
	}

	if len(parts) == 3 {
		switch parts[2] {
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

		default:
			writeError(
				w,
				http.StatusNotFound,
				"route not found",
			)
			return
		}

		writeError(
			w,
			http.StatusMethodNotAllowed,
			"method not allowed",
		)
		return
	}

	if len(parts) == 4 {
		resourceID := parts[3]

		switch parts[2] {
		case "participants":
			switch r.Method {
			case http.MethodPatch:
				h.updateParticipant(
					w,
					r,
					roomID,
					resourceID,
				)

			case http.MethodDelete:
				h.deleteParticipant(
					w,
					roomID,
					resourceID,
				)

			default:
				writeError(
					w,
					http.StatusMethodNotAllowed,
					"method not allowed",
				)
			}

			return

		case "items":
			switch r.Method {
			case http.MethodPatch:
				h.updateItem(
					w,
					r,
					roomID,
					resourceID,
				)

			case http.MethodDelete:
				h.deleteItem(
					w,
					roomID,
					resourceID,
				)

			default:
				writeError(
					w,
					http.StatusMethodNotAllowed,
					"method not allowed",
				)
			}

			return
		}
	}

	if len(parts) == 5 &&
		parts[2] == "assignments" {
		if r.Method == http.MethodDelete {
			h.deleteAssignment(
				w,
				roomID,
				parts[3],
				parts[4],
			)
			return
		}

		writeError(
			w,
			http.StatusMethodNotAllowed,
			"method not allowed",
		)
		return
	}

	writeError(
		w,
		http.StatusNotFound,
		"route not found",
	)
}

type createRoomRequest struct {
	Title             string `json:"title"`
	Currency          string `json:"currency"`
	ServiceFee        int64  `json:"service_fee"`
	TipAmount         int64  `json:"tip_amount"`
	Discount          int64  `json:"discount"`
	ExpectedTotal     int64  `json:"expected_total"`
	LegacyTotalAmount *int64 `json:"total_amount"`
}

func (h *Handler) createRoom(
	w http.ResponseWriter,
	r *http.Request,
) {
	var req createRoomRequest

	if err := readJSON(r, &req); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			"invalid json",
		)
		return
	}

	req.Title = strings.TrimSpace(req.Title)

	req.Currency = strings.ToUpper(
		strings.TrimSpace(req.Currency),
	)

	if req.Currency == "" {
		req.Currency = "EUR"
	}

	if req.ExpectedTotal == 0 &&
		req.LegacyTotalAmount != nil {
		req.ExpectedTotal = *req.LegacyTotalAmount
	}

	if err := validateTitle(req.Title); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			err.Error(),
		)
		return
	}

	if err := validateCurrency(req.Currency); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			err.Error(),
		)
		return
	}

	if err := validateCharges(
		req.ServiceFee,
		req.TipAmount,
		req.Discount,
		req.ExpectedTotal,
	); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			err.Error(),
		)
		return
	}

	createdRoom, err := h.store.CreateRoom(
		domain.Room{
			Title:         req.Title,
			Currency:      req.Currency,
			ServiceFee:    req.ServiceFee,
			TipAmount:     req.TipAmount,
			Discount:      req.Discount,
			ExpectedTotal: req.ExpectedTotal,
		},
	)
	if err != nil {
		writeError(
			w,
			http.StatusInternalServerError,
			"failed to create room",
		)
		return
	}

	writeJSON(
		w,
		http.StatusCreated,
		createdRoom,
	)
}

type updateRoomRequest struct {
	Title             *string `json:"title"`
	Currency          *string `json:"currency"`
	ServiceFee        *int64  `json:"service_fee"`
	TipAmount         *int64  `json:"tip_amount"`
	Discount          *int64  `json:"discount"`
	ExpectedTotal     *int64  `json:"expected_total"`
	LegacyTotalAmount *int64  `json:"total_amount"`
}

func (h *Handler) updateRoom(
	w http.ResponseWriter,
	r *http.Request,
	roomID string,
) {
	room, err := h.store.GetRoom(roomID)
	if err != nil {
		writeStoreError(
			w,
			err,
			"room not found",
		)
		return
	}

	var req updateRoomRequest

	if err := readJSON(r, &req); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			"invalid json",
		)
		return
	}

	if req.Title != nil {
		room.Title = strings.TrimSpace(*req.Title)
	}

	if req.Currency != nil {
		room.Currency = strings.ToUpper(
			strings.TrimSpace(*req.Currency),
		)
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

	if req.ExpectedTotal != nil {
		room.ExpectedTotal = *req.ExpectedTotal
	} else if req.LegacyTotalAmount != nil {
		room.ExpectedTotal = *req.LegacyTotalAmount
	}

	if err := validateTitle(room.Title); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			err.Error(),
		)
		return
	}

	if err := validateCurrency(room.Currency); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			err.Error(),
		)
		return
	}

	if err := validateCharges(
		room.ServiceFee,
		room.TipAmount,
		room.Discount,
		room.ExpectedTotal,
	); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			err.Error(),
		)
		return
	}

	updatedRoom, err := h.store.UpdateRoom(room)
	if err != nil {
		writeStoreError(
			w,
			err,
			"room not found",
		)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		updatedRoom,
	)
}

func (h *Handler) getRoom(
	w http.ResponseWriter,
	roomID string,
) {
	room, err := h.store.GetRoom(roomID)
	if err != nil {
		writeStoreError(
			w,
			err,
			"room not found",
		)
		return
	}

	participants, err :=
		h.store.ListParticipants(roomID)
	if err != nil {
		writeStoreError(
			w,
			err,
			"room not found",
		)
		return
	}

	items, err := h.store.ListItems(roomID)
	if err != nil {
		writeStoreError(
			w,
			err,
			"room not found",
		)
		return
	}

	assignments, err :=
		h.store.ListAssignments(roomID)
	if err != nil {
		writeStoreError(
			w,
			err,
			"room not found",
		)
		return
	}

	response := map[string]any{
		"room": room,

		"participants": nonNilParticipants(participants),

		"items": nonNilItems(items),

		"assignments": nonNilAssignments(assignments),

		"subtotal": calculateSubtotal(items),
	}

	writeJSON(
		w,
		http.StatusOK,
		response,
	)
}

type participantRequest struct {
	Name string `json:"name"`
}

func (h *Handler) addParticipant(
	w http.ResponseWriter,
	r *http.Request,
	roomID string,
) {
	var req participantRequest

	if err := readJSON(r, &req); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			"invalid json",
		)
		return
	}

	req.Name = strings.TrimSpace(req.Name)

	if err := validateParticipantName(
		req.Name,
	); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			err.Error(),
		)
		return
	}

	created, err := h.store.AddParticipant(
		roomID,
		domain.Participant{
			Name: req.Name,
		},
	)
	if err != nil {
		writeStoreError(
			w,
			err,
			"room not found",
		)
		return
	}

	writeJSON(
		w,
		http.StatusCreated,
		created,
	)
}

func (h *Handler) updateParticipant(
	w http.ResponseWriter,
	r *http.Request,
	roomID string,
	participantID string,
) {
	var req participantRequest

	if err := readJSON(r, &req); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			"invalid json",
		)
		return
	}

	req.Name = strings.TrimSpace(req.Name)

	if err := validateParticipantName(
		req.Name,
	); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			err.Error(),
		)
		return
	}

	updated, err := h.store.UpdateParticipant(
		roomID,
		domain.Participant{
			ID:     participantID,
			RoomID: roomID,
			Name:   req.Name,
		},
	)
	if err != nil {
		writeStoreError(
			w,
			err,
			"participant not found",
		)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		updated,
	)
}

func (h *Handler) deleteParticipant(
	w http.ResponseWriter,
	roomID string,
	participantID string,
) {
	if err := h.store.DeleteParticipant(
		roomID,
		participantID,
	); err != nil {
		writeStoreError(
			w,
			err,
			"participant not found",
		)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type addItemRequest struct {
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	UnitPrice int64  `json:"unit_price"`
	Total     int64  `json:"total"`
}

func (h *Handler) addItem(
	w http.ResponseWriter,
	r *http.Request,
	roomID string,
) {
	var req addItemRequest

	if err := readJSON(r, &req); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			"invalid json",
		)
		return
	}

	req.Name = strings.TrimSpace(req.Name)

	if req.Quantity == 0 {
		req.Quantity = 1
	}

	if err := validateItem(
		req.Name,
		req.Quantity,
		req.UnitPrice,
	); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			err.Error(),
		)
		return
	}

	item := domain.ReceiptItem{
		Name:      req.Name,
		Quantity:  req.Quantity,
		UnitPrice: req.UnitPrice,

		Total: int64(req.Quantity) *
			req.UnitPrice,
	}

	created, err := h.store.AddItem(
		roomID,
		item,
	)
	if err != nil {
		writeStoreError(
			w,
			err,
			"room not found",
		)
		return
	}

	writeJSON(
		w,
		http.StatusCreated,
		created,
	)
}

type updateItemRequest struct {
	Name      *string `json:"name"`
	Quantity  *int    `json:"quantity"`
	UnitPrice *int64  `json:"unit_price"`
}

func (h *Handler) updateItem(
	w http.ResponseWriter,
	r *http.Request,
	roomID string,
	itemID string,
) {
	item, err := h.findItem(
		roomID,
		itemID,
	)
	if err != nil {
		writeStoreError(
			w,
			err,
			"item not found",
		)
		return
	}

	var req updateItemRequest

	if err := readJSON(r, &req); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			"invalid json",
		)
		return
	}

	if req.Name != nil {
		item.Name = strings.TrimSpace(
			*req.Name,
		)
	}

	if req.Quantity != nil {
		item.Quantity = *req.Quantity
	}

	if req.UnitPrice != nil {
		item.UnitPrice = *req.UnitPrice
	}

	if err := validateItem(
		item.Name,
		item.Quantity,
		item.UnitPrice,
	); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			err.Error(),
		)
		return
	}

	item.Total =
		int64(item.Quantity) *
			item.UnitPrice

	updated, err := h.store.UpdateItem(
		roomID,
		item,
	)
	if err != nil {
		writeStoreError(
			w,
			err,
			"item not found",
		)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		updated,
	)
}

func (h *Handler) deleteItem(
	w http.ResponseWriter,
	roomID string,
	itemID string,
) {
	if err := h.store.DeleteItem(
		roomID,
		itemID,
	); err != nil {
		writeStoreError(
			w,
			err,
			"item not found",
		)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type addAssignmentRequest struct {
	ItemID        string `json:"item_id"`
	ParticipantID string `json:"participant_id"`
	Weight        int64  `json:"weight"`
}

func (h *Handler) addAssignment(
	w http.ResponseWriter,
	r *http.Request,
	roomID string,
) {
	var req addAssignmentRequest

	if err := readJSON(r, &req); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			"invalid json",
		)
		return
	}

	req.ItemID = strings.TrimSpace(
		req.ItemID,
	)

	req.ParticipantID = strings.TrimSpace(
		req.ParticipantID,
	)

	if req.ItemID == "" {
		writeError(
			w,
			http.StatusBadRequest,
			"item_id is required",
		)
		return
	}

	if req.ParticipantID == "" {
		writeError(
			w,
			http.StatusBadRequest,
			"participant_id is required",
		)
		return
	}

	if req.Weight <= 0 {
		req.Weight = 1
	}

	created, err := h.store.AddAssignment(
		roomID,
		domain.ItemAssignment{
			ItemID:        req.ItemID,
			ParticipantID: req.ParticipantID,
			Weight:        req.Weight,
		},
	)
	if err != nil {
		switch {
		case errors.Is(
			err,
			store.ErrorNotFound,
		):
			writeError(
				w,
				http.StatusNotFound,
				"room not found",
			)

		case errors.Is(
			err,
			store.ErrorItemNotFound,
		):
			writeError(
				w,
				http.StatusBadRequest,
				"item does not exist in this room",
			)

		case errors.Is(
			err,
			store.ErrorParticipantNotFound,
		):
			writeError(
				w,
				http.StatusBadRequest,
				"participant does not exist in this room",
			)

		default:
			writeError(
				w,
				http.StatusInternalServerError,
				"failed to save assignment",
			)
		}

		return
	}

	writeJSON(
		w,
		http.StatusCreated,
		created,
	)
}

func (h *Handler) deleteAssignment(
	w http.ResponseWriter,
	roomID string,
	itemID string,
	participantID string,
) {
	if err := h.store.DeleteAssignment(
		roomID,
		itemID,
		participantID,
	); err != nil {
		writeStoreError(
			w,
			err,
			"assignment not found",
		)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) calculate(
	w http.ResponseWriter,
	roomID string,
) {
	room, err := h.store.GetRoom(roomID)
	if err != nil {
		writeStoreError(
			w,
			err,
			"room not found",
		)
		return
	}

	participants, err :=
		h.store.ListParticipants(roomID)
	if err != nil {
		writeStoreError(
			w,
			err,
			"room not found",
		)
		return
	}

	items, err := h.store.ListItems(roomID)
	if err != nil {
		writeStoreError(
			w,
			err,
			"room not found",
		)
		return
	}

	assignments, err :=
		h.store.ListAssignments(roomID)
	if err != nil {
		writeStoreError(
			w,
			err,
			"room not found",
		)
		return
	}

	results, err := calculation.Calculate(
		calculation.BillInput{
			Participants: participants,
			Items:        items,
			Assignments:  assignments,
			ServiceFee:   room.ServiceFee,
			TipAmount:    room.TipAmount,
			Discount:     room.Discount,
		},
	)
	if err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			err.Error(),
		)
		return
	}

	var calculatedTotal int64

	for _, result := range results {
		calculatedTotal += result.TotalAmount
	}

	difference := int64(0)
	matchesExpected := true

	if room.ExpectedTotal > 0 {
		difference =
			calculatedTotal -
				room.ExpectedTotal

		matchesExpected =
			difference == 0
	}

	writeJSON(
		w,
		http.StatusOK,
		map[string]any{
			"room": room,

			"results": results,

			"subtotal": calculateSubtotal(items),

			"calculated_total": calculatedTotal,

			"difference": difference,

			"matches_expected_total": matchesExpected,
		},
	)
}

func (h *Handler) findItem(
	roomID string,
	itemID string,
) (domain.ReceiptItem, error) {
	items, err := h.store.ListItems(roomID)
	if err != nil {
		return domain.ReceiptItem{}, err
	}

	for _, item := range items {
		if item.ID == itemID {
			return item, nil
		}
	}

	return domain.ReceiptItem{},
		store.ErrorItemNotFound
}

func validateTitle(value string) error {
	if value == "" {
		return errors.New("title is required")
	}

	if utf8.RuneCountInString(value) >
		maxTitleLength {
		return errors.New("title is too long")
	}

	return nil
}

func validateCurrency(value string) error {
	if len(value) != 3 {
		return errors.New(
			"currency must be a three-letter code",
		)
	}

	for _, char := range value {
		if char < 'A' || char > 'Z' {
			return errors.New(
				"currency must contain only latin letters",
			)
		}
	}

	return nil
}

func validateCharges(
	serviceFee int64,
	tipAmount int64,
	discount int64,
	expectedTotal int64,
) error {
	if serviceFee < 0 ||
		tipAmount < 0 ||
		discount < 0 ||
		expectedTotal < 0 {
		return errors.New(
			"service_fee, tip_amount, discount and expected_total must be non-negative",
		)
	}

	return nil
}

func validateParticipantName(
	value string,
) error {
	if value == "" {
		return errors.New("name is required")
	}

	if utf8.RuneCountInString(value) >
		maxParticipantLength {
		return errors.New(
			"participant name is too long",
		)
	}

	return nil
}

func validateItem(
	name string,
	quantity int,
	unitPrice int64,
) error {
	if name == "" {
		return errors.New("name is required")
	}

	if utf8.RuneCountInString(name) >
		maxItemNameLength {
		return errors.New("item name is too long")
	}

	if quantity <= 0 {
		return errors.New(
			"quantity must be positive",
		)
	}

	if unitPrice <= 0 {
		return errors.New(
			"unit_price must be positive",
		)
	}

	return nil
}

func calculateSubtotal(
	items []domain.ReceiptItem,
) int64 {
	var subtotal int64

	for _, item := range items {
		subtotal += item.Total
	}

	return subtotal
}

func nonNilParticipants(
	value []domain.Participant,
) []domain.Participant {
	if value == nil {
		return []domain.Participant{}
	}

	return value
}

func nonNilItems(
	value []domain.ReceiptItem,
) []domain.ReceiptItem {
	if value == nil {
		return []domain.ReceiptItem{}
	}

	return value
}

func nonNilAssignments(
	value []domain.ItemAssignment,
) []domain.ItemAssignment {
	if value == nil {
		return []domain.ItemAssignment{}
	}

	return value
}

func readJSON(
	r *http.Request,
	dst any,
) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return err
	}

	if err := decoder.Decode(
		&struct{}{},
	); !errors.Is(err, io.EOF) {
		return errors.New(
			"body must contain a single json value",
		)
	}

	return nil
}

func writeJSON(
	w http.ResponseWriter,
	status int,
	data any,
) {
	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(data)
}

func writeError(
	w http.ResponseWriter,
	status int,
	message string,
) {
	writeJSON(
		w,
		status,
		map[string]string{
			"error": message,
		},
	)
}

func writeStoreError(
	w http.ResponseWriter,
	err error,
	notFoundMessage string,
) {
	if errors.Is(err, store.ErrorNotFound) ||
		errors.Is(err, store.ErrorItemNotFound) ||
		errors.Is(err, store.ErrorParticipantNotFound) {
		writeError(
			w,
			http.StatusNotFound,
			notFoundMessage,
		)
		return
	}

	writeError(
		w,
		http.StatusInternalServerError,
		"internal server error",
	)
}
