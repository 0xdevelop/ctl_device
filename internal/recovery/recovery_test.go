package recovery

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xdevelop/ctl_device/internal/agent"
	"github.com/0xdevelop/ctl_device/internal/event"
	"github.com/0xdevelop/ctl_device/internal/notify"
	"github.com/0xdevelop/ctl_device/internal/project"
	"github.com/0xdevelop/ctl_device/pkg/protocol"
)

func setupTestRecoveryManager(t *testing.T) (*Manager, *project.FileStore, *agent.Manager, *event.Bus, *notify.Notifier, func()) {
	t.Helper()

	dir := t.TempDir()
	store, err := project.NewFileStore(dir)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	eventBus := event.NewBus()
	notifier := notify.NewNotifier("none", "")

	scheduler := project.NewScheduler(store, eventBus)
	registry, err := agent.NewRegistry(filepath.Join(dir, "agents"))
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	agentMgr, err := agent.NewManager(registry, store, eventBus)
	if err != nil {
		t.Fatalf("Failed to create agent manager: %v", err)
	}

	recoveryMgr := NewManager(scheduler, agentMgr, notifier, eventBus)

	cleanup := func() {
		recoveryMgr.Shutdown()
		agentMgr.Shutdown()
	}

	return recoveryMgr, store, agentMgr, eventBus, notifier, cleanup
}

func TestRecoveryManager_OnAgentReconnect(t *testing.T) {
	mgr, store, agentMgr, _, _, cleanup := setupTestRecoveryManager(t)
	defer cleanup()

	ctx := context.Background()
	mgr.Start(ctx)

	projectName := "test-project"
	proj := &protocol.Project{
		Name:           projectName,
		Dir:            "/tmp/test",
		TimeoutMinutes: 120,
	}
	if err := store.SaveProject(proj); err != nil {
		t.Fatalf("Failed to save project: %v", err)
	}

	task := &protocol.Task{
		ID:          "test-project:01",
		Project:     projectName,
		Num:         "01",
		Name:        "test-task",
		Status:      protocol.TaskExecuting,
		AssignedTo:  "agent-1",
		StartedAt:   time.Now().Add(-10 * time.Minute),
		UpdatedAt:   time.Now(),
		TimeoutMinutes: 120,
	}
	if err := store.SaveTask(task); err != nil {
		t.Fatalf("Failed to save task: %v", err)
	}

	_, _ = agentMgr.Register(&agent.RegisterRequest{
		AgentID: "agent-1",
		Role:    "executor",
	})

	err := mgr.OnAgentReconnect("agent-1")
	if err != nil {
		t.Fatalf("OnAgentReconnect failed: %v", err)
	}
}

func TestRecoveryManager_OnAgentReconnect_Timeout(t *testing.T) {
	mgr, store, agentMgr, _, _, cleanup := setupTestRecoveryManager(t)
	defer cleanup()

	ctx := context.Background()
	mgr.Start(ctx)

	projectName := "test-project-timeout"
	proj := &protocol.Project{
		Name:           projectName,
		Dir:            "/tmp/test",
		TimeoutMinutes: 1,
	}
	if err := store.SaveProject(proj); err != nil {
		t.Fatalf("Failed to save project: %v", err)
	}

	task := &protocol.Task{
		ID:          "test-project-timeout:01",
		Project:     projectName,
		Num:         "01",
		Name:        "test-task",
		Status:      protocol.TaskExecuting,
		AssignedTo:  "agent-2",
		StartedAt:   time.Now().Add(-2 * time.Minute),
		UpdatedAt:   time.Now(),
		TimeoutMinutes: 1,
	}
	if err := store.SaveTask(task); err != nil {
		t.Fatalf("Failed to save task: %v", err)
	}

	_, _ = agentMgr.Register(&agent.RegisterRequest{
		AgentID: "agent-2",
		Role:    "executor",
	})

	err := mgr.OnAgentReconnect("agent-2")
	if err != nil {
		t.Fatalf("OnAgentReconnect failed: %v", err)
	}

	updatedTask, err := store.LoadTask(projectName, "01")
	if err != nil {
		t.Fatalf("Failed to load task: %v", err)
	}

	if updatedTask.Status != protocol.TaskPending {
		t.Errorf("Expected task status to be pending, got %s", updatedTask.Status)
	}
}

