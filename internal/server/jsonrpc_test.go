package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/0xdevelop/ctl_device/internal/agent"
	"github.com/0xdevelop/ctl_device/internal/event"
	"github.com/0xdevelop/ctl_device/internal/project"
	"github.com/0xdevelop/ctl_device/pkg/protocol"
)

func setupTestServer(t *testing.T, token string) (*Server, *project.FileStore, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "ctl_device_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	store, err := project.NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create file store: %v", err)
	}

	eventBus := event.NewBus()
	scheduler := project.NewScheduler(store, eventBus)

	registry, err := agent.NewRegistry(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	manager, err := agent.NewManager(registry, store, eventBus)
	if err != nil {
		t.Fatalf("Failed to create agent manager: %v", err)
	}

	server, err := NewServer(":0", token, manager, scheduler, store, eventBus)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	server.SubscribeToEvents()

	cleanup := func() {
		manager.Shutdown()
		os.RemoveAll(tmpDir)
	}

	return server, store, cleanup
}

func makeRequest(t *testing.T, server *Server, method string, params interface{}) *protocol.Response {
	t.Helper()

	reqBody := protocol.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	httpReq := httptest.NewRequest("POST", "/rpc", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	server.mux.ServeHTTP(rr, httpReq)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp protocol.Response
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	return &resp
}

func TestAgentList(t *testing.T) {
	server, _, cleanup := setupTestServer(t, "")
	defer cleanup()

	resp := makeRequest(t, server, "bridge.agent.list", nil)

	if resp.Error != nil {
		t.Fatalf("RPC error: %d - %s", resp.Error.Code, resp.Error.Message)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be map, got %T", resp.Result)
	}

	agents, ok := result["agents"].([]interface{})
	if !ok {
		t.Fatalf("Expected agents to be array, got %T", result["agents"])
	}

	if len(agents) != 0 {
		t.Fatalf("Expected 0 agents, got %d", len(agents))
	}
}

func TestAgentRegister(t *testing.T) {
	server, _, cleanup := setupTestServer(t, "")
	defer cleanup()

	params := map[string]interface{}{
		"agent_id":      "test-agent",
		"role":          "executor",
		"capabilities":  []interface{}{"go", "python"},
		"resume":        false,
	}

	resp := makeRequest(t, server, "bridge.agent.register", params)

	if resp.Error != nil {
		t.Fatalf("RPC error: %d - %s", resp.Error.Code, resp.Error.Message)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be map, got %T", resp.Result)
	}

	okVal, ok := result["ok"].(bool)
	if !ok || !okVal {
		t.Fatalf("Expected ok to be true, got %v", result["ok"])
	}
}

func TestAgentHeartbeat(t *testing.T) {
	server, _, cleanup := setupTestServer(t, "")
	defer cleanup()

	// First register agent
	registerParams := map[string]interface{}{
		"agent_id":      "test-agent",
		"role":          "executor",
		"capabilities":  []interface{}{"go"},
		"resume":        false,
	}
	makeRequest(t, server, "bridge.agent.register", registerParams)

	// Then send heartbeat
	heartbeatParams := map[string]interface{}{
		"agent_id": "test-agent",
	}
	resp := makeRequest(t, server, "bridge.agent.heartbeat", heartbeatParams)

	if resp.Error != nil {
		t.Fatalf("RPC error: %d - %s", resp.Error.Code, resp.Error.Message)
	}
}

func TestProjectRegister(t *testing.T) {
	server, _, cleanup := setupTestServer(t, "")
	defer cleanup()

	params := map[string]interface{}{
		"name":            "test-project",
		"dir":             "/tmp/test-project",
		"tech":            "go",
		"test_cmd":        "go test ./...",
		"timeout_minutes": 60,
	}

	resp := makeRequest(t, server, "bridge.project.register", params)

	if resp.Error != nil {
		t.Fatalf("RPC error: %d - %s", resp.Error.Code, resp.Error.Message)
	}
}

func TestProjectList(t *testing.T) {
	server, store, cleanup := setupTestServer(t, "")
	defer cleanup()

	// Register a project first
	registerParams := map[string]interface{}{
		"name": "test-project",
		"dir":  "/tmp/test-project",
	}
	makeRequest(t, server, "bridge.project.register", registerParams)

	// List projects
	resp := makeRequest(t, server, "bridge.project.list", nil)

	if resp.Error != nil {
		t.Fatalf("RPC error: %d - %s", resp.Error.Code, resp.Error.Message)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be map, got %T", resp.Result)
	}

	projects, ok := result["projects"].([]interface{})
	if !ok {
		t.Fatalf("Expected projects to be array, got %T", result["projects"])
	}

	if len(projects) != 1 {
		t.Fatalf("Expected 1 project, got %d", len(projects))
	}

	// Verify tasks map exists
	tasks, ok := result["tasks"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected tasks to be map, got %T", result["tasks"])
	}

	if _, ok := tasks["test-project"]; !ok {
		t.Fatalf("Expected tasks for test-project")
	}

	_ = store
}

func TestTaskDispatch(t *testing.T) {
	server, _, cleanup := setupTestServer(t, "")
	defer cleanup()

	// Register project first
	projectParams := map[string]interface{}{
		"name": "test-project",
		"dir":  "/tmp/test-project",
	}
	makeRequest(t, server, "bridge.project.register", projectParams)

	// Dispatch task
	taskParams := map[string]interface{}{
		"project": "test-project",
		"task": map[string]interface{}{
			"num":         "01",
			"name":        "test-task",
			"description": "Test task description",
			"acceptance_criteria": []interface{}{
				"All tests pass",
			},
			"context_files": []interface{}{
				"plan/tasks/01.md",
			},
			"timeout_minutes": 60,
		},
	}

	resp := makeRequest(t, server, "bridge.task.dispatch", taskParams)

	if resp.Error != nil {
		t.Fatalf("RPC error: %d - %s", resp.Error.Code, resp.Error.Message)
	}
}

func TestTaskGet(t *testing.T) {
	server, _, cleanup := setupTestServer(t, "")
	defer cleanup()

	// Register project
	projectParams := map[string]interface{}{
		"name": "test-project",
		"dir":  "/tmp/test-project",
	}
	makeRequest(t, server, "bridge.project.register", projectParams)

	// Dispatch task
	taskParams := map[string]interface{}{
		"project": "test-project",
		"task": map[string]interface{}{
			"num":         "01",
			"name":        "test-task",
			"description": "Test task description",
		},
	}
	makeRequest(t, server, "bridge.task.dispatch", taskParams)

	// Get task
	getParams := map[string]interface{}{
		"project": "test-project",
	}
	resp := makeRequest(t, server, "bridge.task.get", getParams)

	if resp.Error != nil {
		t.Fatalf("RPC error: %d - %s", resp.Error.Code, resp.Error.Message)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected result to be map, got %T", resp.Result)
	}

	status, ok := result["status"].(string)
	if !ok || status != "ok" {
		t.Fatalf("Expected status to be 'ok', got %v", result["status"])
	}

	taskData, ok := result["task"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected task to be map, got %T", result["task"])
	}

	if taskData["num"] != "01" {
		t.Fatalf("Expected task num to be '01', got %v", taskData["num"])
	}
}

func TestTaskStatus(t *testing.T) {
	server, _, cleanup := setupTestServer(t, "")
	defer cleanup()

	// Setup project and task
	makeRequest(t, server, "bridge.project.register", map[string]interface{}{
		"name": "test-project",
		"dir":  "/tmp/test-project",
	})

	makeRequest(t, server, "bridge.task.dispatch", map[string]interface{}{
		"project": "test-project",
		"task": map[string]interface{}{
			"num":  "01",
			"name": "test-task",
		},
	})

	// Update task status
	statusParams := map[string]interface{}{
		"project":  "test-project",
		"task_num": "01",
		"status":   "executing",
	}
	resp := makeRequest(t, server, "bridge.task.status", statusParams)

	if resp.Error != nil {
		t.Fatalf("RPC error: %d - %s", resp.Error.Code, resp.Error.Message)
	}
}

func TestTaskComplete(t *testing.T) {
	server, _, cleanup := setupTestServer(t, "")
	defer cleanup()

	// Setup
	makeRequest(t, server, "bridge.project.register", map[string]interface{}{
		"name": "test-project",
		"dir":  "/tmp/test-project",
	})

	makeRequest(t, server, "bridge.task.dispatch", map[string]interface{}{
		"project": "test-project",
		"task": map[string]interface{}{
			"num":  "01",
			"name": "test-task",
		},
	})

	// Complete task
	completeParams := map[string]interface{}{
		"project":     "test-project",
		"task_num":    "01",
		"summary":     "Task completed successfully",
		"commit":      "abc123",
		"test_output": "All tests passed",
	}
	resp := makeRequest(t, server, "bridge.task.complete", completeParams)

	if resp.Error != nil {
		t.Fatalf("RPC error: %d - %s", resp.Error.Code, resp.Error.Message)
	}
}

func TestTaskBlock(t *testing.T) {
	server, _, cleanup := setupTestServer(t, "")
	defer cleanup()

	// Setup
	makeRequest(t, server, "bridge.project.register", map[string]interface{}{
		"name": "test-project",
		"dir":  "/tmp/test-project",
	})

	makeRequest(t, server, "bridge.task.dispatch", map[string]interface{}{
		"project": "test-project",
		"task": map[string]interface{}{
			"num":  "01",
			"name": "test-task",
		},
	})

	// Block task
	blockParams := map[string]interface{}{
		"project":  "test-project",
		"task_num": "01",
		"reason":   "Missing dependencies",
		"details":  "Cannot proceed without required libraries",
	}
	resp := makeRequest(t, server, "bridge.task.block", blockParams)

	if resp.Error != nil {
		t.Fatalf("RPC error: %d - %s", resp.Error.Code, resp.Error.Message)
	}
}

func TestTaskAdvance(t *testing.T) {
	server, _, cleanup := setupTestServer(t, "")
	defer cleanup()

	// Setup
	makeRequest(t, server, "bridge.project.register", map[string]interface{}{
		"name": "test-project",
		"dir":  "/tmp/test-project",
	})

	makeRequest(t, server, "bridge.task.dispatch", map[string]interface{}{
		"project": "test-project",
		"task": map[string]interface{}{
			"num":  "01",
			"name": "test-task",
		},
	})

	// Complete first task
	makeRequest(t, server, "bridge.task.complete", map[string]interface{}{
		"project":  "test-project",
		"task_num": "01",
		"summary":  "Done",
	})

	// Advance
	advanceParams := map[string]interface{}{
		"project": "test-project",
	}
	resp := makeRequest(t, server, "bridge.task.advance", advanceParams)

	if resp.Error != nil {
		t.Fatalf("RPC error: %d - %s", resp.Error.Code, resp.Error.Message)
	}
}

func TestAuthMiddleware(t *testing.T) {
	server, _, cleanup := setupTestServer(t, "test-token")
	defer cleanup()

	// Request without token should still go through (middleware passes through)
	reqBody := protocol.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "bridge.agent.list",
	}

	body, _ := json.Marshal(reqBody)
	httpReq := httptest.NewRequest("POST", "/rpc", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	server.mux.ServeHTTP(rr, httpReq)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rr.Code)
	}

	// Request with correct token in body
	reqBody.Auth = &protocol.AuthToken{Token: "test-token"}
	body, _ = json.Marshal(reqBody)
	httpReq = httptest.NewRequest("POST", "/rpc", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")

	rr = httptest.NewRecorder()
	server.mux.ServeHTTP(rr, httpReq)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rr.Code)
	}

	// Request with correct token in header
	reqBody.Auth = nil
	body, _ = json.Marshal(reqBody)
	httpReq = httptest.NewRequest("POST", "/rpc", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer test-token")

	rr = httptest.NewRecorder()
	server.mux.ServeHTTP(rr, httpReq)

	if rr.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rr.Code)
	}
}

