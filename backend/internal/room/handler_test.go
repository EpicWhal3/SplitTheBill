package room

import (
	"bytes"
	"encoding/json"
	"net/http"

	"net/http/httptest"
	"testing"

	"splitthebill/backend/internal/domain"
	"splitthebill/backend/internal/store"
)

type createRoomResponse struct {
	Room       domain.Room `json:"room"`
	AdminToken string      `json:"admin_token"`
}

type joinResponse struct {
	Participant      domain.Participant `json:"participant"`
	ParticipantToken string             `json:"participant_token"`
}

type calculationPayload struct {
	Room                 domain.Room                `json:"room"`
	Results              []domain.ParticipantResult `json:"results"`
	Debts                []domain.Debt              `json:"debts"`
	CalculatedTotal      int64                      `json:"calculated_total"`
	Difference           int64                      `json:"difference"`
	MatchesExpectedTotal bool                       `json:"matches_expected_total"`
}

func TestCollaborativeRoomLifecycle(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	created := doJSON[createRoomResponse](
		t,
		handler,
		http.MethodPost,
		"/rooms",
		map[string]any{
			"title":          "Dinner",
			"currency":       "EUR",
			"expected_total": 1000,
			"discount_mode":  "equal",
		},
		nil,
		http.StatusCreated,
	)

	if created.AdminToken == "" || created.Room.Status != domain.RoomStatusDraft {
		t.Fatalf("unexpected created room: %#v", created)
	}

	doJSON[map[string]string](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+created.Room.ID+"/participants",
		map[string]any{"name": "Аня"},
		nil,
		http.StatusUnauthorized,
	)

	participant := doJSON[domain.Participant](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+created.Room.ID+"/participants",
		map[string]any{"name": "Аня"},
		map[string]string{"X-Admin-Token": created.AdminToken},
		http.StatusCreated,
	)

	joined := doJSON[joinResponse](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+created.Room.ID+"/join",
		map[string]any{"name": "аня"},
		nil,
		http.StatusCreated,
	)

	item := doJSON[domain.ReceiptItem](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+created.Room.ID+"/items",
		map[string]any{
			"name":       "Pizza",
			"quantity":   1,
			"unit_price": 1000,
		},
		map[string]string{"X-Admin-Token": created.AdminToken},
		http.StatusCreated,
	)

	opened := doJSON[domain.Room](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+created.Room.ID+"/open",
		nil,
		map[string]string{"X-Admin-Token": created.AdminToken},
		http.StatusOK,
	)
	if opened.Status != domain.RoomStatusClaiming {
		t.Fatalf("expected claiming, got %s", opened.Status)
	}

	selection := doJSON[domain.ItemAssignment](
		t,
		handler,
		http.MethodPut,
		"/rooms/"+created.Room.ID+"/selections/"+item.ID,
		map[string]any{"weight": 2},
		map[string]string{"X-Participant-Token": joined.ParticipantToken},
		http.StatusOK,
	)
	if selection.ParticipantID != participant.ID || selection.Weight != 2 {
		t.Fatalf("unexpected selection: %#v", selection)
	}

	selection = doJSON[domain.ItemAssignment](
		t,
		handler,
		http.MethodPut,
		"/rooms/"+created.Room.ID+"/selections/"+item.ID,
		map[string]any{"weight": 3},
		map[string]string{"X-Participant-Token": joined.ParticipantToken},
		http.StatusOK,
	)
	if selection.Weight != 3 {
		t.Fatalf("participant must be able to update own weight: %#v", selection)
	}

	updatedRoom := doJSON[domain.Room](
		t,
		handler,
		http.MethodPatch,
		"/rooms/"+created.Room.ID,
		map[string]any{"payer_participant_id": participant.ID},
		map[string]string{"X-Admin-Token": created.AdminToken},
		http.StatusOK,
	)
	if updatedRoom.PayerParticipantID != participant.ID {
		t.Fatalf("expected payer %s, got %s", participant.ID, updatedRoom.PayerParticipantID)
	}

	finalized := doJSON[calculationPayload](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+created.Room.ID+"/finalize",
		nil,
		map[string]string{"X-Admin-Token": created.AdminToken},
		http.StatusOK,
	)
	if finalized.Room.Status != domain.RoomStatusFinalized || finalized.Room.FinalizedAt == nil {
		t.Fatalf("room was not finalized: %#v", finalized.Room)
	}
	if finalized.CalculatedTotal != 1000 || !finalized.MatchesExpectedTotal {
		t.Fatalf("unexpected calculation: %#v", finalized)
	}

	doJSON[map[string]string](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+created.Room.ID+"/items",
		map[string]any{"name": "Blocked", "quantity": 1, "unit_price": 100},
		map[string]string{"X-Admin-Token": created.AdminToken},
		http.StatusConflict,
	)

	reopened := doJSON[domain.Room](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+created.Room.ID+"/reopen",
		nil,
		map[string]string{"X-Admin-Token": created.AdminToken},
		http.StatusOK,
	)
	if reopened.Status != domain.RoomStatusClaiming || reopened.FinalizedAt != nil {
		t.Fatalf("unexpected reopened room: %#v", reopened)
	}
}

