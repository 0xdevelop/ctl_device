package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"time"
)

// Notifier sends notifications to configured channels.
type Notifier struct {
	channel string
	target  string
}

// RecoverySummary represents a summary of recovery state.
type RecoverySummary struct {
	Completed  []TaskInfo `json:"completed,omitempty"`
	Blocked    []TaskInfo `json:"blocked,omitempty"`
	InProgress []TaskInfo `json:"in_progress,omitempty"`
	Pending    []TaskInfo `json:"pending,omitempty"`
}

// TaskInfo represents basic task information.
type TaskInfo struct {
	Task      string `json:"task"`
	Commit    string `json:"commit,omitempty"`
	Report    string `json:"report,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Executor  string `json:"executor,omitempty"`
	StartedAt time.Time `json:"started_at,omitempty"`
}

// NewNotifier creates a new Notifier.
func NewNotifier(channel, target string) *Notifier {
	return &Notifier{channel: channel, target: target}
}

// Send sends a notification message.
func (n *Notifier) Send(message string) error {
	switch n.channel {
	case "openclaw-weixin", "telegram", "discord", "slack":
		return n.sendOpenClaw(message)
	case "webhook":
		return n.sendWebhook(message)
	case "none":
		log.Printf("[NOTIFY] %s", message)
		return nil
	default:
		log.Printf("[NOTIFY] %s", message)
		return nil
	}
}

// sendOpenClaw sends a message via openclaw CLI.
func (n *Notifier) sendOpenClaw(message string) error {
	cmd := exec.Command("openclaw", "message", "send", "--channel", n.channel, "--message", message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("openclaw message send failed: %w, output: %s", err, string(output))
	}
	return nil
}

// sendWebhook sends a message via HTTP POST.
func (n *Notifier) sendWebhook(message string) error {
	if n.target == "" {
		return fmt.Errorf("webhook target not configured")
	}

	payload := map[string]string{
		"message": message,
		"time":    time.Now().Format(time.RFC3339),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(n.target, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// TaskCompleted sends a task completed notification.
func (n *Notifier) TaskCompleted(project, taskNum, commit string) {
	msg := fmt.Sprintf("✅ Task %s/%s completed (commit: %s)", project, taskNum, commit)
	_ = n.Send(msg)
}

// TaskBlocked sends a task blocked notification.
func (n *Notifier) TaskBlocked(project, taskNum, reason string) {
	msg := fmt.Sprintf("🚫 Task %s/%s blocked: %s", project, taskNum, reason)
	_ = n.Send(msg)
}

// AgentOffline sends an agent offline notification.
func (n *Notifier) AgentOffline(agentID, taskID string) {
	msg := fmt.Sprintf("⚠️ Agent %s offline (task: %s)", agentID, taskID)
	_ = n.Send(msg)
}

// AgentReconnected sends an agent reconnected notification.
func (n *Notifier) AgentReconnected(agentID string) {
	msg := fmt.Sprintf("🔌 Agent %s reconnected", agentID)
	_ = n.Send(msg)
}

// TaskTimeout sends a task timeout notification.
func (n *Notifier) TaskTimeout(project, taskNum string) {
	msg := fmt.Sprintf("⏰ Task %s/%s timed out", project, taskNum)
	_ = n.Send(msg)
}

// ServerRestarted sends a server restarted notification.
func (n *Notifier) ServerRestarted(summary *RecoverySummary) {
	msg := fmt.Sprintf("🔄 Server restarted - Completed: %d, Blocked: %d, In Progress: %d, Pending: %d",
		len(summary.Completed), len(summary.Blocked), len(summary.InProgress), len(summary.Pending))
	_ = n.Send(msg)
}

// ExecutorLimit sends an executor limit notification.
func (n *Notifier) ExecutorLimit(project, taskNum, agentID string) {
	msg := fmt.Sprintf("⛔ Task %s/%s hit executor limit (agent: %s)", project, taskNum, agentID)
	_ = n.Send(msg)
}

// PushFailed sends a push failed notification.
func (n *Notifier) PushFailed(project, taskNum, commit string) {
	msg := fmt.Sprintf("❌ Task %s/%s push failed (commit: %s)", project, taskNum, commit)
	_ = n.Send(msg)
}