func TestRecoveryManager_OnSchedulerReconnect(t *testing.T) {
	mgr, store, _, _, _, cleanup := setupTestRecoveryManager(t)
	defer cleanup()

	ctx := context.Background()
	mgr.Start(ctx)

	projectName := "test-project-scheduler"
	proj := &protocol.Project{
		Name:           projectName,
		Dir:            "/tmp/test",
		TimeoutMinutes: 120,
	}
	if err := store.SaveProject(proj); err != nil {
		t.Fatalf("Failed to save project: %v", err)
	}

	tasks := []*protocol.Task{
		{
			ID:        "test-project-scheduler:01",
			Project:   projectName,
			Num:       "01",
			Name:      "completed-task",
			Status:    protocol.TaskCompleted,
			Commit:    "abc123",
			Report:    "All tests passed",
			UpdatedAt: time.Now(),
		},
		{
			ID:        "test-project-scheduler:02",
			Project:   projectName,
			Num:       "02",
			Name:      "blocked-task",
			Status:    protocol.TaskBlocked,
			Report:    "Missing dependency",
			UpdatedAt: time.Now(),
		},
		{
			ID:        "test-project-scheduler:03",
			Project:   projectName,
			Num:       "03",
			Name:      "executing-task",
			Status:    protocol.TaskExecuting,
			AssignedTo: "agent-3",
			StartedAt: time.Now().Add(-5 * time.Minute),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "test-project-scheduler:04",
			Project:   projectName,
			Num:       "04",
			Name:      "pending-task",
			Status:    protocol.TaskPending,
			UpdatedAt: time.Now(),
		},
	}

	for _, task := range tasks {
		if err := store.SaveTask(task); err != nil {
			t.Fatalf("Failed to save task: %v", err)
		}
	}

	summary, err := mgr.OnSchedulerReconnect("scheduler-1")
	if err != nil {
		t.Fatalf("OnSchedulerReconnect failed: %v", err)
	}

	if len(summary.Completed) != 1 {
		t.Errorf("Expected 1 completed task, got %d", len(summary.Completed))
	}
	if len(summary.Blocked) != 1 {
		t.Errorf("Expected 1 blocked task, got %d", len(summary.Blocked))
	}
	if len(summary.InProgress) != 1 {
		t.Errorf("Expected 1 in-progress task, got %d", len(summary.InProgress))
	}
	if len(summary.Pending) != 1 {
		t.Errorf("Expected 1 pending task, got %d", len(summary.Pending))
	}
}

func TestRecoveryManager_HandleExecutorLimit(t *testing.T) {
	mgr, store, _, _, _, cleanup := setupTestRecoveryManager(t)
	defer cleanup()

	ctx := context.Background()
	mgr.Start(ctx)

	projectName := "test-project-limit"
	proj := &protocol.Project{
		Name:           projectName,
		Dir:            "/tmp/test",
		TimeoutMinutes: 120,
	}
	if err := store.SaveProject(proj); err != nil {
		t.Fatalf("Failed to save project: %v", err)
	}

	task := &protocol.Task{
		ID:        "test-project-limit:01",
		Project:   projectName,
		Num:       "01",
		Name:      "limit-task",
		Status:    protocol.TaskExecuting,
		UpdatedAt: time.Now(),
	}
	if err := store.SaveTask(task); err != nil {
		t.Fatalf("Failed to save task: %v", err)
	}

	err := mgr.HandleExecutorLimit(projectName, "01", "agent-4")
	if err != nil {
		t.Fatalf("HandleExecutorLimit failed: %v", err)
	}

	updatedTask, err := store.LoadTask(projectName, "01")
	if err != nil {
		t.Fatalf("Failed to load task: %v", err)
	}

	if updatedTask.Status != protocol.TaskExecutorLimit {
		t.Errorf("Expected task status to be executor_limit, got %s", updatedTask.Status)
	}
}

func TestRecoveryManager_CheckTimeouts(t *testing.T) {
	mgr, store, _, _, _, cleanup := setupTestRecoveryManager(t)
	defer cleanup()

	ctx := context.Background()
	mgr.Start(ctx)

	projectName := "test-project-timeout-check"
	proj := &protocol.Project{
		Name:           projectName,
		Dir:            "/tmp/test",
		TimeoutMinutes: 1,
	}
	if err := store.SaveProject(proj); err != nil {
		t.Fatalf("Failed to save project: %v", err)
	}

	task := &protocol.Task{
		ID:          "test-project-timeout-check:01",
		Project:     projectName,
		Num:         "01",
		Name:        "timeout-task",
		Status:      protocol.TaskExecuting,
		AssignedTo:  "agent-5",
		StartedAt:   time.Now().Add(-2 * time.Minute),
		UpdatedAt:   time.Now(),
		TimeoutMinutes: 1,
	}
	if err := store.SaveTask(task); err != nil {
		t.Fatalf("Failed to save task: %v", err)
	}

	mgr.CheckTimeouts()

	time.Sleep(100 * time.Millisecond)

	updatedTask, err := store.LoadTask(projectName, "01")
	if err != nil {
		t.Fatalf("Failed to load task: %v", err)
	}

	if updatedTask.Status != protocol.TaskTimeout {
		t.Errorf("Expected task status to be timeout, got %s", updatedTask.Status)
	}
}