func TestFinalizeBuildsDebtToPayer(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room, _ := memoryStore.CreateRoom(domain.Room{
		Title: "Dinner", Currency: "EUR", ExpectedTotal: 1000,
		Status: domain.RoomStatusClaiming,
	})
	payer, _ := memoryStore.AddParticipant(room.ID, domain.Participant{Name: "Max"})
	guest, _ := memoryStore.AddParticipant(room.ID, domain.Participant{Name: "Anna"})
	item, _ := memoryStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Pizza", Quantity: 1, UnitPrice: 1000, Total: 1000,
	})
	_, _ = memoryStore.AddAssignment(room.ID, domain.ItemAssignment{
		ItemID: item.ID, ParticipantID: payer.ID, Weight: 1,
	})
	_, _ = memoryStore.AddAssignment(room.ID, domain.ItemAssignment{
		ItemID: item.ID, ParticipantID: guest.ID, Weight: 1,
	})
	room.PayerParticipantID = payer.ID
	_, _ = memoryStore.UpdateRoom(room)

	result := doJSON[calculationPayload](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/finalize",
		nil,
		map[string]string{"X-Admin-Token": room.AdminToken},
		http.StatusOK,
	)

	if len(result.Debts) != 1 || result.Debts[0].FromParticipantID != guest.ID || result.Debts[0].Amount != 500 {
		t.Fatalf("unexpected debts: %#v", result.Debts)
	}
}

func TestParticipantCanOnlyUseValidSession(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room, _ := memoryStore.CreateRoom(domain.Room{
		Title: "Dinner", Currency: "EUR", Status: domain.RoomStatusClaiming,
	})
	item, _ := memoryStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Pizza", Quantity: 1, UnitPrice: 1000, Total: 1000,
	})

	doJSON[map[string]string](
		t,
		handler,
		http.MethodPut,
		"/rooms/"+room.ID+"/selections/"+item.ID,
		map[string]any{"weight": 1},
		map[string]string{"X-Participant-Token": "invalid"},
		http.StatusUnauthorized,
	)
}

func TestCreateRoomDefaults(t *testing.T) {
	handler := NewHandler(store.NewMemoryStore())

	created := doJSON[createRoomResponse](
		t,
		handler,
		http.MethodPost,
		"/rooms",
		map[string]any{"title": "Dinner", "currency": "EUR"},
		nil,
		http.StatusCreated,
	)

	if created.Room.Status != domain.RoomStatusDraft {
		t.Fatalf("expected draft status, got %q", created.Room.Status)
	}
	if created.Room.DiscountMode != domain.DiscountModeProportional {
		t.Fatalf("expected proportional discount mode, got %q", created.Room.DiscountMode)
	}
	if created.AdminToken == "" {
		t.Fatal("expected admin token")
	}
}

func TestCreateRoomWithEqualDiscountMode(t *testing.T) {
	handler := NewHandler(store.NewMemoryStore())

	created := doJSON[createRoomResponse](
		t,
		handler,
		http.MethodPost,
		"/rooms",
		map[string]any{
			"title":         "Dinner",
			"currency":      "EUR",
			"discount_mode": "equal",
		},
		nil,
		http.StatusCreated,
	)

	if created.Room.DiscountMode != domain.DiscountModeEqual {
		t.Fatalf("expected equal discount mode, got %q", created.Room.DiscountMode)
	}
}

func TestCreateRoomRejectsInvalidDiscountMode(t *testing.T) {
	handler := NewHandler(store.NewMemoryStore())

	doJSON[map[string]string](
		t,
		handler,
		http.MethodPost,
		"/rooms",
		map[string]any{
			"title":         "Dinner",
			"currency":      "EUR",
			"discount_mode": "unknown",
		},
		nil,
		http.StatusBadRequest,
	)
}

