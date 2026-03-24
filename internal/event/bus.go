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
	EventAgentOnline       EventType = "agent_online"
	EventAgentOffline      EventType = "agent_offline"
	EventProjectRegistered EventType = "project_registered"
)

// Event is a message published on the Bus.
type Event struct {
	Type      EventType   `json:"type"`
	Project   string      `json:"project,omitempty"`
	AgentID   string      `json:"agent_id,omitempty"`
	TaskID    string      `json:"task_id,omitempty"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
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
			default:
			}
		}
	}
}

// Subscribe registers a channel to receive events and returns an unsubscribe function.
// Pass event types to filter; pass nothing to receive all events.
func (b *Bus) Subscribe(ch chan Event, filters ...EventType) (unsubscribe func()) {
	b.mu.Lock()
	b.subs[ch] = filters
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		delete(b.subs, ch)
		b.mu.Unlock()
		close(ch)
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
