package protocol

import "time"

// AgentRole describes the role an agent plays in the system.
type AgentRole string

const (
	RoleScheduler AgentRole = "scheduler"
	RoleExecutor  AgentRole = "executor"
	RoleBoth      AgentRole = "both"
)

// Agent represents a registered agent in the system.
type Agent struct {
	ID            string    `json:"id"`
	Role          AgentRole `json:"role"`
	Capabilities  []string  `json:"capabilities"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	Online        bool      `json:"online"`
	CurrentTask   string    `json:"current_task,omitempty"`
	ResumeOnline  bool      `json:"resume_online"`
}