func TestUpdateRoomChangesDiscountMode(t *testing.T) {
	handler := NewHandler(store.NewMemoryStore())

	created := doJSON[createRoomResponse](
		t,
		handler,
		http.MethodPost,
		"/rooms",
		map[string]any{"title": "Dinner", "currency": "EUR"},
		nil,
		http.StatusCreated,
	)

	updated := doJSON[domain.Room](
		t,
		handler,
		http.MethodPatch,
		"/rooms/"+created.Room.ID,
		map[string]any{"discount_mode": "equal"},
		map[string]string{"X-Admin-Token": created.AdminToken},
		http.StatusOK,
	)

	if updated.DiscountMode != domain.DiscountModeEqual {
		t.Fatalf("expected equal discount mode, got %q", updated.DiscountMode)
	}
}

func TestUpdateRoomRejectsInvalidDiscountMode(t *testing.T) {
	handler := NewHandler(store.NewMemoryStore())

	created := doJSON[createRoomResponse](
		t,
		handler,
		http.MethodPost,
		"/rooms",
		map[string]any{"title": "Dinner", "currency": "EUR"},
		nil,
		http.StatusCreated,
	)

	doJSON[map[string]string](
		t,
		handler,
		http.MethodPatch,
		"/rooms/"+created.Room.ID,
		map[string]any{"discount_mode": "unknown"},
		map[string]string{"X-Admin-Token": created.AdminToken},
		http.StatusBadRequest,
	)
}

func TestUpdateRoomRejectsFinalizedRoom(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room, _ := memoryStore.CreateRoom(domain.Room{
		Title: "Dinner", Currency: "EUR", Status: domain.RoomStatusFinalized,
	})

	doJSON[map[string]string](
		t,
		handler,
		http.MethodPatch,
		"/rooms/"+room.ID,
		map[string]any{"title": "Changed"},
		map[string]string{"X-Admin-Token": room.AdminToken},
		http.StatusConflict,
	)
}

func TestOpenClaimingRequiresItems(t *testing.T) {
	handler := NewHandler(store.NewMemoryStore())

	created := doJSON[createRoomResponse](
		t,
		handler,
		http.MethodPost,
		"/rooms",
		map[string]any{"title": "Dinner", "currency": "EUR"},
		nil,
		http.StatusCreated,
	)

	doJSON[map[string]string](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+created.Room.ID+"/open",
		nil,
		map[string]string{"X-Admin-Token": created.AdminToken},
		http.StatusConflict,
	)
}

func TestOpenClaimingRequiresAdmin(t *testing.T) {
	handler := NewHandler(store.NewMemoryStore())

	created := doJSON[createRoomResponse](
		t,
		handler,
		http.MethodPost,
		"/rooms",
		map[string]any{"title": "Dinner", "currency": "EUR"},
		nil,
		http.StatusCreated,
	)
	doJSON[domain.ReceiptItem](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+created.Room.ID+"/items",
		map[string]any{"name": "Pizza", "quantity": 1, "unit_price": 1000},
		map[string]string{"X-Admin-Token": created.AdminToken},
		http.StatusCreated,
	)

	doJSON[map[string]string](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+created.Room.ID+"/open",
		nil,
		nil,
		http.StatusUnauthorized,
	)
}

func TestOpenClaimingSetsClaimingStatus(t *testing.T) {
	handler := NewHandler(store.NewMemoryStore())

	created := doJSON[createRoomResponse](
		t,
		handler,
		http.MethodPost,
		"/rooms",
		map[string]any{"title": "Dinner", "currency": "EUR"},
		nil,
		http.StatusCreated,
	)
	doJSON[domain.ReceiptItem](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+created.Room.ID+"/items",
		map[string]any{"name": "Pizza", "quantity": 1, "unit_price": 1000},
		map[string]string{"X-Admin-Token": created.AdminToken},
		http.StatusCreated,
	)

	opened := doJSON[domain.Room](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+created.Room.ID+"/open",
		nil,
		map[string]string{"X-Admin-Token": created.AdminToken},
		http.StatusOK,
	)

	if opened.Status != domain.RoomStatusClaiming {
		t.Fatalf("expected claiming status, got %q", opened.Status)
	}
}

