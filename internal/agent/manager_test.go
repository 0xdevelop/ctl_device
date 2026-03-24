package agent

import (
	"os"
	"testing"
	"time"

	"github.com/0xdevelop/ctl_device/internal/event"
	"github.com/0xdevelop/ctl_device/pkg/protocol"
)

type mockStore struct {
	projects map[string]*protocol.Project
	tasks    map[string][]*protocol.Task
}

func newMockStore() *mockStore {
	return &mockStore{
		projects: make(map[string]*protocol.Project),
		tasks:    make(map[string][]*protocol.Task),
	}
}

func (m *mockStore) ListTasks(projectName string) ([]*protocol.Task, error) {
	return m.tasks[projectName], nil
}

func (m *mockStore) LoadProject(name string) (*protocol.Project, error) {
	return m.projects[name], nil
}

func setupTestManager(t *testing.T) (*Manager, *Registry, *mockStore, *event.Bus, func()) {
	dir := t.TempDir()

	reg, err := NewRegistry(dir)
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}

	store := newMockStore()
	bus := event.NewBus()

	mgr, err := NewManager(reg, store, bus)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	cleanup := func() {
		mgr.Shutdown()
		os.RemoveAll(dir)
	}

	return mgr, reg, store, bus, cleanup
}

func TestManager_Register(t *testing.T) {
	mgr, _, _, bus, cleanup := setupTestManager(t)
	defer cleanup()

	ch := make(chan event.Event, 10)
	bus.Subscribe(ch, event.EventAgentOnline)

	req := &RegisterRequest{
		AgentID:      "agent-1",
		Role:         "executor",
		Capabilities: []string{"go", "python"},
	}

	resp, err := mgr.Register(req)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if !resp.OK {
		t.Fatal("expected OK=true")
	}

	agent, ok := mgr.GetAgent("agent-1")
	if !ok {
		t.Fatal("agent not found")
	}
	if agent.ID != "agent-1" {
		t.Errorf("expected agent-1, got %s", agent.ID)
	}
	if !agent.Online {
		t.Error("expected agent to be online")
	}

	select {
	case e := <-ch:
		if e.Type != event.EventAgentOnline {
			t.Errorf("expected EventAgentOnline, got %v", e.Type)
		}
		if e.AgentID != "agent-1" {
			t.Errorf("expected agent-1, got %v", e.AgentID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

func TestManager_Heartbeat(t *testing.T) {
	mgr, _, _, _, cleanup := setupTestManager(t)
	defer cleanup()

	req := &RegisterRequest{
		AgentID: "agent-1",
		Role:    "executor",
	}
	mgr.Register(req)

	time.Sleep(10 * time.Millisecond)
	before := mgr.agents["agent-1"].LastHeartbeat

	if err := mgr.Heartbeat("agent-1"); err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}

	after := mgr.agents["agent-1"].LastHeartbeat
	if !after.After(before) {
		t.Error("heartbeat timestamp not updated")
	}
}

func TestManager_HeartbeatNotFound(t *testing.T) {
	mgr, _, _, _, cleanup := setupTestManager(t)
	defer cleanup()

	err := mgr.Heartbeat("non-existent")
	if err == nil {
		t.Fatal("expected error for non-existent agent")
	}
}

func TestManager_GetOnlineExecutors(t *testing.T) {
	mgr, _, _, _, cleanup := setupTestManager(t)
	defer cleanup()

	mgr.Register(&RegisterRequest{AgentID: "exec-1", Role: "executor"})
	mgr.Register(&RegisterRequest{AgentID: "exec-2", Role: "executor"})
	mgr.Register(&RegisterRequest{AgentID: "sched-1", Role: "scheduler"})

	online := mgr.GetOnlineExecutors()
	if len(online) != 2 {
		t.Fatalf("expected 2 online executors, got %d", len(online))
	}
}

func TestManager_FindExecutorForProject(t *testing.T) {
	mgr, _, store, _, cleanup := setupTestManager(t)
	defer cleanup()

	store.projects["proj-go"] = &protocol.Project{
		Name:     "proj-go",
		Tech:     "go",
		Executor: "exec-1",
	}

	mgr.Register(&RegisterRequest{
		AgentID:      "exec-1",
		Role:         "executor",
		Capabilities: []string{"go"},
	})
	mgr.Register(&RegisterRequest{
		AgentID:      "exec-2",
		Role:         "executor",
		Capabilities: []string{"python"},
	})

	exec, err := mgr.FindExecutorForProject("proj-go")
	if err != nil {
		t.Fatalf("FindExecutorForProject failed: %v", err)
	}
	if exec.ID != "exec-1" {
		t.Errorf("expected exec-1, got %s", exec.ID)
	}
}

func TestManager_FindExecutorForProject_NoMatch(t *testing.T) {
	mgr, _, store, _, cleanup := setupTestManager(t)
	defer cleanup()

	store.projects["proj-rust"] = &protocol.Project{
		Name: "proj-rust",
		Tech: "rust",
	}

	mgr.Register(&RegisterRequest{
		AgentID:      "exec-1",
		Role:         "executor",
		Capabilities: []string{"go"},
	})

	exec, err := mgr.FindExecutorForProject("proj-rust")
	if err == nil {
		t.Error("expected error when no matching executor")
	}
	if exec != nil {
		t.Error("expected nil executor")
	}
}

func TestManager_RegisterResume(t *testing.T) {
	mgr, _, store, _, cleanup := setupTestManager(t)
	defer cleanup()

	store.tasks["proj-1"] = []*protocol.Task{
		{
			ID:      "proj-1:01",
			Project: "proj-1",
			Num:     "01",
			Status:  protocol.TaskExecuting,
		},
	}

	mgr.Register(&RegisterRequest{
		AgentID:  "exec-1",
		Role:     "executor",
		Projects: []string{"proj-1"},
	})
	mgr.SetAgentTask("exec-1", "proj-1")

	resp, err := mgr.Register(&RegisterRequest{
		AgentID: "exec-1",
		Role:    "executor",
		Resume:  true,
	})
	if err != nil {
		t.Fatalf("Register with resume failed: %v", err)
	}
	if len(resp.PendingTasks) != 1 {
		t.Fatalf("expected 1 pending task, got %d", len(resp.PendingTasks))
	}
}

func TestManager_SetAgentTask(t *testing.T) {
	mgr, _, _, _, cleanup := setupTestManager(t)
	defer cleanup()

	mgr.Register(&RegisterRequest{
		AgentID: "exec-1",
		Role:    "executor",
	})

	if err := mgr.SetAgentTask("exec-1", "proj-1:01"); err != nil {
		t.Fatalf("SetAgentTask failed: %v", err)
	}

	agent, _ := mgr.GetAgent("exec-1")
	if agent.CurrentTask != "proj-1:01" {
		t.Errorf("expected current task proj-1:01, got %s", agent.CurrentTask)
	}
}

func TestManager_ListAgents(t *testing.T) {
	mgr, _, _, _, cleanup := setupTestManager(t)
	defer cleanup()

	mgr.Register(&RegisterRequest{AgentID: "agent-1", Role: "executor"})
	mgr.Register(&RegisterRequest{AgentID: "agent-2", Role: "executor"})

	agents := mgr.ListAgents()
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}
}
