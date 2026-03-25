package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/0xdevelop/ctl_device/internal/agent"
	"github.com/0xdevelop/ctl_device/internal/event"
	"github.com/0xdevelop/ctl_device/internal/project"
)

func TestMCPStdio_Initialize(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	eventBus := event.NewBus()
	scheduler := project.NewScheduler(store, eventBus)

	registry, err := agent.NewRegistry("")
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	manager, err := agent.NewManager(registry, store, eventBus)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{}}}` + "\n"

	server := NewMCPStdioServer(scheduler, manager, store, eventBus)

	var buf bytes.Buffer
	server.writer = &buf
	server.reader = bufio.NewReader(strings.NewReader(input))

	done := make(chan error, 1)
	go func() {
		done <- server.Run()
	}()

	select {
	case err := <-done:
		if err != nil && err != io.EOF {
			t.Fatalf("Server run failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Server did not complete in time")
	}

	output := buf.String()
	if !strings.Contains(output, `"jsonrpc":"2.0"`) {
		t.Errorf("Expected jsonrpc version in output, got: %s", output)
	}

	if !strings.Contains(output, `"serverInfo"`) {
		t.Errorf("Expected serverInfo in output, got: %s", output)
	}

	if !strings.Contains(output, `"ctl_device"`) {
		t.Errorf("Expected server name in output, got: %s", output)
	}
}

func TestMCPStdio_ToolsList(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	eventBus := event.NewBus()
	scheduler := project.NewScheduler(store, eventBus)

	registry, err := agent.NewRegistry("")
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	manager, err := agent.NewManager(registry, store, eventBus)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	server := NewMCPStdioServer(scheduler, manager, store, eventBus)

	var buf bytes.Buffer
	server.writer = &buf

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	}

	data, _ := json.Marshal(req)
	server.reader = bufio.NewReader(strings.NewReader(string(data) + "\n"))

	err = server.Run()
	if err != nil && err != io.EOF {
		t.Fatalf("Server run failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"tools"`) {
		t.Errorf("Expected tools in output, got: %s", output)
	}

	if !strings.Contains(output, `"task_get"`) {
		t.Errorf("Expected task_get tool in output, got: %s", output)
	}

	if !strings.Contains(output, `"project_list"`) {
		t.Errorf("Expected project_list tool in output, got: %s", output)
	}
}

func TestMCPStdio_ToolsCall(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	eventBus := event.NewBus()
	scheduler := project.NewScheduler(store, eventBus)

	registry, err := agent.NewRegistry("")
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	manager, err := agent.NewManager(registry, store, eventBus)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	server := NewMCPStdioServer(scheduler, manager, store, eventBus)

	var buf bytes.Buffer
	server.writer = &buf

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"project_list","arguments":{}}`),
	}

	data, _ := json.Marshal(req)
	server.reader = bufio.NewReader(strings.NewReader(string(data) + "\n"))

	err = server.Run()
	if err != nil && err != io.EOF {
		t.Fatalf("Server run failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"result"`) {
		t.Errorf("Expected result in output, got: %s", output)
	}
}

func TestMCPStdio_InvalidMethod(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	eventBus := event.NewBus()
	scheduler := project.NewScheduler(store, eventBus)

	registry, err := agent.NewRegistry("")
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	manager, err := agent.NewManager(registry, store, eventBus)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	server := NewMCPStdioServer(scheduler, manager, store, eventBus)

	var buf bytes.Buffer
	server.writer = &buf

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "invalid_method",
	}

	data, _ := json.Marshal(req)
	server.reader = bufio.NewReader(strings.NewReader(string(data) + "\n"))

	err = server.Run()
	if err != nil && err != io.EOF {
		t.Fatalf("Server run failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"error"`) {
		t.Errorf("Expected error in output for invalid method, got: %s", output)
	}

	if !strings.Contains(output, `Method not found`) {
		t.Errorf("Expected 'Method not found' error message, got: %s", output)
	}
}

func TestMCPSSEServer_Start(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	eventBus := event.NewBus()
	scheduler := project.NewScheduler(store, eventBus)

	registry, err := agent.NewRegistry("")
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}
	manager, err := agent.NewManager(registry, store, eventBus)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	server := NewMCPSSEServer(":0", scheduler, manager, store, eventBus)

	done := make(chan error, 1)
	go func() {
		done <- server.Start()
	}()

	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("Failed to shutdown server: %v", err)
	}

	select {
	case err := <-done:
		if err != nil && err != http.ErrServerClosed {
			t.Fatalf("Server error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Server did not shutdown in time")
	}
}

func setupTestStore(t *testing.T) (*project.FileStore, func()) {
	dir := t.TempDir()
	store, err := project.NewFileStore(dir)
	if err != nil {
		t.Fatalf("Failed to create file store: %v", err)
	}
	return store, func() {}
}
