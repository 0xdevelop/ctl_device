package agent

import "github.com/0xdevelop/ctl_device/pkg/protocol"

// Manager manages agent lifecycle and assignments.
type Manager struct {
	registry *Registry
}

// NewManager creates a new agent Manager.
func NewManager() *Manager {
	return &Manager{registry: NewRegistry()}
}

// Register registers an agent (stub).
func (m *Manager) Register(a *protocol.Agent) error {
	return m.registry.Add(a)
}

// Heartbeat updates agent heartbeat (stub).
func (m *Manager) Heartbeat(agentID string) error {
	return nil
}

// List returns all known agents (stub).
func (m *Manager) List() []*protocol.Agent {
	return m.registry.List()
}
