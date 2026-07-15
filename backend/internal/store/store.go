package store

import (
	"errors"

	"splitthebill/backend/internal/domain"
)

var (
	ErrorNotFound            = errors.New("not found")
	ErrorItemNotFound        = errors.New("item not found")
	ErrorParticipantNotFound = errors.New("participant not found")
)

type Store interface {
	CreateRoom(room domain.Room) (domain.Room, error)
	GetRoom(roomID string) (domain.Room, error)
	UpdateRoom(room domain.Room) (domain.Room, error)

	AddParticipant(roomID string, participant domain.Participant) (domain.Participant, error)
	ListParticipants(roomID string) ([]domain.Participant, error)
	UpdateParticipant(roomID string, participant domain.Participant) (domain.Participant, error)
	DeleteParticipant(roomID string, participantID string) error

	AddItem(roomID string, item domain.ReceiptItem) (domain.ReceiptItem, error)
	ListItems(roomID string) ([]domain.ReceiptItem, error)
	UpdateItem(roomID string, item domain.ReceiptItem) (domain.ReceiptItem, error)
	DeleteItem(roomID string, itemID string) error

	AddAssignment(roomID string, assignment domain.ItemAssignment) (domain.ItemAssignment, error)
	ListAssignments(roomID string) ([]domain.ItemAssignment, error)
	DeleteAssignment(roomID string, itemID string, participantID string) error
}