func TestSelectionRejectedInDraftStatus(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room, _ := memoryStore.CreateRoom(domain.Room{
		Title: "Dinner", Currency: "EUR", Status: domain.RoomStatusDraft,
	})
	joined := doJSON[joinResponse](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/join",
		map[string]any{"name": "Аня"},
		nil,
		http.StatusCreated,
	)
	item, _ := memoryStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Pizza", Quantity: 1, UnitPrice: 1000, Total: 1000,
	})

	doJSON[map[string]string](
		t,
		handler,
		http.MethodPut,
		"/rooms/"+room.ID+"/selections/"+item.ID,
		map[string]any{"weight": 1},
		map[string]string{"X-Participant-Token": joined.ParticipantToken},
		http.StatusConflict,
	)
}

func TestSelectionAllowedInClaimingStatus(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room, _ := memoryStore.CreateRoom(domain.Room{
		Title: "Dinner", Currency: "EUR", Status: domain.RoomStatusClaiming,
	})
	joined := doJSON[joinResponse](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/join",
		map[string]any{"name": "Аня"},
		nil,
		http.StatusCreated,
	)
	item, _ := memoryStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Pizza", Quantity: 1, UnitPrice: 1000, Total: 1000,
	})

	selection := doJSON[domain.ItemAssignment](
		t,
		handler,
		http.MethodPut,
		"/rooms/"+room.ID+"/selections/"+item.ID,
		map[string]any{"weight": 1},
		map[string]string{"X-Participant-Token": joined.ParticipantToken},
		http.StatusOK,
	)

	if selection.ParticipantID != joined.Participant.ID || selection.Weight != 1 {
		t.Fatalf("unexpected selection: %#v", selection)
	}
}

func TestSelectionWithWeight(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room, _ := memoryStore.CreateRoom(domain.Room{
		Title: "Dinner", Currency: "EUR", Status: domain.RoomStatusClaiming,
	})
	joined := doJSON[joinResponse](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/join",
		map[string]any{"name": "Аня"},
		nil,
		http.StatusCreated,
	)
	item, _ := memoryStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Pizza", Quantity: 1, UnitPrice: 1000, Total: 1000,
	})

	selection := doJSON[domain.ItemAssignment](
		t,
		handler,
		http.MethodPut,
		"/rooms/"+room.ID+"/selections/"+item.ID,
		map[string]any{"weight": 7},
		map[string]string{"X-Participant-Token": joined.ParticipantToken},
		http.StatusOK,
	)

	if selection.Weight != 7 {
		t.Fatalf("expected weight 7, got %d", selection.Weight)
	}
}

func TestSelectionRejectsInvalidWeight(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room, _ := memoryStore.CreateRoom(domain.Room{
		Title: "Dinner", Currency: "EUR", Status: domain.RoomStatusClaiming,
	})
	joined := doJSON[joinResponse](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/join",
		map[string]any{"name": "Аня"},
		nil,
		http.StatusCreated,
	)
	item, _ := memoryStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Pizza", Quantity: 1, UnitPrice: 1000, Total: 1000,
	})

	for _, weight := range []int64{0, -1, 1001} {
		doJSON[map[string]string](
			t,
			handler,
			http.MethodPut,
			"/rooms/"+room.ID+"/selections/"+item.ID,
			map[string]any{"weight": weight},
			map[string]string{"X-Participant-Token": joined.ParticipantToken},
			http.StatusBadRequest,
		)
	}
}

func TestSelectionRejectedAfterFinalization(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room, _ := memoryStore.CreateRoom(domain.Room{
		Title: "Dinner", Currency: "EUR", Status: domain.RoomStatusFinalized,
	})
	joined := doJSON[joinResponse](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/join",
		map[string]any{"name": "Аня"},
		nil,
		http.StatusConflict,
	)
	_ = joined

	item, _ := memoryStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Pizza", Quantity: 1, UnitPrice: 1000, Total: 1000,
	})

	doJSON[map[string]string](
		t,
		handler,
		http.MethodPut,
		"/rooms/"+room.ID+"/selections/"+item.ID,
		map[string]any{"weight": 1},
		map[string]string{"X-Participant-Token": "any"},
		http.StatusUnauthorized,
	)
}