func TestRecoveryManager_HandlePushFailed(t *testing.T) {
	mgr, _, _, _, _, cleanup := setupTestRecoveryManager(t)
	defer cleanup()

	ctx := context.Background()
	mgr.Start(ctx)

	err := mgr.HandlePushFailed("test-project", "01", "abc123")
	if err != nil {
		t.Fatalf("HandlePushFailed failed: %v", err)
	}
}

func TestRecoveryManager_OnServerStart(t *testing.T) {
	mgr, store, agentMgr, _, _, cleanup := setupTestRecoveryManager(t)
	defer cleanup()

	ctx := context.Background()
	mgr.Start(ctx)

	projectName := "test-project-server-start"
	proj := &protocol.Project{
		Name:           projectName,
		Dir:            "/tmp/test",
		TimeoutMinutes: 120,
	}
	if err := store.SaveProject(proj); err != nil {
		t.Fatalf("Failed to save project: %v", err)
	}

	tasks := []*protocol.Task{
		{
			ID:        "test-project-server-start:01",
			Project:   projectName,
			Num:       "01",
			Name:      "completed",
			Status:    protocol.TaskCompleted,
			Commit:    "def456",
			Report:    "Done",
			UpdatedAt: time.Now(),
		},
		{
			ID:        "test-project-server-start:02",
			Project:   projectName,
			Num:       "02",
			Name:      "executing",
			Status:    protocol.TaskExecuting,
			AssignedTo: "agent-6",
			StartedAt: time.Now().Add(-5 * time.Minute),
			UpdatedAt: time.Now(),
		},
	}

	for _, task := range tasks {
		if err := store.SaveTask(task); err != nil {
			t.Fatalf("Failed to save task: %v", err)
		}
	}

	_, _ = agentMgr.Register(&agent.RegisterRequest{
		AgentID: "agent-6",
		Role:    "executor",
	})

	err := mgr.OnServerStart()
	if err != nil {
		t.Fatalf("OnServerStart failed: %v", err)
	}
}

func TestParseTaskID(t *testing.T) {
	tests := []struct {
		input     string
		project   string
		taskNum   string
	}{
		{"project:01", "project", "01"},
		{"my-project:10", "my-project", "10"},
		{"a:b:c:05", "a:b:c", "05"},
		{"single", "", "single"},
	}

	for _, tt := range tests {
		project, taskNum := parseTaskID(tt.input)
		if project != tt.project || taskNum != tt.taskNum {
			t.Errorf("parseTaskID(%q) = (%q, %q), want (%q, %q)", tt.input, project, taskNum, tt.project, tt.taskNum)
		}
	}
}

func TestRecoveryManager_EventHandling(t *testing.T) {
	mgr, store, agentMgr, eventBus, _, cleanup := setupTestRecoveryManager(t)
	defer cleanup()

	ctx := context.Background()
	mgr.Start(ctx)

	projectName := "test-project-event"
	proj := &protocol.Project{
		Name:           projectName,
		Dir:            "/tmp/test",
		TimeoutMinutes: 120,
	}
	if err := store.SaveProject(proj); err != nil {
		t.Fatalf("Failed to save project: %v", err)
	}

	task := &protocol.Task{
		ID:        "test-project-event:01",
		Project:   projectName,
		Num:       "01",
		Name:      "event-task",
		Status:    protocol.TaskExecuting,
		AssignedTo: "agent-7",
		StartedAt: time.Now().Add(-5 * time.Minute),
		UpdatedAt: time.Now(),
	}
	if err := store.SaveTask(task); err != nil {
		t.Fatalf("Failed to save task: %v", err)
	}

	_, _ = agentMgr.Register(&agent.RegisterRequest{
		AgentID: "agent-7",
		Role:    "executor",
		Resume:  true,
	})

	eventBus.Publish(event.Event{
		Type:    event.EventAgentOnline,
		AgentID: "agent-7",
		Timestamp: time.Now(),
	})

	time.Sleep(100 * time.Millisecond)
}
