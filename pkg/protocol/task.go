package protocol

import "time"

// TaskStatus represents the current state of a task.
type TaskStatus string

const (
	TaskPending       TaskStatus = "pending"
	TaskExecuting     TaskStatus = "executing"
	TaskCompleted     TaskStatus = "completed"
	TaskBlocked       TaskStatus = "blocked"
	TaskExecutorLimit TaskStatus = "executor_limit"
	TaskTimeout       TaskStatus = "timeout"
	TaskArchived      TaskStatus = "archived"
)

// Task represents a unit of work assigned to an agent.
type Task struct {
	ID                 string     `json:"id"`
	Project            string     `json:"project"`
	Num                string     `json:"num"`
	Name               string     `json:"name"`
	Description        string     `json:"description"`
	AcceptanceCriteria []string   `json:"acceptance_criteria"`
	ContextFiles       []string   `json:"context_files"`
	Status             TaskStatus `json:"status"`
	AssignedTo         string     `json:"assigned_to"`
	StartedAt          time.Time  `json:"started_at,omitempty"`
	UpdatedAt          time.Time  `json:"updated_at"`
	Commit             string     `json:"commit,omitempty"`
	Report             string     `json:"report,omitempty"`
	TimeoutMinutes     int        `json:"timeout_minutes"`
}