func TestUnselectRemovesAssignment(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room, _ := memoryStore.CreateRoom(domain.Room{
		Title: "Dinner", Currency: "EUR", Status: domain.RoomStatusClaiming,
	})
	joined := doJSON[joinResponse](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/join",
		map[string]any{"name": "Аня"},
		nil,
		http.StatusCreated,
	)
	item, _ := memoryStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Pizza", Quantity: 1, UnitPrice: 1000, Total: 1000,
	})

	doJSON[domain.ItemAssignment](
		t,
		handler,
		http.MethodPut,
		"/rooms/"+room.ID+"/selections/"+item.ID,
		map[string]any{"weight": 1},
		map[string]string{"X-Participant-Token": joined.ParticipantToken},
		http.StatusOK,
	)

	doJSON[map[string]string](
		t,
		handler,
		http.MethodDelete,
		"/rooms/"+room.ID+"/selections/"+item.ID,
		nil,
		map[string]string{"X-Participant-Token": joined.ParticipantToken},
		http.StatusNoContent,
	)

	assignments, err := memoryStore.ListAssignments(room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(assignments) != 0 {
		t.Fatalf("expected no assignments after unselect, got %#v", assignments)
	}
}

func TestUnselectMissingSelectionReturnsNotFound(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room, _ := memoryStore.CreateRoom(domain.Room{
		Title: "Dinner", Currency: "EUR", Status: domain.RoomStatusClaiming,
	})
	joined := doJSON[joinResponse](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/join",
		map[string]any{"name": "Аня"},
		nil,
		http.StatusCreated,
	)
	item, _ := memoryStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Pizza", Quantity: 1, UnitPrice: 1000, Total: 1000,
	})

	doJSON[map[string]string](
		t,
		handler,
		http.MethodDelete,
		"/rooms/"+room.ID+"/selections/"+item.ID,
		nil,
		map[string]string{"X-Participant-Token": joined.ParticipantToken},
		http.StatusNotFound,
	)
}

func TestFinalizeRejectedFromDraft(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room, _ := memoryStore.CreateRoom(domain.Room{
		Title: "Dinner", Currency: "EUR", Status: domain.RoomStatusDraft,
	})

	doJSON[map[string]string](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/finalize",
		nil,
		map[string]string{"X-Admin-Token": room.AdminToken},
		http.StatusConflict,
	)
}

func TestFinalizeRequiresPayer(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room, _ := memoryStore.CreateRoom(domain.Room{
		Title: "Dinner", Currency: "EUR", Status: domain.RoomStatusClaiming,
	})
	participant, _ := memoryStore.AddParticipant(room.ID, domain.Participant{Name: "Max"})
	item, _ := memoryStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Pizza", Quantity: 1, UnitPrice: 1000, Total: 1000,
	})
	_, _ = memoryStore.AddAssignment(room.ID, domain.ItemAssignment{
		ItemID: item.ID, ParticipantID: participant.ID, Weight: 1,
	})

	doJSON[map[string]string](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/finalize",
		nil,
		map[string]string{"X-Admin-Token": room.AdminToken},
		http.StatusConflict,
	)
}

func TestFinalizeRejectsUnassignedItems(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room, _ := memoryStore.CreateRoom(domain.Room{
		Title: "Dinner", Currency: "EUR", Status: domain.RoomStatusClaiming,
	})
	payer, _ := memoryStore.AddParticipant(room.ID, domain.Participant{Name: "Max"})
	_, _ = memoryStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Pizza", Quantity: 1, UnitPrice: 1000, Total: 1000,
	})
	room.PayerParticipantID = payer.ID
	_, _ = memoryStore.UpdateRoom(room)

	doJSON[map[string]string](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/finalize",
		nil,
		map[string]string{"X-Admin-Token": room.AdminToken},
		http.StatusConflict,
	)
}

func TestFinalizeRejectsMismatchedExpectedTotal(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room, _ := memoryStore.CreateRoom(domain.Room{
		Title: "Dinner", Currency: "EUR", Status: domain.RoomStatusClaiming,
		ExpectedTotal: 500,
	})
	payer, _ := memoryStore.AddParticipant(room.ID, domain.Participant{Name: "Max"})
	item, _ := memoryStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Pizza", Quantity: 1, UnitPrice: 1000, Total: 1000,
	})
	_, _ = memoryStore.AddAssignment(room.ID, domain.ItemAssignment{
		ItemID: item.ID, ParticipantID: payer.ID, Weight: 1,
	})
	room.PayerParticipantID = payer.ID
	_, _ = memoryStore.UpdateRoom(room)

	doJSON[map[string]string](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/finalize",
		nil,
		map[string]string{"X-Admin-Token": room.AdminToken},
		http.StatusConflict,
	)
}

