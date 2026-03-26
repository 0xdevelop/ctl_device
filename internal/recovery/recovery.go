package recovery

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/0xdevelop/ctl_device/internal/agent"
	"github.com/0xdevelop/ctl_device/internal/event"
	"github.com/0xdevelop/ctl_device/internal/notify"
	"github.com/0xdevelop/ctl_device/internal/project"
	"github.com/0xdevelop/ctl_device/pkg/protocol"
)

// Manager handles crash recovery and task resumption.
type Manager struct {
	scheduler *project.Scheduler
	agentMgr  *agent.Manager
	notifier  *notify.Notifier
	eventBus  *event.Bus
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewManager creates a new recovery Manager.
func NewManager(scheduler *project.Scheduler, agentMgr *agent.Manager, notifier *notify.Notifier, eventBus *event.Bus) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		scheduler: scheduler,
		agentMgr:  agentMgr,
		notifier:  notifier,
		eventBus:  eventBus,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start subscribes to all agent/task events and drives recovery logic.
func (m *Manager) Start(ctx context.Context) {
	eventCh := make(chan event.Event, 100)
	unsubscribe := m.eventBus.Subscribe(eventCh)

	go func() {
		defer unsubscribe()
		for {
			select {
			case <-ctx.Done():
				return
			case <-m.ctx.Done():
				return
			case evt := <-eventCh:
				m.handleEvent(evt)
			}
		}
	}()

	go m.startTimeoutChecker()
}

// Shutdown gracefully shuts down the recovery manager.
func (m *Manager) Shutdown() {
	if m.cancel != nil {
		m.cancel()
	}
}

// handleEvent routes events to appropriate handlers.
func (m *Manager) handleEvent(evt event.Event) {
	switch evt.Type {
	case event.EventAgentOnline:
		if evt.AgentID != "" {
			agent, ok := m.agentMgr.GetAgent(evt.AgentID)
			if ok && agent.ResumeOnline {
				_ = m.OnAgentReconnect(evt.AgentID)
			}
		}
	case event.EventAgentOffline:
		if evt.AgentID != "" && evt.TaskID != "" {
			m.notifier.AgentOffline(evt.AgentID, evt.TaskID)
		}
	}
}

// OnAgentReconnect handles an executor reconnection.
// Scenario 1: Executor disconnect/reconnect
func (m *Manager) OnAgentReconnect(agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find task assigned to this agent - check CurrentTask first, then scan all tasks
	var task *protocol.Task
	agent, ok := m.agentMgr.GetAgent(agentID)
	if ok && agent.CurrentTask != "" {
		projectName, taskNum := parseTaskID(agent.CurrentTask)
		task, _ = m.scheduler.GetCurrentTask(projectName)
		if task != nil && task.Num != taskNum {
			task = nil
		}
	}

	// Fall back: scan all projects for a task assigned to this agent
	if task == nil {
		projects, _ := m.listAllProjects()
		for _, proj := range projects {
			tasks, _ := m.listTasksForProject(proj.Name)
			for _, t := range tasks {
				if t.AssignedTo == agentID && (t.Status == protocol.TaskExecuting) {
					task = t
					break
				}
			}
			if task != nil {
				break
			}
		}
	}

	if task == nil {
		return nil // no task to recover
	}

	projectName := task.Project
	taskNum := task.Num
	taskID := task.ID

	timeoutMinutes := task.TimeoutMinutes
	if timeoutMinutes == 0 {
		proj, _ := m.loadProject(projectName)
		if proj != nil {
			timeoutMinutes = proj.TimeoutMinutes
		}
	}
	if timeoutMinutes == 0 {
		timeoutMinutes = 120
	}

	elapsed := time.Since(task.StartedAt)
	if elapsed > time.Duration(timeoutMinutes)*time.Minute {
		log.Printf("[RECOVERY] Task %s timed out (%v > %vm), resetting to pending", taskID, elapsed, timeoutMinutes)
		if err := m.scheduler.UpdateTaskStatus(projectName, taskNum, protocol.TaskPending); err != nil {
			return err
		}
		if err := m.agentMgr.ClearAgentTask(agentID); err != nil {
			return err
		}
		return nil
	}

	log.Printf("[RECOVERY] Task %s resuming for agent %s", taskID, agentID)
	m.notifier.AgentReconnected(agentID)

	return nil
}

// OnServerStart handles server restart recovery.
// Scenario 2: Server restart
func (m *Manager) OnServerStart() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	projects, err := m.listAllProjects()
	if err != nil {
		return err
	}

	var summary notify.RecoverySummary

	for _, proj := range projects {
		tasks, err := m.listTasksForProject(proj.Name)
		if err != nil {
			continue
		}

		for _, task := range tasks {
			switch task.Status {
			case protocol.TaskCompleted:
				summary.Completed = append(summary.Completed, notify.TaskInfo{
					Task:   task.ID,
					Commit: task.Commit,
					Report: task.Report,
				})
			case protocol.TaskBlocked, protocol.TaskExecutorLimit, protocol.TaskTimeout:
				summary.Blocked = append(summary.Blocked, notify.TaskInfo{
					Task:   task.ID,
					Reason: task.Report,
				})
			case protocol.TaskExecuting:
				executor := task.AssignedTo
				if executor != "" {
					agent, ok := m.agentMgr.GetAgent(executor)
					if ok && agent.Online {
						summary.InProgress = append(summary.InProgress, notify.TaskInfo{
							Task:      task.ID,
							Executor:  executor,
							StartedAt: task.StartedAt,
						})
						m.notifier.AgentReconnected(executor)
					} else {
						summary.InProgress = append(summary.InProgress, notify.TaskInfo{
							Task:      task.ID,
							Executor:  executor,
							StartedAt: task.StartedAt,
						})
					}
				}
			case protocol.TaskPending:
				summary.Pending = append(summary.Pending, notify.TaskInfo{
					Task: task.ID,
				})
			}
		}
	}

	m.checkAndResetTimeoutTasks(&summary)

	log.Printf("[RECOVERY] Server started - Completed: %d, Blocked: %d, In Progress: %d, Pending: %d",
		len(summary.Completed), len(summary.Blocked), len(summary.InProgress), len(summary.Pending))

	m.notifier.ServerRestarted(&summary)

	return nil
}