func TestInvalidMethod(t *testing.T) {
	server, _, cleanup := setupTestServer(t, "")
	defer cleanup()

	resp := makeRequest(t, server, "invalid.method", nil)

	if resp.Error == nil {
		t.Fatalf("Expected error for invalid method")
	}

	if resp.Error.Code != ErrCodeInvalidParams {
		t.Fatalf("Expected error code %d, got %d", ErrCodeInvalidParams, resp.Error.Code)
	}
}

func TestInvalidParams(t *testing.T) {
	server, _, cleanup := setupTestServer(t, "")
	defer cleanup()

	// Missing required project parameter
	params := map[string]interface{}{}
	resp := makeRequest(t, server, "bridge.task.get", params)

	if resp.Error == nil {
		t.Fatalf("Expected error for missing params")
	}
}

func TestEndToEndWorkflow(t *testing.T) {
	server, _, cleanup := setupTestServer(t, "")
	defer cleanup()

	// 1. Register project
	makeRequest(t, server, "bridge.project.register", map[string]interface{}{
		"name":            "e2e-project",
		"dir":             "/tmp/e2e-project",
		"tech":            "go",
		"test_cmd":        "go test ./...",
		"timeout_minutes": 60,
	})

	// 2. Register agent
	makeRequest(t, server, "bridge.agent.register", map[string]interface{}{
		"agent_id":      "e2e-agent",
		"role":          "executor",
		"capabilities":  []interface{}{"go"},
		"resume":        false,
	})

	// 3. Dispatch task
	makeRequest(t, server, "bridge.task.dispatch", map[string]interface{}{
		"project": "e2e-project",
		"task": map[string]interface{}{
			"num":         "01",
			"name":        "init-project",
			"description": "Initialize project structure",
			"acceptance_criteria": []interface{}{
				"Project structure created",
				"All tests pass",
			},
		},
	})

	// 4. Agent gets task
	taskResp := makeRequest(t, server, "bridge.task.get", map[string]interface{}{
		"project": "e2e-project",
	})
	if taskResp.Error != nil {
		t.Fatalf("Failed to get task: %v", taskResp.Error)
	}

	// 5. Update status to executing
	makeRequest(t, server, "bridge.task.status", map[string]interface{}{
		"project":  "e2e-project",
		"task_num": "01",
		"status":   "executing",
	})

	// 6. Agent sends heartbeat
	makeRequest(t, server, "bridge.agent.heartbeat", map[string]interface{}{
		"agent_id": "e2e-agent",
	})

	// 7. Complete task
	makeRequest(t, server, "bridge.task.complete", map[string]interface{}{
		"project":     "e2e-project",
		"task_num":    "01",
		"summary":     "Project initialized",
		"commit":      "abc123def",
		"test_output": "PASS\nok  \tgithub.com/example/project\t0.001s",
	})

	// 8. Advance to next task
	makeRequest(t, server, "bridge.task.advance", map[string]interface{}{
		"project": "e2e-project",
	})

	// 9. Verify project state
	listResp := makeRequest(t, server, "bridge.project.list", nil)
	result := listResp.Result.(map[string]interface{})
	projects := result["projects"].([]interface{})
	
	if len(projects) != 1 {
		t.Fatalf("Expected 1 project, got %d", len(projects))
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
