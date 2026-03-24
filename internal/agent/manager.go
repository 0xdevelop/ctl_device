package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/0xdevelop/ctl_device/internal/event"
	"github.com/0xdevelop/ctl_device/pkg/protocol"
)

// RegisterRequest represents a request to register an agent.
type RegisterRequest struct {
	AgentID      string   `json:"agent_id"`
	Role         string   `json:"role"`
	Capabilities []string `json:"capabilities"`
	Projects     []string `json:"projects,omitempty"`
	Resume       bool     `json:"resume,omitempty"`
}

// RegisterResponse represents the response to a register request.
type RegisterResponse struct {
	OK           bool           `json:"ok"`
	PendingTasks []*protocol.Task `json:"pending_tasks,omitempty"`
}

// Store is an interface for accessing projects and tasks.
type Store interface {
	ListTasks(projectName string) ([]*protocol.Task, error)
	LoadProject(name string) (*protocol.Project, error)
}

// Manager manages agent lifecycle and assignments.
type Manager struct {
	mu       sync.RWMutex
	agents   map[string]*protocol.Agent
	registry *Registry
	store    Store
	eventBus *event.Bus
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewManager creates a new agent Manager.
func NewManager(registry *Registry, store Store, eventBus *event.Bus) (*Manager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		agents:   make(map[string]*protocol.Agent),
		registry: registry,
		store:    store,
		eventBus: eventBus,
		ctx:      ctx,
		cancel:   cancel,
	}

	if err := m.loadPersistedAgents(); err != nil {
		cancel()
		return nil, err
	}

	go m.startHeartbeatWatcher(ctx)
	return m, nil
}

// loadPersistedAgents loads all persisted agents and marks them as offline.
func (m *Manager) loadPersistedAgents() error {
	agents, err := m.registry.LoadAll()
	if err != nil {
		return err
	}
	for _, a := range agents {
		a.Online = false
		a.ResumeOnline = false
		m.agents[a.ID] = a
	}
	return nil
}

// Register registers an agent (executioner comes online).
func (m *Manager) Register(req *RegisterRequest) (*RegisterResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	agent := &protocol.Agent{
		ID:            req.AgentID,
		Role:          protocol.AgentRole(req.Role),
		Capabilities:  req.Capabilities,
		LastHeartbeat: now,
		Online:        true,
		ResumeOnline:  req.Resume,
	}

	if existing, ok := m.agents[req.AgentID]; ok {
		agent.CurrentTask = existing.CurrentTask
	}

	m.agents[req.AgentID] = agent

	if err := m.registry.Save(agent); err != nil {
		return nil, err
	}

	m.eventBus.Publish(event.Event{
		Type:      event.EventAgentOnline,
		AgentID:   req.AgentID,
		Timestamp: now,
	})

	if req.Resume && agent.CurrentTask != "" {
		tasks, err := m.getPendingTasksForAgent(agent.CurrentTask)
		if err != nil {
			return nil, err
		}
		return &RegisterResponse{
			OK:           true,
			PendingTasks: tasks,
		}, nil
	}

	return &RegisterResponse{OK: true}, nil
}

// getPendingTasksForAgent finds tasks in executing or executor_offline state for the agent.
func (m *Manager) getPendingTasksForAgent(projectName string) ([]*protocol.Task, error) {
	tasks, err := m.store.ListTasks(projectName)
	if err != nil {
		return nil, err
	}

	var pending []*protocol.Task
	for _, t := range tasks {
		if t.Status == protocol.TaskExecuting || t.Status == protocol.TaskStatus("executor_offline") {
			pending = append(pending, t)
		}
	}
	return pending, nil
}

// Heartbeat updates agent heartbeat timestamp.
func (m *Manager) Heartbeat(agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, ok := m.agents[agentID]
	if !ok {
		return fmt.Errorf("agent %s not found", agentID)
	}

	agent.LastHeartbeat = time.Now()
	agent.Online = true
	return m.registry.Save(agent)
}

// GetOnlineExecutors returns all online executors.
func (m *Manager) GetOnlineExecutors() []*protocol.Agent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var online []*protocol.Agent
	for _, a := range m.agents {
		if a.Online && (a.Role == protocol.RoleExecutor || a.Role == protocol.RoleBoth) {
			online = append(online, a)
		}
	}
	return online
}

// FindExecutorForProject finds an executor for a specific project.
// Prefer specified executor, then find capabilities matching online executor.
func (m *Manager) FindExecutorForProject(projectName string) (*protocol.Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	proj, err := m.getProject(projectName)
	if err != nil {
		return nil, err
	}

	if proj.Executor != "" {
		if agent, ok := m.agents[proj.Executor]; ok && agent.Online {
			return agent, nil
		}
	}

	for _, a := range m.agents {
		if !a.Online || (a.Role != protocol.RoleExecutor && a.Role != protocol.RoleBoth) {
			continue
		}
		if len(a.Capabilities) == 0 || proj.Tech == "" {
			return a, nil
		}
		for _, cap := range a.Capabilities {
			if cap == proj.Tech {
				return a, nil
			}
		}
	}

	return nil, fmt.Errorf("no online executor found for project %s", projectName)
}

// getProject is a helper to get project info.
func (m *Manager) getProject(projectName string) (*protocol.Project, error) {
	if m.store == nil {
		return &protocol.Project{}, nil
	}
	p, err := m.store.LoadProject(projectName)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return &protocol.Project{}, nil
	}
	return p, nil
}

// GetAgent returns an agent by ID.
func (m *Manager) GetAgent(agentID string) (*protocol.Agent, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	agent, ok := m.agents[agentID]
	if !ok {
		return nil, false
	}
	return agent, true
}

// ListAgents returns all known agents.
func (m *Manager) ListAgents() []*protocol.Agent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*protocol.Agent, 0, len(m.agents))
	for _, a := range m.agents {
		out = append(out, a)
	}
	return out
}

// SetAgentTask sets the current task for an agent.
func (m *Manager) SetAgentTask(agentID, taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, ok := m.agents[agentID]
	if !ok {
		return fmt.Errorf("agent %s not found", agentID)
	}

	agent.CurrentTask = taskID
	return m.registry.Save(agent)
}

// ClearAgentTask clears the current task for an agent.
func (m *Manager) ClearAgentTask(agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, ok := m.agents[agentID]
	if !ok {
		return fmt.Errorf("agent %s not found", agentID)
	}

	agent.CurrentTask = ""
	return m.registry.Save(agent)
}

// startHeartbeatWatcher monitors agent heartbeats and marks agents offline on timeout.
func (m *Manager) startHeartbeatWatcher(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkHeartbeatTimeouts()
		}
	}
}

// checkHeartbeatTimeouts checks for agents that have exceeded the heartbeat timeout.
func (m *Manager) checkHeartbeatTimeouts() {
	m.mu.Lock()
	defer m.mu.Unlock()

	timeout := 45 * time.Second
	now := time.Now()

	for _, agent := range m.agents {
		if !agent.Online {
			continue
		}

		if now.Sub(agent.LastHeartbeat) > timeout {
			agent.Online = false
			taskID := agent.CurrentTask

			if err := m.registry.Save(agent); err != nil {
				continue
			}

			m.eventBus.Publish(event.Event{
				Type:      event.EventAgentOffline,
				AgentID:   agent.ID,
				TaskID:    taskID,
				Timestamp: now,
			})
		}
	}
}

// Shutdown gracefully shuts down the manager.
func (m *Manager) Shutdown() {
	m.cancel()
}