// OnSchedulerReconnect handles scheduler (OpenClaw) reconnection.
// Scenario 3: Scheduler disconnect/reconnect
func (m *Manager) OnSchedulerReconnect(agentID string) (*notify.RecoverySummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	projects, err := m.listAllProjects()
	if err != nil {
		return nil, err
	}

	summary := notify.RecoverySummary{}

	for _, proj := range projects {
		tasks, err := m.listTasksForProject(proj.Name)
		if err != nil {
			continue
		}

		for _, task := range tasks {
			switch task.Status {
			case protocol.TaskCompleted:
				summary.Completed = append(summary.Completed, notify.TaskInfo{
					Task:   task.ID,
					Commit: task.Commit,
					Report: task.Report,
				})
			case protocol.TaskBlocked, protocol.TaskExecutorLimit, protocol.TaskTimeout:
				summary.Blocked = append(summary.Blocked, notify.TaskInfo{
					Task:   task.ID,
					Reason: task.Report,
				})
			case protocol.TaskExecuting:
				summary.InProgress = append(summary.InProgress, notify.TaskInfo{
					Task:      task.ID,
					Executor:  task.AssignedTo,
					StartedAt: task.StartedAt,
				})
			case protocol.TaskPending:
				summary.Pending = append(summary.Pending, notify.TaskInfo{
					Task: task.ID,
				})
			}
		}
	}

	log.Printf("[RECOVERY] Scheduler %s reconnected - returning summary", agentID)

	return &summary, nil
}

// HandleExecutorLimit handles token limit scenario.
// Scenario 4: Token limit (executor_limit status)
func (m *Manager) HandleExecutorLimit(project, taskNum, agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	taskID := fmt.Sprintf("%s:%s", project, taskNum)

	if err := m.scheduler.UpdateTaskStatus(project, taskNum, protocol.TaskExecutorLimit); err != nil {
		return err
	}

	m.notifier.ExecutorLimit(project, taskNum, agentID)

	log.Printf("[RECOVERY] Task %s marked as executor_limit", taskID)

	return nil
}

// startTimeoutChecker periodically checks for timed out tasks.
// Scenario 5: Timeout auto-reset
func (m *Manager) startTimeoutChecker() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.CheckTimeouts()
		}
	}
}

// CheckTimeouts checks all executing tasks for timeout.
func (m *Manager) CheckTimeouts() {
	m.mu.Lock()
	defer m.mu.Unlock()

	projects, err := m.listAllProjects()
	if err != nil {
		return
	}

	now := time.Now()

	for _, proj := range projects {
		tasks, err := m.listTasksForProject(proj.Name)
		if err != nil {
			continue
		}

		for _, task := range tasks {
			if task.Status != protocol.TaskExecuting {
				continue
			}

			timeoutMinutes := task.TimeoutMinutes
			if timeoutMinutes == 0 {
				timeoutMinutes = proj.TimeoutMinutes
			}
			if timeoutMinutes == 0 {
				timeoutMinutes = 120
			}

			if !task.StartedAt.IsZero() && now.Sub(task.StartedAt) > time.Duration(timeoutMinutes)*time.Minute {
				log.Printf("[RECOVERY] Task %s/%s timed out after %d minutes", task.Project, task.Num, timeoutMinutes)
				m.notifier.TaskTimeout(task.Project, task.Num)

				if err := m.scheduler.UpdateTaskStatus(task.Project, task.Num, protocol.TaskTimeout); err != nil {
					continue
				}

				go m.scheduleTimeoutReset(task.Project, task.Num)
			}
		}
	}
}

