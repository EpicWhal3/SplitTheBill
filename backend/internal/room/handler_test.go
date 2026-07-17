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

func TestRoomCRUDAndCalculation(
	t *testing.T,
) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room := doJSON[domain.Room](
		t,
		handler,
		http.MethodPost,
		"/rooms",
		map[string]any{
			"title":          "Dinner",
			"currency":       "EUR",
			"expected_total": 1000,
		},
		http.StatusCreated,
	)

	participant := doJSON[domain.Participant](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/participants",
		map[string]any{
			"name": "Аня",
		},
		http.StatusCreated,
	)

	participant = doJSON[domain.Participant](
		t,
		handler,
		http.MethodPatch,
		"/rooms/"+room.ID+
			"/participants/"+
			participant.ID,
		map[string]any{
			"name": "Анна",
		},
		http.StatusOK,
	)

	if participant.Name != "Анна" {
		t.Fatalf(
			"expected updated participant name, got %q",
			participant.Name,
		)
	}

	item := doJSON[domain.ReceiptItem](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/items",
		map[string]any{
			"name":       "Pizza",
			"quantity":   1,
			"unit_price": 1000,
		},
		http.StatusCreated,
	)

	item = doJSON[domain.ReceiptItem](
		t,
		handler,
		http.MethodPatch,
		"/rooms/"+room.ID+
			"/items/"+
			item.ID,
		map[string]any{
			"name": "Large Pizza",
		},
		http.StatusOK,
	)

	if item.Name != "Large Pizza" ||
		item.Total != 1000 {
		t.Fatalf(
			"unexpected updated item: %#v",
			item,
		)
	}

	doJSON[domain.ItemAssignment](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/assignments",
		map[string]any{
			"item_id":        item.ID,
			"participant_id": participant.ID,
			"weight":         1,
		},
		http.StatusCreated,
	)

	calculation := doJSON[struct {
		Subtotal int64 `json:"subtotal"`

		CalculatedTotal int64 `json:"calculated_total"`

		Difference int64 `json:"difference"`

		MatchesExpectedTotal bool `json:"matches_expected_total"`
	}](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/calculate",
		nil,
		http.StatusOK,
	)

	if calculation.Subtotal != 1000 ||
		calculation.CalculatedTotal != 1000 ||
		calculation.Difference != 0 ||
		!calculation.MatchesExpectedTotal {
		t.Fatalf(
			"unexpected calculation response: %#v",
			calculation,
		)
	}

	doNoContent(
		t,
		handler,
		http.MethodDelete,
		"/rooms/"+room.ID+
			"/participants/"+
			participant.ID,
	)

	assignments, err :=
		memoryStore.ListAssignments(room.ID)
	if err != nil {
		t.Fatal(err)
	}

	if len(assignments) != 0 {
		t.Fatalf(
			"participant deletion must cascade assignments, got %#v",
			assignments,
		)
	}
}

func TestCalculateReportsExpectedTotalDifference(
	t *testing.T,
) {
	memoryStore := store.NewMemoryStore()
	handler := NewHandler(memoryStore)

	room, _ := memoryStore.CreateRoom(
		domain.Room{
			Title:         "Dinner",
			Currency:      "EUR",
			ExpectedTotal: 1200,
		},
	)

	participant, _ :=
		memoryStore.AddParticipant(
			room.ID,
			domain.Participant{
				Name: "A",
			},
		)

	item, _ := memoryStore.AddItem(
		room.ID,
		domain.ReceiptItem{
			Name:      "Pizza",
			Quantity:  1,
			UnitPrice: 1000,
			Total:     1000,
		},
	)

	_, _ = memoryStore.AddAssignment(
		room.ID,
		domain.ItemAssignment{
			ItemID:        item.ID,
			ParticipantID: participant.ID,
			Weight:        1,
		},
	)

	calculation := doJSON[struct {
		Difference int64 `json:"difference"`

		MatchesExpectedTotal bool `json:"matches_expected_total"`
	}](
		t,
		handler,
		http.MethodPost,
		"/rooms/"+room.ID+"/calculate",
		nil,
		http.StatusOK,
	)

	if calculation.Difference != -200 ||
		calculation.MatchesExpectedTotal {
		t.Fatalf(
			"unexpected expected-total comparison: %#v",
			calculation,
		)
	}
}

func doNoContent(
	t *testing.T,
	handler http.Handler,
	method string,
	path string,
) {
	t.Helper()

	req := httptest.NewRequest(
		method,
		path,
		nil,
	)

	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf(
			"%s %s: expected status %d, got %d: %s",
			method,
			path,
			http.StatusNoContent,
			res.Code,
			res.Body.String(),
		)
	}
}

func doJSON[T any](
	t *testing.T,
	handler http.Handler,
	method string,
	path string,
	body any,
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

	req := httptest.NewRequest(
		method,
		path,
		requestBody,
	)

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != expectedStatus {
		t.Fatalf(
			"%s %s: expected status %d, got %d: %s",
			method,
			path,
			expectedStatus,
			res.Code,
			res.Body.String(),
		)
	}

	var result T

	if err := json.Unmarshal(
		res.Body.Bytes(),
		&result,
	); err != nil {
		t.Fatalf(
			"decode response: %v; body=%s",
			err,
			res.Body.String(),
		)
	}

	return result
}
