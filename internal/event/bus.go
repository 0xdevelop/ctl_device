package event

import "sync"

// Handler is a function that handles an event payload.
type Handler func(payload interface{})

// Bus is a simple in-process pub/sub event bus.
type Bus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
}

// NewBus creates a new event Bus.
func NewBus() *Bus {
	return &Bus{handlers: make(map[string][]Handler)}
}

// Subscribe registers a handler for the given event type (stub).
func (b *Bus) Subscribe(eventType string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], h)
}

// Publish sends an event to all subscribers (stub).
func (b *Bus) Publish(eventType string, payload interface{}) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, h := range b.handlers[eventType] {
		go h(payload)
	}
}