func TestFinalizeSuccessSetsStatusAndTimestamp(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room, _ := memoryStore.CreateRoom(domain.Room{
		Title: "Dinner", Currency: "EUR", Status: domain.RoomStatusClaiming,
		ExpectedTotal: 1000,
	})
	payer, _ := memoryStore.AddParticipant(room.ID, domain.Participant{Name: "Max"})
	guest, _ := memoryStore.AddParticipant(room.ID, domain.Participant{Name: "Anna"})
	item, _ := memoryStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Pizza", Quantity: 1, UnitPrice: 1000, Total: 1000,
	})
	_, _ = memoryStore.AddAssignment(room.ID, domain.ItemAssignment{
		ItemID: item.ID, ParticipantID: payer.ID, Weight: 1,
	})
	_, _ = memoryStore.AddAssignment(room.ID, domain.ItemAssignment{
		ItemID: item.ID, ParticipantID: guest.ID, Weight: 1,
	})
	room.PayerParticipantID = payer.ID
	_, _ = memoryStore.UpdateRoom(room)

	result := doJSON[calculationPayload](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/finalize",
		nil,
		map[string]string{"X-Admin-Token": room.AdminToken},
		http.StatusOK,
	)

	if result.Room.Status != domain.RoomStatusFinalized {
		t.Fatalf("expected finalized status, got %q", result.Room.Status)
	}
	if result.Room.FinalizedAt == nil {
		t.Fatal("expected finalized_at to be set")
	}
	if len(result.Results) != 2 {
		t.Fatalf("expected 2 results, got %#v", result.Results)
	}
	if len(result.Debts) != 1 || result.Debts[0].Amount != 500 {
		t.Fatalf("expected single debt of 500, got %#v", result.Debts)
	}
}

func TestReopenRequiresFinalizedStatus(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room, _ := memoryStore.CreateRoom(domain.Room{
		Title: "Dinner", Currency: "EUR", Status: domain.RoomStatusClaiming,
	})

	doJSON[map[string]string](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/reopen",
		nil,
		map[string]string{"X-Admin-Token": room.AdminToken},
		http.StatusConflict,
	)
}

func TestReopenAllowsSelectionsAndRefinalization(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room, _ := memoryStore.CreateRoom(domain.Room{
		Title: "Dinner", Currency: "EUR", Status: domain.RoomStatusClaiming,
		ExpectedTotal: 1000,
	})
	payer, _ := memoryStore.AddParticipant(room.ID, domain.Participant{Name: "Max"})
	item, _ := memoryStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Pizza", Quantity: 1, UnitPrice: 1000, Total: 1000,
	})
	_, _ = memoryStore.AddAssignment(room.ID, domain.ItemAssignment{
		ItemID: item.ID, ParticipantID: payer.ID, Weight: 1,
	})
	room.PayerParticipantID = payer.ID
	_, _ = memoryStore.UpdateRoom(room)

	doJSON[calculationPayload](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/finalize",
		nil,
		map[string]string{"X-Admin-Token": room.AdminToken},
		http.StatusOK,
	)

	reopened := doJSON[domain.Room](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/reopen",
		nil,
		map[string]string{"X-Admin-Token": room.AdminToken},
		http.StatusOK,
	)
	if reopened.Status != domain.RoomStatusClaiming || reopened.FinalizedAt != nil {
		t.Fatalf("unexpected reopened room: %#v", reopened)
	}

	joined := doJSON[joinResponse](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/join",
		map[string]any{"name": "Anna"},
		nil,
		http.StatusCreated,
	)
	doJSON[domain.ItemAssignment](
		t,
		handler,
		http.MethodPut,
		"/rooms/"+room.ID+"/selections/"+item.ID,
		map[string]any{"weight": 1},
		map[string]string{"X-Participant-Token": joined.ParticipantToken},
		http.StatusOK,
	)

	refinalized := doJSON[calculationPayload](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/finalize",
		nil,
		map[string]string{"X-Admin-Token": room.AdminToken},
		http.StatusOK,
	)
	if refinalized.Room.Status != domain.RoomStatusFinalized {
		t.Fatalf("expected finalized status, got %q", refinalized.Room.Status)
	}
}

func TestCalculateWithoutPayerReturnsEmptyDebts(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room, _ := memoryStore.CreateRoom(domain.Room{
		Title: "Dinner", Currency: "EUR", Status: domain.RoomStatusClaiming,
	})
	participant, _ := memoryStore.AddParticipant(room.ID, domain.Participant{Name: "Max"})
	item, _ := memoryStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Pizza", Quantity: 1, UnitPrice: 1000, Total: 1000,
	})
	_, _ = memoryStore.AddAssignment(room.ID, domain.ItemAssignment{
		ItemID: item.ID, ParticipantID: participant.ID, Weight: 1,
	})

	result := doJSON[calculationPayload](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/calculate",
		nil,
		nil,
		http.StatusOK,
	)

	if len(result.Debts) != 0 {
		t.Fatalf("expected empty debts without payer, got %#v", result.Debts)
	}
	if result.CalculatedTotal != 1000 {
		t.Fatalf("expected calculated total 1000, got %d", result.CalculatedTotal)
	}
}

