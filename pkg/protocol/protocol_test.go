package protocol_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/0xdevelop/ctl_device/pkg/protocol"
)

func TestTaskJSON(t *testing.T) {
	task := protocol.Task{
		ID:             "myproject:01",
		Project:        "myproject",
		Num:            "01",
		Name:           "test task",
		Status:         protocol.TaskPending,
		UpdatedAt:      time.Now(),
		TimeoutMinutes: 60,
	}
	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal task: %v", err)
	}
	var out protocol.Task
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal task: %v", err)
	}
	if out.ID != task.ID {
		t.Errorf("expected ID %q got %q", task.ID, out.ID)
	}
	if out.Status != task.Status {
		t.Errorf("expected Status %q got %q", task.Status, out.Status)
	}
}

func TestAgentJSON(t *testing.T) {
	agent := protocol.Agent{
		ID:   "agent-1",
		Role: protocol.RoleExecutor,
	}
	data, err := json.Marshal(agent)
	if err != nil {
		t.Fatalf("marshal agent: %v", err)
	}
	var out protocol.Agent
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal agent: %v", err)
	}
	if out.Role != agent.Role {
		t.Errorf("expected Role %q got %q", agent.Role, out.Role)
	}
}

func TestAllTools(t *testing.T) {
	tools := protocol.AllTools()
	if len(tools) == 0 {
		t.Fatal("expected at least one tool")
	}
	for _, tool := range tools {
		if tool.Name == "" {
			t.Errorf("tool has empty name")
		}
		if tool.Description == "" {
			t.Errorf("tool %q has empty description", tool.Name)
		}
	}
}

func TestRPCRequestJSON(t *testing.T) {
	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "task_get",
		Params:  map[string]string{"project": "myproject"},
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	var out protocol.Request
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if out.Method != req.Method {
		t.Errorf("expected Method %q got %q", req.Method, out.Method)
	}
}
