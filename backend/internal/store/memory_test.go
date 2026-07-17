package store

import (
	"testing"

	"splitthebill/backend/internal/domain"
)

func TestMemoryStoreCRUDAndAssignmentCascade(
	t *testing.T,
) {
	store := NewMemoryStore()

	room, err := store.CreateRoom(
		domain.Room{
			Title:    "Dinner",
			Currency: "EUR",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	participant, err := store.AddParticipant(
		room.ID,
		domain.Participant{
			Name: "Аня",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	item, err := store.AddItem(
		room.ID,
		domain.ReceiptItem{
			Name:      "Pizza",
			Quantity:  1,
			UnitPrice: 1000,
			Total:     1000,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.AddAssignment(
		room.ID,
		domain.ItemAssignment{
			ItemID:        item.ID,
			ParticipantID: participant.ID,
			Weight:        1,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	participant.Name = "Анна"

	updatedParticipant, err :=
		store.UpdateParticipant(
			room.ID,
			participant,
		)
	if err != nil {
		t.Fatal(err)
	}

	if updatedParticipant.Name != "Анна" {
		t.Fatalf(
			"expected updated name, got %q",
			updatedParticipant.Name,
		)
	}

	item.Name = "Large Pizza"
	item.Quantity = 2
	item.UnitPrice = 750
	item.Total = 1500

	updatedItem, err := store.UpdateItem(
		room.ID,
		item,
	)
	if err != nil {
		t.Fatal(err)
	}

	if updatedItem.Total != 1500 {
		t.Fatalf(
			"expected updated total, got %d",
			updatedItem.Total,
		)
	}

	if err := store.DeleteParticipant(
		room.ID,
		participant.ID,
	); err != nil {
		t.Fatal(err)
	}

	assignments, err :=
		store.ListAssignments(room.ID)
	if err != nil {
		t.Fatal(err)
	}

	if len(assignments) != 0 {
		t.Fatalf(
			"expected assignments to be deleted with participant, got %#v",
			assignments,
		)
	}

	if err := store.DeleteItem(
		room.ID,
		item.ID,
	); err != nil {
		t.Fatal(err)
	}

	items, err := store.ListItems(room.ID)
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 0 {
		t.Fatalf(
			"expected item to be deleted, got %#v",
			items,
		)
	}
}