func TestCalculateWithPayerBuildsDebts(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room, _ := memoryStore.CreateRoom(domain.Room{
		Title: "Dinner", Currency: "EUR", Status: domain.RoomStatusClaiming,
	})
	payer, _ := memoryStore.AddParticipant(room.ID, domain.Participant{Name: "Max"})
	guest, _ := memoryStore.AddParticipant(room.ID, domain.Participant{Name: "Anna"})
	item, _ := memoryStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Pizza", Quantity: 1, UnitPrice: 1000, Total: 1000,
	})
	_, _ = memoryStore.AddAssignment(room.ID, domain.ItemAssignment{
		ItemID: item.ID, ParticipantID: payer.ID, Weight: 1,
	})
	_, _ = memoryStore.AddAssignment(room.ID, domain.ItemAssignment{
		ItemID: item.ID, ParticipantID: guest.ID, Weight: 1,
	})
	room.PayerParticipantID = payer.ID
	_, _ = memoryStore.UpdateRoom(room)

	result := doJSON[calculationPayload](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/calculate",
		nil,
		nil,
		http.StatusOK,
	)

	if len(result.Debts) != 1 {
		t.Fatalf("expected 1 debt, got %#v", result.Debts)
	}
	if result.Debts[0].FromParticipantID != guest.ID || result.Debts[0].Amount != 500 {
		t.Fatalf("unexpected debt: %#v", result.Debts[0])
	}
}

func TestCalculateRespectsEqualDiscountMode(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room, _ := memoryStore.CreateRoom(domain.Room{
		Title: "Dinner", Currency: "EUR", Status: domain.RoomStatusClaiming,
		Discount: 200, DiscountMode: domain.DiscountModeEqual,
	})
	p1, _ := memoryStore.AddParticipant(room.ID, domain.Participant{Name: "A"})
	p2, _ := memoryStore.AddParticipant(room.ID, domain.Participant{Name: "B"})
	expensive, _ := memoryStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Expensive", Quantity: 1, UnitPrice: 900, Total: 900,
	})
	cheap, _ := memoryStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Cheap", Quantity: 1, UnitPrice: 100, Total: 100,
	})
	_, _ = memoryStore.AddAssignment(room.ID, domain.ItemAssignment{
		ItemID: expensive.ID, ParticipantID: p1.ID, Weight: 1,
	})
	_, _ = memoryStore.AddAssignment(room.ID, domain.ItemAssignment{
		ItemID: cheap.ID, ParticipantID: p2.ID, Weight: 1,
	})

	result := doJSON[calculationPayload](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/calculate",
		nil,
		nil,
		http.StatusOK,
	)

	for _, participantResult := range result.Results {
		if participantResult.DiscountShare != 100 {
			t.Fatalf("expected equal discount of 100, got %#v", result.Results)
		}
	}
}

func TestCalculateRejectsUnassignedItems(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room, _ := memoryStore.CreateRoom(domain.Room{
		Title: "Dinner", Currency: "EUR", Status: domain.RoomStatusClaiming,
	})
	_, _ = memoryStore.AddParticipant(room.ID, domain.Participant{Name: "Max"})
	_, _ = memoryStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Pizza", Quantity: 1, UnitPrice: 1000, Total: 1000,
	})

	doJSON[map[string]string](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/calculate",
		nil,
		nil,
		http.StatusBadRequest,
	)
}

func TestAdminCanAssignWeightManually(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room, _ := memoryStore.CreateRoom(domain.Room{
		Title: "Dinner", Currency: "EUR", Status: domain.RoomStatusClaiming,
	})
	participant, _ := memoryStore.AddParticipant(room.ID, domain.Participant{Name: "Max"})
	item, _ := memoryStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Pizza", Quantity: 1, UnitPrice: 1000, Total: 1000,
	})

	assignment := doJSON[domain.ItemAssignment](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/assignments",
		map[string]any{
			"item_id":        item.ID,
			"participant_id": participant.ID,
			"weight":         4,
		},
		map[string]string{"X-Admin-Token": room.AdminToken},
		http.StatusCreated,
	)

	if assignment.Weight != 4 {
		t.Fatalf("expected weight 4, got %d", assignment.Weight)
	}
}

