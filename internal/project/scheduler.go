package project

import "github.com/0xdevelop/ctl_device/pkg/protocol"

// Scheduler handles task scheduling and advancement.
type Scheduler struct {
	store *Store
}

// NewScheduler creates a new Scheduler.
func NewScheduler(store *Store) *Scheduler {
	return &Scheduler{store: store}
}

// Advance moves the project to the next pending task (stub).
func (sc *Scheduler) Advance(projectName string) (*protocol.Task, error) {
	return nil, nil
}

// Dispatch assigns a task to an agent (stub).
func (sc *Scheduler) Dispatch(t *protocol.Task, agentID string) error {
	return nil
}
