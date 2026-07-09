package store

import "splitthebill/backend/internal/domain"

type Store interface {
	CreateRoom(room domain.Room) (domain.Room, error)
	GetRoom(roomID string) (domain.Room, error)
	UpdateRoom(room domain.Room) (domain.Room, error)

	AddParticipant(roomID string, participant domain.Participant) (domain.Participant, error)
	ListParticipants(roomID string) ([]domain.Participant, error)

	AddItem(roomID string, item domain.ReceiptItem) (domain.ReceiptItem, error)
	ListItems(roomID string) ([]domain.ReceiptItem, error)

	AddAssignment(roomID string, assignment domain.ItemAssignment) (domain.ItemAssignment, error)
	ListAssignments(roomID string) ([]domain.ItemAssignment, error)
}
