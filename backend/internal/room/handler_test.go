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
