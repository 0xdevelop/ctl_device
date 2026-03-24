package project

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/0xdevelop/ctl_device/internal/event"
	"github.com/0xdevelop/ctl_device/pkg/protocol"
)

// Scheduler coordinates task lifecycle for projects.
type Scheduler struct {
	store    *FileStore
	eventBus *event.Bus
	mu       sync.Mutex
}

// NewScheduler creates a Scheduler backed by store and bus.
func NewScheduler(store *FileStore, bus *event.Bus) *Scheduler {
	return &Scheduler{store: store, eventBus: bus}
}

// Dispatch saves task as pending under projectName.
func (s *Scheduler) Dispatch(projectName string, task *protocol.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	task.Project = projectName
	task.Status = protocol.TaskPending
	task.UpdatedAt = time.Now()
	if err := s.store.SaveTask(task); err != nil {
		return err
	}
	s.eventBus.Publish(event.Event{
		Type:    event.EventTaskStatusChanged,
		Project: projectName,
		Payload: task,
		At:      time.Now(),
	})
	return nil
}

// GetCurrentTask returns the active (executing) task or the first pending task.
func (s *Scheduler) GetCurrentTask(projectName string) (*protocol.Task, error) {
	tasks, err := s.store.ListTasks(projectName)
	if err != nil {
		return nil, err
	}
	for _, t := range tasks {
		if t.Status == protocol.TaskExecuting {
			return t, nil
		}
	}
	for _, t := range tasks {
		if t.Status == protocol.TaskPending {
			return t, nil
		}
	}
	return nil, nil
}

// UpdateTaskStatus changes the status of a task and persists it.
func (s *Scheduler) UpdateTaskStatus(projectName, taskNum string, status protocol.TaskStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, err := s.store.LoadTask(projectName, taskNum)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("task %s/%s not found", projectName, taskNum)
	}
	t.Status = status
	t.UpdatedAt = time.Now()
	if status == protocol.TaskExecuting && t.StartedAt.IsZero() {
		t.StartedAt = time.Now()
	}
	if err := s.store.SaveTask(t); err != nil {
		return err
	}
	s.eventBus.Publish(event.Event{
		Type:    event.EventTaskStatusChanged,
		Project: projectName,
		Payload: t,
		At:      time.Now(),
	})
	return nil
}

// CompleteTask marks a task completed with commit hash and report text.
func (s *Scheduler) CompleteTask(projectName, taskNum string, commit, report string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, err := s.store.LoadTask(projectName, taskNum)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("task %s/%s not found", projectName, taskNum)
	}
	t.Status = protocol.TaskCompleted
	t.Commit = commit
	t.Report = report
	t.UpdatedAt = time.Now()
	if err := s.store.SaveTask(t); err != nil {
		return err
	}
	s.eventBus.Publish(event.Event{
		Type:    event.EventTaskCompleted,
		Project: projectName,
		Payload: t,
		At:      time.Now(),
	})
	return nil
}

// BlockTask marks a task blocked with a reason.
func (s *Scheduler) BlockTask(projectName, taskNum string, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, err := s.store.LoadTask(projectName, taskNum)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("task %s/%s not found", projectName, taskNum)
	}
	t.Status = protocol.TaskBlocked
	t.Report = reason
	t.UpdatedAt = time.Now()
	if err := s.store.SaveTask(t); err != nil {
		return err
	}
	s.eventBus.Publish(event.Event{
		Type:    event.EventTaskBlocked,
		Project: projectName,
		Payload: t,
		At:      time.Now(),
	})
	return nil
}

// Advance archives the first completed/blocked task and leaves the next pending task ready.
func (s *Scheduler) Advance(projectName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tasks, err := s.store.ListTasks(projectName)
	if err != nil {
		return err
	}
	for _, t := range tasks {
		if t.Status == protocol.TaskCompleted || t.Status == protocol.TaskBlocked {
			t.Status = protocol.TaskArchived
			t.UpdatedAt = time.Now()
			if err := s.store.SaveTask(t); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

// StartSnapshotLoop saves a full snapshot every interval until ctx is cancelled.
func (s *Scheduler) StartSnapshotLoop(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = s.saveSnapshot()
			}
		}
	}()
}

func (s *Scheduler) saveSnapshot() error {
	projects, err := s.store.ListProjects()
	if err != nil {
		return err
	}
	var tasks []*protocol.Task
	for _, p := range projects {
		ts, err := s.store.ListTasks(p.Name)
		if err != nil {
			return err
		}
		tasks = append(tasks, ts...)
	}
	return s.store.SaveSnapshot(&Snapshot{
		Version:  "1",
		SavedAt:  time.Now(),
		Projects: projects,
		Tasks:    tasks,
	})
}

// CheckTimeouts periodically finds executing tasks that exceeded their timeout,
// calls notifyFn with a description, then resets them to pending.
func (s *Scheduler) CheckTimeouts(ctx context.Context, notifyFn func(string)) {
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.checkTimeoutsOnce(notifyFn)
			}
		}
	}()
}

func (s *Scheduler) checkTimeoutsOnce(notifyFn func(string)) {
	projects, err := s.store.ListProjects()
	if err != nil {
		return
	}
	now := time.Now()
	for _, p := range projects {
		tasks, err := s.store.ListTasks(p.Name)
		if err != nil {
			continue
		}
		for _, t := range tasks {
			if t.Status != protocol.TaskExecuting {
				continue
			}
			timeout := t.TimeoutMinutes
			if timeout == 0 {
				timeout = p.TimeoutMinutes
			}
			if timeout == 0 {
				continue
			}
			if !t.StartedAt.IsZero() && now.Sub(t.StartedAt) > time.Duration(timeout)*time.Minute {
				notifyFn(fmt.Sprintf("task %s/%s timeout after %d minutes", t.Project, t.Num, timeout))
				t.Status = protocol.TaskPending
				t.UpdatedAt = now
				_ = s.store.SaveTask(t)
			}
		}
	}
}
