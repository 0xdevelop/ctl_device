package agent

import (
	"sync"

	"github.com/0xdevelop/ctl_device/pkg/protocol"
)

// Registry is an in-memory store of agents.
type Registry struct {
	mu     sync.RWMutex
	agents map[string]*protocol.Agent
}

// NewRegistry creates a new agent Registry.
func NewRegistry() *Registry {
	return &Registry{agents: make(map[string]*protocol.Agent)}
}

// Add adds or replaces an agent (stub).
func (r *Registry) Add(a *protocol.Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[a.ID] = a
	return nil
}

// Get retrieves an agent by ID (stub).
func (r *Registry) Get(id string) (*protocol.Agent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.agents[id]
	return a, ok
}

// List returns all agents (stub).
func (r *Registry) List() []*protocol.Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*protocol.Agent, 0, len(r.agents))
	for _, a := range r.agents {
		out = append(out, a)
	}
	return out
}
