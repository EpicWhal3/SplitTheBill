package store

import (
	"errors"
	"testing"
	"time"

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
	if participant.AccessToken == "" {
		t.Fatal("organizer-created participant must receive an invite token")
	}

	joined, err := appStore.JoinParticipantByToken(
		room.ID,
		participant.AccessToken,
	)
	if err != nil {
		t.Fatal(err)
	}

	if !joined.Claimed || joined.AccessToken != participant.AccessToken {
		t.Fatal("joined participant must be claimed with the invite token")
	}

	if joined.ID != participant.ID {
		t.Fatalf("expected existing participant %s to be claimed, got %s", participant.ID, joined.ID)
	}

	_, err = appStore.JoinParticipantByToken(room.ID, "unknown-token")
	if !errors.Is(err, ErrorParticipantNotFound) {
		t.Fatalf("expected ErrorParticipantNotFound, got %v", err)
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

func TestMemoryStoreJoinParticipantByTokenRejectsUnknownToken(t *testing.T) {
	appStore := NewMemoryStore()

	room, err := appStore.CreateRoom(domain.Room{Title: "Dinner", Currency: "EUR"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := appStore.JoinParticipantByToken(room.ID, "unknown-token"); !errors.Is(err, ErrorParticipantNotFound) {
		t.Fatalf("expected ErrorParticipantNotFound, got %v", err)
	}

	participants, err := appStore.ListParticipants(room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(participants) != 0 {
		t.Fatalf("join must not create participants, got %#v", participants)
	}
}

func TestMemoryStoreRotateParticipantToken(t *testing.T) {
	appStore := NewMemoryStore()

	room, err := appStore.CreateRoom(domain.Room{Title: "Dinner", Currency: "EUR"})
	if err != nil {
		t.Fatal(err)
	}
	participant, err := appStore.AddParticipant(
		room.ID,
		domain.Participant{Name: "Аня"},
	)
	if err != nil {
		t.Fatal(err)
	}

	rotated, err := appStore.RotateParticipantToken(room.ID, participant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rotated.AccessToken == "" || rotated.AccessToken == participant.AccessToken {
		t.Fatalf("expected a fresh invite token, got %#v", rotated)
	}

	if _, err := appStore.JoinParticipantByToken(room.ID, participant.AccessToken); !errors.Is(err, ErrorParticipantNotFound) {
		t.Fatalf("old invite token must be rejected, got %v", err)
	}

	joined, err := appStore.JoinParticipantByToken(room.ID, rotated.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if joined.ID != participant.ID || !joined.Claimed {
		t.Fatalf("expected rotated token to claim the participant, got %#v", joined)
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

func TestMemoryStoreDeleteRoomCascades(t *testing.T) {
	appStore := NewMemoryStore()

	room, err := appStore.CreateRoom(domain.Room{
		Title:    "Dinner",
		Currency: "EUR",
		Status:   domain.RoomStatusFinalized,
	})
	if err != nil {
		t.Fatal(err)
	}

	participant, err := appStore.AddParticipant(
		room.ID,
		domain.Participant{Name: "Аня"},
	)
	if err != nil {
		t.Fatal(err)
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
	if _, err := appStore.AddAssignment(room.ID, domain.ItemAssignment{
		ItemID:        item.ID,
		ParticipantID: participant.ID,
		Weight:        1,
	}); err != nil {
		t.Fatal(err)
	}

	if err := appStore.DeleteRoom(room.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := appStore.GetRoom(room.ID); !errors.Is(err, ErrorNotFound) {
		t.Fatalf("GetRoom: expected ErrorNotFound, got %v", err)
	}
	if _, err := appStore.ListParticipants(room.ID); !errors.Is(err, ErrorNotFound) {
		t.Fatalf("ListParticipants: expected ErrorNotFound, got %v", err)
	}
	if _, err := appStore.ListItems(room.ID); !errors.Is(err, ErrorNotFound) {
		t.Fatalf("ListItems: expected ErrorNotFound, got %v", err)
	}
	if _, err := appStore.ListAssignments(room.ID); !errors.Is(err, ErrorNotFound) {
		t.Fatalf("ListAssignments: expected ErrorNotFound, got %v", err)
	}

	if _, ok := appStore.rooms[room.ID]; ok {
		t.Fatal("room must be removed from the rooms map")
	}
	if _, ok := appStore.participants[room.ID]; ok {
		t.Fatal("participants must be removed with the room")
	}
	if _, ok := appStore.items[room.ID]; ok {
		t.Fatal("items must be removed with the room")
	}
	if _, ok := appStore.assignments[room.ID]; ok {
		t.Fatal("assignments must be removed with the room")
	}
}

func TestMemoryStoreDeleteRoomNotFound(t *testing.T) {
	appStore := NewMemoryStore()

	if err := appStore.DeleteRoom("missing"); !errors.Is(err, ErrorNotFound) {
		t.Fatalf("expected ErrorNotFound, got %v", err)
	}
}

func TestMemoryStoreRotateAdminToken(t *testing.T) {
	appStore := NewMemoryStore()

	room, err := appStore.CreateRoom(domain.Room{
		Title:    "Dinner",
		Currency: "EUR",
	})
	if err != nil {
		t.Fatal(err)
	}
	previousToken := room.AdminToken
	if previousToken == "" {
		t.Fatal("expected initial admin token")
	}

	rotated, err := appStore.RotateAdminToken(room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rotated.ID != room.ID {
		t.Fatalf("expected room %s, got %s", room.ID, rotated.ID)
	}
	if rotated.AdminToken == "" || rotated.AdminToken == previousToken {
		t.Fatalf("expected a new admin token, got %q", rotated.AdminToken)
	}

	stored, err := appStore.GetRoom(room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.AdminToken != rotated.AdminToken {
		t.Fatalf("expected stored token %q, got %q", rotated.AdminToken, stored.AdminToken)
	}

	rotatedAgain, err := appStore.RotateAdminToken(room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rotatedAgain.AdminToken == rotated.AdminToken {
		t.Fatal("token rotation must generate a different token every time")
	}
}

func TestMemoryStoreRotateAdminTokenNotFound(t *testing.T) {
	appStore := NewMemoryStore()

	if _, err := appStore.RotateAdminToken("missing"); !errors.Is(err, ErrorNotFound) {
		t.Fatalf("expected ErrorNotFound, got %v", err)
	}
}

func TestMemoryStoreListRoomsByOrganizer(t *testing.T) {
	appStore := NewMemoryStore()

	now := time.Now().UTC()

	first, err := appStore.CreateRoom(domain.Room{
		Title:            "First",
		Currency:         "EUR",
		OrganizerName:    "Anna",
		OrganizerPinHash: "hash-a",
		CreatedAt:        now.Add(-2 * time.Hour),
		UpdatedAt:        now.Add(-2 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	second, err := appStore.CreateRoom(domain.Room{
		Title:            "Second",
		Currency:         "EUR",
		OrganizerName:    "ANNA",
		OrganizerPinHash: "hash-a",
		CreatedAt:        now.Add(-1 * time.Hour),
		UpdatedAt:        now.Add(-1 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := appStore.CreateRoom(domain.Room{
		Title:            "Other",
		Currency:         "EUR",
		OrganizerName:    "Bob",
		OrganizerPinHash: "hash-a",
	}); err != nil {
		t.Fatal(err)
	}

	rooms, err := appStore.ListRoomsByOrganizer("anna", "hash-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(rooms) != 2 {
		t.Fatalf("expected 2 rooms, got %d", len(rooms))
	}
	if rooms[0].ID != second.ID || rooms[1].ID != first.ID {
		t.Fatalf("expected rooms sorted by updated_at desc, got %s then %s", rooms[0].ID, rooms[1].ID)
	}

	wrongHash, err := appStore.ListRoomsByOrganizer("anna", "wrong-hash")
	if err != nil {
		t.Fatal(err)
	}
	if len(wrongHash) != 0 {
		t.Fatalf("expected 0 rooms for wrong hash, got %d", len(wrongHash))
	}
}

func TestMemoryStoreDeleteExpiredRooms(t *testing.T) {
	appStore := NewMemoryStore()

	now := time.Now().UTC()

	oldRoom, err := appStore.CreateRoom(domain.Room{
		Title:     "Old",
		Currency:  "EUR",
		CreatedAt: now.Add(-10 * time.Hour),
		UpdatedAt: now.Add(-10 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}

	freshRoom, err := appStore.CreateRoom(domain.Room{
		Title:     "Fresh",
		Currency:  "EUR",
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}

	removed, err := appStore.DeleteExpiredRooms(now.Add(-1 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removed room, got %d", removed)
	}

	if _, err := appStore.GetRoom(oldRoom.ID); !errors.Is(err, ErrorNotFound) {
		t.Fatalf("expected old room to be deleted, got %v", err)
	}
	if _, err := appStore.GetRoom(freshRoom.ID); err != nil {
		t.Fatalf("expected fresh room to remain, got %v", err)
	}
}
