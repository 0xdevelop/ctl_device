package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xdevelop/ctl_device/pkg/protocol"
)

func setupTestRegistry(t *testing.T) (*Registry, func()) {
	dir := t.TempDir()
	reg, err := NewRegistry(dir)
	if err != nil {
		t.Fatalf("failed to create registry: %v", err)
	}
	return reg, func() {
		os.RemoveAll(dir)
	}
}

func TestRegistry_SaveLoad(t *testing.T) {
	reg, cleanup := setupTestRegistry(t)
	defer cleanup()

	agent := &protocol.Agent{
		ID:            "test-agent",
		Role:          protocol.RoleExecutor,
		Capabilities:  []string{"go", "python"},
		LastHeartbeat: time.Now(),
		Online:        true,
		CurrentTask:   "project-01",
	}

	if err := reg.Save(agent); err != nil {
		t.Fatalf("failed to save agent: %v", err)
	}

	loaded, err := reg.Load("test-agent")
	if err != nil {
		t.Fatalf("failed to load agent: %v", err)
	}
	if loaded == nil {
		t.Fatal("loaded agent is nil")
	}

	if loaded.ID != agent.ID {
		t.Errorf("expected ID %v, got %v", agent.ID, loaded.ID)
	}
	if loaded.Role != agent.Role {
		t.Errorf("expected role %v, got %v", agent.Role, loaded.Role)
	}
	if len(loaded.Capabilities) != len(agent.Capabilities) {
		t.Errorf("expected %d capabilities, got %d", len(agent.Capabilities), len(loaded.Capabilities))
	}
}

func TestRegistry_LoadNonExistent(t *testing.T) {
	reg, cleanup := setupTestRegistry(t)
	defer cleanup()

	agent, err := reg.Load("non-existent")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if agent != nil {
		t.Fatal("expected nil agent for non-existent ID")
	}
}

func TestRegistry_LoadAll(t *testing.T) {
	reg, cleanup := setupTestRegistry(t)
	defer cleanup()

	agents := []*protocol.Agent{
		{ID: "agent-1", Role: protocol.RoleExecutor, Capabilities: []string{"go"}},
		{ID: "agent-2", Role: protocol.RoleScheduler, Capabilities: []string{"python"}},
		{ID: "agent-3", Role: protocol.RoleBoth, Capabilities: []string{"go", "rust"}},
	}

	for _, a := range agents {
		if err := reg.Save(a); err != nil {
			t.Fatalf("failed to save agent %s: %v", a.ID, err)
		}
	}

	loaded, err := reg.LoadAll()
	if err != nil {
		t.Fatalf("failed to load all agents: %v", err)
	}
	if len(loaded) != len(agents) {
		t.Fatalf("expected %d agents, got %d", len(agents), len(loaded))
	}
}

func TestRegistry_Delete(t *testing.T) {
	reg, cleanup := setupTestRegistry(t)
	defer cleanup()

	agent := &protocol.Agent{
		ID:   "test-agent",
		Role: protocol.RoleExecutor,
	}

	if err := reg.Save(agent); err != nil {
		t.Fatalf("failed to save agent: %v", err)
	}

	if err := reg.Delete("test-agent"); err != nil {
		t.Fatalf("failed to delete agent: %v", err)
	}

	loaded, err := reg.Load("test-agent")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if loaded != nil {
		t.Fatal("expected nil agent after delete")
	}
}

func TestRegistry_AtomicWrite(t *testing.T) {
	reg, cleanup := setupTestRegistry(t)
	defer cleanup()

	agent := &protocol.Agent{
		ID:   "atomic-test",
		Role: protocol.RoleExecutor,
	}

	for i := 0; i < 10; i++ {
		if err := reg.Save(agent); err != nil {
			t.Fatalf("save failed: %v", err)
		}
	}

	files, err := os.ReadDir(reg.dir)
	if err != nil {
		t.Fatalf("failed to read directory: %v", err)
	}

	for _, f := range files {
		if filepath.Ext(f.Name()) == ".tmp" {
			t.Error("found leftover .tmp file")
		}
	}
}
