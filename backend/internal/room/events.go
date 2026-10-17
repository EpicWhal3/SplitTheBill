package room

import "sync"

type roomEvent struct {
	RoomID string `json:"room_id"`
}

type roomHub struct {
	mu          sync.Mutex
	subscribers map[string]map[chan struct{}]struct{}
}

func newRoomHub() *roomHub {
	return &roomHub{
		subscribers: make(map[string]map[chan struct{}]struct{}),
	}
}

func (h *roomHub) subscribe(roomID string) (chan struct{}, func()) {
	ch := make(chan struct{}, 1)

	h.mu.Lock()
	if h.subscribers[roomID] == nil {
		h.subscribers[roomID] = make(map[chan struct{}]struct{})
	}
	h.subscribers[roomID][ch] = struct{}{}
	h.mu.Unlock()

	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()

		subscribers := h.subscribers[roomID]
		delete(subscribers, ch)
		if len(subscribers) == 0 {
			delete(h.subscribers, roomID)
		}
	}
}

func (h *roomHub) publish(roomID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for subscriber := range h.subscribers[roomID] {
		select {
		case subscriber <- struct{}{}:
		default:
		}
	}
}

func (h *roomHub) removeRoom(roomID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for subscriber := range h.subscribers[roomID] {
		close(subscriber)
	}

	delete(h.subscribers, roomID)
}
