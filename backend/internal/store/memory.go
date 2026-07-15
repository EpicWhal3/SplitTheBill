package store

import (
	"splitthebill/backend/internal/domain"
	"sync"
)

type MemoryStore struct {
	mu sync.RWMutex

	rooms        map[string]domain.Room
	participants map[string][]domain.Participant
	items        map[string][]domain.ReceiptItem
	assignments  map[string][]domain.ItemAssignment
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		rooms:        make(map[string]domain.Room),
		participants: make(map[string][]domain.Participant),
		items:        make(map[string][]domain.ReceiptItem),
		assignments:  make(map[string][]domain.ItemAssignment),
	}
}

func (s *MemoryStore) CreateRoom(room domain.Room) (domain.Room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	room.ID = newID()
	s.rooms[room.ID] = room
	s.participants[room.ID] = []domain.Participant{}
	s.items[room.ID] = []domain.ReceiptItem{}
	s.assignments[room.ID] = []domain.ItemAssignment{}
	return room, nil
}

func (s *MemoryStore) GetRoom(roomID string) (domain.Room, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	room, ok := s.rooms[roomID]
	if !ok {
		return domain.Room{}, ErrorNotFound
	}

	return room, nil
}

func (s *MemoryStore) UpdateRoom(room domain.Room) (domain.Room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.rooms[room.ID]; !ok {
		return domain.Room{}, ErrorNotFound
	}
	s.rooms[room.ID] = room
	return room, nil
}

func (s *MemoryStore) AddParticipant(
	roomID string,
	participant domain.Participant,
) (domain.Participant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.rooms[roomID]; !ok {
		return domain.Participant{}, ErrorNotFound
	}

	participant.ID = newID()
	participant.RoomID = roomID

	s.participants[roomID] = append(s.participants[roomID], participant)
	return participant, nil
}

func (s *MemoryStore) ListParticipants(
	roomID string,
) ([]domain.Participant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.rooms[roomID]; !ok {
		return nil, ErrorNotFound
	}
	return append(
		[]domain.Participant(nil),
		s.participants[roomID]...,
	), nil
}

func (s *MemoryStore) UpdateParticipant(
	roomID string,
	participant domain.Participant,
) (domain.Participant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.rooms[roomID]; !ok {
		return domain.Participant{}, ErrorNotFound
	}

	participants := s.participants[roomID]

	for i, existing := range participants {
		if existing.ID == participant.ID {
			participant.RoomID = roomID
			participants[i] = participant
			s.participants[roomID] = participants
			return participant, nil
		}
	}

	return domain.Participant{}, ErrorParticipantNotFound
}

func (s *MemoryStore) DeleteParticipant(
	roomID string,
	participantID string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.rooms[roomID]; !ok {
		return ErrorNotFound
	}

	participants := s.participants[roomID]
	found := false

	filteredParticipants := make(
		[]domain.Participant,
		0,
		len(participants),
	)

	for _, participant := range participants {
		if participant.ID == participantID {
			found = true
			continue
		}

		filteredParticipants = append(
			filteredParticipants,
			participant,
		)
	}

	if !found {
		return ErrorParticipantNotFound
	}

	s.participants[roomID] = filteredParticipants

	assignments := s.assignments[roomID]

	filteredAssignments := make(
		[]domain.ItemAssignment,
		0,
		len(assignments),
	)

	for _, assignment := range assignments {
		if assignment.ParticipantID != participantID {
			filteredAssignments = append(
				filteredAssignments,
				assignment,
			)
		}
	}

	s.assignments[roomID] = filteredAssignments

	return nil
}

func (s *MemoryStore) AddItem(
	roomID string,
	item domain.ReceiptItem,
) (domain.ReceiptItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.rooms[roomID]; !ok {
		return domain.ReceiptItem{}, ErrorNotFound
	}

	item.ID = newID()
	item.RoomID = roomID

	s.items[roomID] = append(s.items[roomID], item)

	return item, nil
}

