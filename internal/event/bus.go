package event

import (
	"sync"
	"time"
)

// EventType identifies the kind of event.
type EventType string

const (
	EventTaskStatusChanged EventType = "task_status_changed"
	EventTaskCompleted     EventType = "task_completed"
	EventTaskBlocked       EventType = "task_blocked"
)

// Event is a message published on the Bus.
type Event struct {
	Type    EventType   `json:"type"`
	Project string      `json:"project,omitempty"`
	Payload interface{} `json:"payload"`
	At      time.Time   `json:"at"`
}

// Bus is a simple in-process pub/sub event bus backed by channels.
type Bus struct {
	mu   sync.RWMutex
	subs map[chan Event][]EventType
}

// NewBus creates a new Bus.
func NewBus() *Bus {
	return &Bus{subs: make(map[chan Event][]EventType)}
}

// Publish sends e to all subscribers whose filter matches (empty filter = all events).
func (b *Bus) Publish(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch, filters := range b.subs {
		if matchesFilter(e.Type, filters) {
			select {
			case ch <- e:
			default: // drop if subscriber is slow
			}
		}
	}
}

// Subscribe returns a read-only channel and an unsubscribe function.
// Pass event types to filter; pass nothing to receive all events.
func (b *Bus) Subscribe(filters ...EventType) (ch <-chan Event, unsubscribe func()) {
	c := make(chan Event, 64)
	b.mu.Lock()
	b.subs[c] = filters
	b.mu.Unlock()
	return c, func() {
		b.mu.Lock()
		delete(b.subs, c)
		b.mu.Unlock()
		close(c)
	}
}

func matchesFilter(t EventType, filters []EventType) bool {
	if len(filters) == 0 {
		return true
	}
	for _, f := range filters {
		if f == t {
			return true
		}
	}
	return false
}
