package recovery

import "github.com/0xdevelop/ctl_device/pkg/protocol"

// Manager handles crash recovery and task resumption.
type Manager struct{}

// NewManager creates a new recovery Manager.
func NewManager() *Manager {
	return &Manager{}
}

// Recover attempts to recover in-progress tasks for an agent (stub).
func (m *Manager) Recover(agentID string) ([]*protocol.Task, error) {
	return nil, nil
}