func TestAdminAssignmentRejectsInvalidWeight(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room, _ := memoryStore.CreateRoom(domain.Room{
		Title: "Dinner", Currency: "EUR", Status: domain.RoomStatusClaiming,
	})
	participant, _ := memoryStore.AddParticipant(room.ID, domain.Participant{Name: "Max"})
	item, _ := memoryStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Pizza", Quantity: 1, UnitPrice: 1000, Total: 1000,
	})

	doJSON[map[string]string](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/assignments",
		map[string]any{
			"item_id":        item.ID,
			"participant_id": participant.ID,
			"weight":         1001,
		},
		map[string]string{"X-Admin-Token": room.AdminToken},
		http.StatusBadRequest,
	)
}

func TestParticipantCanUpdateOwnWeight(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room, _ := memoryStore.CreateRoom(domain.Room{
		Title: "Dinner", Currency: "EUR", Status: domain.RoomStatusClaiming,
	})
	joined := doJSON[joinResponse](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/join",
		map[string]any{"name": "Аня"},
		nil,
		http.StatusCreated,
	)
	item, _ := memoryStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Pizza", Quantity: 1, UnitPrice: 1000, Total: 1000,
	})

	first := doJSON[domain.ItemAssignment](
		t,
		handler,
		http.MethodPut,
		"/rooms/"+room.ID+"/selections/"+item.ID,
		map[string]any{"weight": 1},
		map[string]string{"X-Participant-Token": joined.ParticipantToken},
		http.StatusOK,
	)
	if first.Weight != 1 {
		t.Fatalf("expected weight 1, got %d", first.Weight)
	}

	second := doJSON[domain.ItemAssignment](
		t,
		handler,
		http.MethodPut,
		"/rooms/"+room.ID+"/selections/"+item.ID,
		map[string]any{"weight": 3},
		map[string]string{"X-Participant-Token": joined.ParticipantToken},
		http.StatusOK,
	)
	if second.Weight != 3 {
		t.Fatalf("participant must be able to update own weight, got %d", second.Weight)
	}

	assignments, err := memoryStore.ListAssignments(room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(assignments) != 1 {
		t.Fatalf("weight update must not duplicate assignment, got %#v", assignments)
	}
}

func TestWeightTwoVersusWeightOneThroughAPI(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room, _ := memoryStore.CreateRoom(domain.Room{
		Title: "Dinner", Currency: "EUR", Status: domain.RoomStatusClaiming,
	})
	heavy, _ := memoryStore.AddParticipant(room.ID, domain.Participant{Name: "Heavy"})
	light, _ := memoryStore.AddParticipant(room.ID, domain.Participant{Name: "Light"})
	item, _ := memoryStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Pizza", Quantity: 1, UnitPrice: 900, Total: 900,
	})

	doJSON[domain.ItemAssignment](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/assignments",
		map[string]any{
			"item_id":        item.ID,
			"participant_id": heavy.ID,
			"weight":         2,
		},
		map[string]string{"X-Admin-Token": room.AdminToken},
		http.StatusCreated,
	)
	doJSON[domain.ItemAssignment](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/assignments",
		map[string]any{
			"item_id":        item.ID,
			"participant_id": light.ID,
			"weight":         1,
		},
		map[string]string{"X-Admin-Token": room.AdminToken},
		http.StatusCreated,
	)

	result := doJSON[calculationPayload](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/calculate",
		nil,
		nil,
		http.StatusOK,
	)

	amounts := map[string]int64{}
	for _, participantResult := range result.Results {
		amounts[participantResult.ParticipantID] = participantResult.BaseAmount
	}
	if amounts[heavy.ID] != 600 || amounts[light.ID] != 300 {
		t.Fatalf("weight 2 must get double of weight 1: %#v", amounts)
	}
}

func doJSON[T any](
	t *testing.T,
	handler http.Handler,
	method string,
	path string,
	body any,
	headers map[string]string,
	expectedStatus int,
) T {
	t.Helper()

	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		requestBody = bytes.NewReader(data)
	}

	req := httptest.NewRequest(method, path, requestBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}

	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != expectedStatus {
		t.Fatalf("%s %s: expected status %d, got %d: %s", method, path, expectedStatus, res.Code, res.Body.String())
	}

	var result T
	if res.Body.Len() == 0 {
		return result
	}
	if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, res.Body.String())
	}
	return result
}