func (s *MemoryStore) ListItems(
	roomID string,
) ([]domain.ReceiptItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.rooms[roomID]; !ok {
		return nil, ErrorNotFound
	}

	return append(
		[]domain.ReceiptItem(nil),
		s.items[roomID]...,
	), nil
}

func (s *MemoryStore) UpdateItem(
	roomID string,
	item domain.ReceiptItem,
) (domain.ReceiptItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.rooms[roomID]; !ok {
		return domain.ReceiptItem{}, ErrorNotFound
	}

	items := s.items[roomID]

	for i, existing := range items {
		if existing.ID == item.ID {
			item.RoomID = roomID
			items[i] = item
			s.items[roomID] = items
			return item, nil
		}
	}

	return domain.ReceiptItem{}, ErrorItemNotFound
}

func (s *MemoryStore) DeleteItem(
	roomID string,
	itemID string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.rooms[roomID]; !ok {
		return ErrorNotFound
	}

	items := s.items[roomID]
	found := false

	filteredItems := make(
		[]domain.ReceiptItem,
		0,
		len(items),
	)

	for _, item := range items {
		if item.ID == itemID {
			found = true
			continue
		}

		filteredItems = append(filteredItems, item)
	}

	if !found {
		return ErrorItemNotFound
	}

	s.items[roomID] = filteredItems

	assignments := s.assignments[roomID]

	filteredAssignments := make(
		[]domain.ItemAssignment,
		0,
		len(assignments),
	)

	for _, assignment := range assignments {
		if assignment.ItemID != itemID {
			filteredAssignments = append(
				filteredAssignments,
				assignment,
			)
		}
	}

	s.assignments[roomID] = filteredAssignments

	return nil
}

func (s *MemoryStore) AddAssignment(
	roomID string,
	assignment domain.ItemAssignment,
) (domain.ItemAssignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.rooms[roomID]; !ok {
		return domain.ItemAssignment{}, ErrorNotFound
	}

	if !s.itemExists(roomID, assignment.ItemID) {
		return domain.ItemAssignment{}, ErrorItemNotFound
	}

	if !s.participantExists(
		roomID,
		assignment.ParticipantID,
	) {
		return domain.ItemAssignment{}, ErrorParticipantNotFound
	}

	assignments := s.assignments[roomID]

	for i, existing := range assignments {
		if existing.ItemID == assignment.ItemID &&
			existing.ParticipantID == assignment.ParticipantID {
			assignments[i] = assignment
			s.assignments[roomID] = assignments
			return assignment, nil
		}
	}

	s.assignments[roomID] = append(
		s.assignments[roomID],
		assignment,
	)

	return assignment, nil
}

func (s *MemoryStore) ListAssignments(
	roomID string,
) ([]domain.ItemAssignment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.rooms[roomID]; !ok {
		return nil, ErrorNotFound
	}

	return append(
		[]domain.ItemAssignment(nil),
		s.assignments[roomID]...,
	), nil
}

func (s *MemoryStore) DeleteAssignment(
	roomID string,
	itemID string,
	participantID string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.rooms[roomID]; !ok {
		return ErrorNotFound
	}

	assignments := s.assignments[roomID]
	found := false

	filtered := make(
		[]domain.ItemAssignment,
		0,
		len(assignments),
	)

	for _, assignment := range assignments {
		if assignment.ItemID == itemID &&
			assignment.ParticipantID == participantID {
			found = true
			continue
		}

		filtered = append(filtered, assignment)
	}

	if !found {
		return ErrorNotFound
	}

	s.assignments[roomID] = filtered

	return nil
}

func (s *MemoryStore) itemExists(
	roomID string,
	itemID string,
) bool {
	for _, item := range s.items[roomID] {
		if item.ID == itemID {
			return true
		}
	}

	return false
}

func (s *MemoryStore) participantExists(
	roomID string,
	participantID string,
) bool {
	for _, participant := range s.participants[roomID] {
		if participant.ID == participantID {
			return true
		}
	}

	return false
}
