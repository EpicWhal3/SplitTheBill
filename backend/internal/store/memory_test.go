package store

import (
	"errors"
	"testing"

	"splitthebill/backend/internal/domain"
)

func TestMemoryStoreCollaborationFlow(t *testing.T) {
	appStore := NewMemoryStore()

	room, err := appStore.CreateRoom(domain.Room{
		Title:    "Dinner",
		Currency: "EUR",
	})
	if err != nil {
		t.Fatal(err)
	}

	if room.AdminToken == "" {
		t.Fatal("expected admin token")
	}

	participant, err := appStore.AddParticipant(
		room.ID,
		domain.Participant{Name: "Аня"},
	)
	if err != nil {
		t.Fatal(err)
	}

	if participant.Claimed {
		t.Fatal("organizer-created participant must be unclaimed")
	}

	joined, err := appStore.JoinParticipant(room.ID, "аня")
	if err != nil {
		t.Fatal(err)
	}

	if !joined.Claimed || joined.AccessToken == "" {
		t.Fatal("joined participant must receive a token")
	}

	if joined.ID != participant.ID {
		t.Fatalf("expected existing participant %s to be claimed, got %s", participant.ID, joined.ID)
	}

	_, err = appStore.JoinParticipant(room.ID, "Аня")
	if !errors.Is(err, ErrorNameTaken) {
		t.Fatalf("expected ErrorNameTaken, got %v", err)
	}

	found, err := appStore.FindParticipantByToken(room.ID, joined.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != joined.ID {
		t.Fatalf("expected participant %s, got %s", joined.ID, found.ID)
	}

	item, err := appStore.AddItem(room.ID, domain.ReceiptItem{
		Name:      "Pizza",
		Quantity:  1,
		UnitPrice: 1000,
		Total:     1000,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = appStore.AddAssignment(room.ID, domain.ItemAssignment{
		ItemID:        item.ID,
		ParticipantID: joined.ID,
		Weight:        1,
	})
	if err != nil {
		t.Fatal(err)
	}

	room.PayerParticipantID = joined.ID
	if _, err := appStore.UpdateRoom(room); err != nil {
		t.Fatal(err)
	}

	if err := appStore.DeleteParticipant(room.ID, joined.ID); err != nil {
		t.Fatal(err)
	}

	updatedRoom, err := appStore.GetRoom(room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedRoom.PayerParticipantID != "" {
		t.Fatal("payer must be cleared when participant is deleted")
	}

	assignments, err := appStore.ListAssignments(room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(assignments) != 0 {
		t.Fatalf("participant deletion must cascade assignments, got %#v", assignments)
	}
}

func TestMemoryStoreCreateRoomDefaults(t *testing.T) {
	appStore := NewMemoryStore()

	room, err := appStore.CreateRoom(domain.Room{
		Title:    "Dinner",
		Currency: "EUR",
	})
	if err != nil {
		t.Fatal(err)
	}

	if room.Status != domain.RoomStatusDraft {
		t.Fatalf("expected draft status, got %q", room.Status)
	}
	if room.DiscountMode != domain.DiscountModeProportional {
		t.Fatalf("expected proportional discount mode, got %q", room.DiscountMode)
	}
	if room.AdminToken == "" {
		t.Fatal("expected generated admin token")
	}
	if room.ID == "" {
		t.Fatal("expected generated room id")
	}
}

func TestMemoryStoreCreateRoomKeepsExplicitValues(t *testing.T) {
	appStore := NewMemoryStore()

	room, err := appStore.CreateRoom(domain.Room{
		Title:        "Dinner",
		Currency:     "EUR",
		DiscountMode: domain.DiscountModeEqual,
		Status:       domain.RoomStatusClaiming,
	})
	if err != nil {
		t.Fatal(err)
	}

	if room.DiscountMode != domain.DiscountModeEqual {
		t.Fatalf("expected equal discount mode, got %q", room.DiscountMode)
	}
	if room.Status != domain.RoomStatusClaiming {
		t.Fatalf("expected claiming status, got %q", room.Status)
	}
}

func TestMemoryStoreUpdateRoomPreservesAdminToken(t *testing.T) {
	appStore := NewMemoryStore()

	room, err := appStore.CreateRoom(domain.Room{Title: "Dinner", Currency: "EUR"})
	if err != nil {
		t.Fatal(err)
	}

	room.Title = "Updated"
	room.AdminToken = ""

	updated, err := appStore.UpdateRoom(room)
	if err != nil {
		t.Fatal(err)
	}
	if updated.AdminToken == "" {
		t.Fatal("admin token must be preserved when omitted")
	}

	stored, err := appStore.GetRoom(room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Title != "Updated" {
		t.Fatalf("expected updated title, got %q", stored.Title)
	}
}

func TestMemoryStoreUpdateRoomUnknownRoom(t *testing.T) {
	appStore := NewMemoryStore()

	_, err := appStore.UpdateRoom(domain.Room{ID: "missing", Title: "X"})
	if !errors.Is(err, ErrorNotFound) {
		t.Fatalf("expected ErrorNotFound, got %v", err)
	}
}

func TestMemoryStoreAddAssignmentUpsertsWeight(t *testing.T) {
	appStore := NewMemoryStore()

	room, err := appStore.CreateRoom(domain.Room{Title: "Dinner", Currency: "EUR"})
	if err != nil {
		t.Fatal(err)
	}
	participant, err := appStore.AddParticipant(room.ID, domain.Participant{Name: "Аня"})
	if err != nil {
		t.Fatal(err)
	}
	item, err := appStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Pizza", Quantity: 1, UnitPrice: 1000, Total: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}

	created, err := appStore.AddAssignment(room.ID, domain.ItemAssignment{
		ItemID: item.ID, ParticipantID: participant.ID, Weight: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Weight != 1 {
		t.Fatalf("expected weight 1, got %d", created.Weight)
	}

	updated, err := appStore.AddAssignment(room.ID, domain.ItemAssignment{
		ItemID: item.ID, ParticipantID: participant.ID, Weight: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Weight != 5 {
		t.Fatalf("expected weight 5, got %d", updated.Weight)
	}

	assignments, err := appStore.ListAssignments(room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(assignments) != 1 {
		t.Fatalf("repeated assignment must upsert, got %#v", assignments)
	}
	if assignments[0].Weight != 5 {
		t.Fatalf("expected stored weight 5, got %d", assignments[0].Weight)
	}
}

func TestMemoryStoreAddAssignmentRejectsUnknownReferences(t *testing.T) {
	appStore := NewMemoryStore()

	room, err := appStore.CreateRoom(domain.Room{Title: "Dinner", Currency: "EUR"})
	if err != nil {
		t.Fatal(err)
	}
	participant, err := appStore.AddParticipant(room.ID, domain.Participant{Name: "Аня"})
	if err != nil {
		t.Fatal(err)
	}
	item, err := appStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Pizza", Quantity: 1, UnitPrice: 1000, Total: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = appStore.AddAssignment(room.ID, domain.ItemAssignment{
		ItemID: "missing", ParticipantID: participant.ID, Weight: 1,
	})
	if !errors.Is(err, ErrorItemNotFound) {
		t.Fatalf("expected ErrorItemNotFound, got %v", err)
	}

	_, err = appStore.AddAssignment(room.ID, domain.ItemAssignment{
		ItemID: item.ID, ParticipantID: "missing", Weight: 1,
	})
	if !errors.Is(err, ErrorParticipantNotFound) {
		t.Fatalf("expected ErrorParticipantNotFound, got %v", err)
	}
}

func TestMemoryStoreDeleteAssignment(t *testing.T) {
	appStore := NewMemoryStore()

	room, err := appStore.CreateRoom(domain.Room{Title: "Dinner", Currency: "EUR"})
	if err != nil {
		t.Fatal(err)
	}
	participant, err := appStore.AddParticipant(room.ID, domain.Participant{Name: "Аня"})
	if err != nil {
		t.Fatal(err)
	}
	item, err := appStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Pizza", Quantity: 1, UnitPrice: 1000, Total: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := appStore.AddAssignment(room.ID, domain.ItemAssignment{
		ItemID: item.ID, ParticipantID: participant.ID, Weight: 1,
	}); err != nil {
		t.Fatal(err)
	}

	if err := appStore.DeleteAssignment(room.ID, item.ID, participant.ID); err != nil {
		t.Fatal(err)
	}

	assignments, err := appStore.ListAssignments(room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(assignments) != 0 {
		t.Fatalf("expected no assignments after delete, got %#v", assignments)
	}
}

func TestMemoryStoreDeleteAssignmentMissingReturnsError(t *testing.T) {
	appStore := NewMemoryStore()

	room, err := appStore.CreateRoom(domain.Room{Title: "Dinner", Currency: "EUR"})
	if err != nil {
		t.Fatal(err)
	}

	err = appStore.DeleteAssignment(room.ID, "missing-item", "missing-participant")
	if !errors.Is(err, ErrorNotFound) {
		t.Fatalf("expected ErrorNotFound, got %v", err)
	}
}

func TestMemoryStoreDeleteItemCascadesAssignments(t *testing.T) {
	appStore := NewMemoryStore()

	room, err := appStore.CreateRoom(domain.Room{Title: "Dinner", Currency: "EUR"})
	if err != nil {
		t.Fatal(err)
	}
	participant, err := appStore.AddParticipant(room.ID, domain.Participant{Name: "Аня"})
	if err != nil {
		t.Fatal(err)
	}
	item, err := appStore.AddItem(room.ID, domain.ReceiptItem{
		Name: "Pizza", Quantity: 1, UnitPrice: 1000, Total: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := appStore.AddAssignment(room.ID, domain.ItemAssignment{
		ItemID: item.ID, ParticipantID: participant.ID, Weight: 1,
	}); err != nil {
		t.Fatal(err)
	}

	if err := appStore.DeleteItem(room.ID, item.ID); err != nil {
		t.Fatal(err)
	}

	assignments, err := appStore.ListAssignments(room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(assignments) != 0 {
		t.Fatalf("item deletion must cascade assignments, got %#v", assignments)
	}
}

func TestMemoryStoreUnknownRoomReturnsNotFound(t *testing.T) {
	appStore := NewMemoryStore()

	if _, err := appStore.GetRoom("missing"); !errors.Is(err, ErrorNotFound) {
		t.Fatalf("GetRoom: expected ErrorNotFound, got %v", err)
	}
	if _, err := appStore.ListParticipants("missing"); !errors.Is(err, ErrorNotFound) {
		t.Fatalf("ListParticipants: expected ErrorNotFound, got %v", err)
	}
	if _, err := appStore.ListItems("missing"); !errors.Is(err, ErrorNotFound) {
		t.Fatalf("ListItems: expected ErrorNotFound, got %v", err)
	}
	if _, err := appStore.ListAssignments("missing"); !errors.Is(err, ErrorNotFound) {
		t.Fatalf("ListAssignments: expected ErrorNotFound, got %v", err)
	}
}

func TestMemoryStoreJoinParticipantCreatesNewWhenUnknown(t *testing.T) {
	appStore := NewMemoryStore()

	room, err := appStore.CreateRoom(domain.Room{Title: "Dinner", Currency: "EUR"})
	if err != nil {
		t.Fatal(err)
	}

	joined, err := appStore.JoinParticipant(room.ID, "Новый")
	if err != nil {
		t.Fatal(err)
	}
	if !joined.Claimed || joined.AccessToken == "" {
		t.Fatalf("joined participant must be claimed with token: %#v", joined)
	}

	participants, err := appStore.ListParticipants(room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(participants) != 1 {
		t.Fatalf("expected single participant, got %#v", participants)
	}
}

func TestMemoryStoreFindParticipantByTokenRejectsEmptyToken(t *testing.T) {
	appStore := NewMemoryStore()

	room, err := appStore.CreateRoom(domain.Room{Title: "Dinner", Currency: "EUR"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := appStore.AddParticipant(room.ID, domain.Participant{Name: "Аня"}); err != nil {
		t.Fatal(err)
	}

	_, err = appStore.FindParticipantByToken(room.ID, "")
	if !errors.Is(err, ErrorParticipantNotFound) {
		t.Fatalf("expected ErrorParticipantNotFound, got %v", err)
	}
}