// scheduleTimeoutReset waits 30 minutes then resets the task to pending.
func (m *Manager) scheduleTimeoutReset(project, taskNum string) {
	resetDelay := 30 * time.Minute
	select {
	case <-time.After(resetDelay):
		m.mu.Lock()
		defer m.mu.Unlock()

		task, err := m.loadTask(project, taskNum)
		if err != nil || task == nil {
			return
		}

		if task.Status != protocol.TaskTimeout {
			return
		}

		if err := m.scheduler.UpdateTaskStatus(project, taskNum, protocol.TaskPending); err != nil {
			return
		}

		if task.AssignedTo != "" {
			_ = m.agentMgr.ClearAgentTask(task.AssignedTo)
		}

		log.Printf("[RECOVERY] Task %s/%s reset to pending after 30min timeout wait", project, taskNum)
	case <-m.ctx.Done():
		return
	}
}

// HandlePushFailed handles git push failure scenario.
// Scenario 6: Git push failure
func (m *Manager) HandlePushFailed(project, taskNum, commit string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	taskID := fmt.Sprintf("%s:%s", project, taskNum)

	log.Printf("[RECOVERY] Task %s push failed (commit: %s), marked as commit_pending", taskID, commit)

	m.notifier.PushFailed(project, taskNum, commit)

	return nil
}

// checkAndResetTimeoutTasks checks and resets timeout tasks during server start.
func (m *Manager) checkAndResetTimeoutTasks(summary *notify.RecoverySummary) {
	for i, taskInfo := range summary.InProgress {
		project, taskNum := parseTaskID(taskInfo.Task)
		task, err := m.loadTask(project, taskNum)
		if err != nil || task == nil {
			continue
		}

		timeoutMinutes := task.TimeoutMinutes
		if timeoutMinutes == 0 {
			proj, _ := m.loadProject(project)
			if proj != nil {
				timeoutMinutes = proj.TimeoutMinutes
			}
		}
		if timeoutMinutes == 0 {
			timeoutMinutes = 120
		}

		if !task.StartedAt.IsZero() && time.Since(task.StartedAt) > time.Duration(timeoutMinutes)*time.Minute {
			log.Printf("[RECOVERY] Task %s timed out during server start, resetting", taskInfo.Task)

			if err := m.scheduler.UpdateTaskStatus(project, taskNum, protocol.TaskPending); err != nil {
				continue
			}

			if task.AssignedTo != "" {
				_ = m.agentMgr.ClearAgentTask(task.AssignedTo)
			}

			summary.InProgress = append(summary.InProgress[:i], summary.InProgress[i+1:]...)
			summary.Pending = append(summary.Pending, notify.TaskInfo{Task: taskInfo.Task})
		}
	}
}

// Helper functions

func parseTaskID(taskID string) (project, taskNum string) {
	// Format: "project:taskNum"
	for i := len(taskID) - 1; i >= 0; i-- {
		if taskID[i] == ':' {
			return taskID[:i], taskID[i+1:]
		}
	}
	return "", taskID
}

func (m *Manager) listAllProjects() ([]*protocol.Project, error) {
	if m.scheduler == nil {
		return nil, nil
	}
	store := m.getStore()
	if store == nil {
		return nil, nil
	}
	return store.ListProjects()
}

func (m *Manager) listTasksForProject(projectName string) ([]*protocol.Task, error) {
	if m.scheduler == nil {
		return nil, nil
	}
	store := m.getStore()
	if store == nil {
		return nil, nil
	}
	return store.ListTasks(projectName)
}

func (m *Manager) loadTask(project, taskNum string) (*protocol.Task, error) {
	store := m.getStore()
	if store == nil {
		return nil, nil
	}
	return store.LoadTask(project, taskNum)
}

func (m *Manager) loadProject(name string) (*protocol.Project, error) {
	store := m.getStore()
	if store == nil {
		return nil, nil
	}
	return store.LoadProject(name)
}

func (m *Manager) getStore() *project.FileStore {
	if m.scheduler == nil {
		return nil
	}
	return m.scheduler.GetStore()
}
