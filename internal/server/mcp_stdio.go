package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/0xdevelop/ctl_device/internal/agent"
	"github.com/0xdevelop/ctl_device/internal/event"
	"github.com/0xdevelop/ctl_device/internal/project"
	"github.com/0xdevelop/ctl_device/pkg/protocol"
)

const (
	MCPProtocolVersion = "2024-11-05"
	MCPServerName      = "ctl_device"
	MCPServerVersion   = "0.1.0"
)

type MCPStdioServer struct {
	scheduler  *project.Scheduler
	agentMgr   *agent.Manager
	store      *project.FileStore
	eventBus   *event.Bus
	reader     *bufio.Reader
	writer     io.Writer
	mu         sync.Mutex
	initialized bool
}

type MCPInitializeRequest struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
}

type MCPInitializeResponse struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ServerInfo      MCPInfo                `json:"serverInfo"`
}

type MCPInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type MCPToolsListResponse struct {
	Tools []protocol.MCPToolSchema `json:"tools"`
}

type MCPToolCallRequest struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type MCPToolCallResponse struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

type MCPContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewMCPStdioServer(scheduler *project.Scheduler, agentMgr *agent.Manager, store *project.FileStore, eventBus *event.Bus) *MCPStdioServer {
	return &MCPStdioServer{
		scheduler: scheduler,
		agentMgr:  agentMgr,
		store:     store,
		eventBus:  eventBus,
		reader:    bufio.NewReader(os.Stdin),
		writer:    os.Stdout,
	}
}

func (s *MCPStdioServer) Run() error {
	if err := s.sendInitializeResponse(); err != nil {
		return fmt.Errorf("failed to send initialize response: %w", err)
	}

	for {
		line, err := s.reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("failed to read from stdin: %w", err)
		}

		line = []byte(strings.TrimSpace(string(line)))
		if len(line) == 0 {
			continue
		}

		var req MCPRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(nil, -32700, "Parse error: "+err.Error())
			continue
		}

		if req.JSONRPC != "2.0" {
			s.sendError(req.ID, -32600, "Invalid Request: invalid jsonrpc version")
			continue
		}

		s.handleRequest(req)
	}
}

func (s *MCPStdioServer) sendInitializeResponse() error {
	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result: MCPInitializeResponse{
			ProtocolVersion: MCPProtocolVersion,
			Capabilities: map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			ServerInfo: MCPInfo{
				Name:    MCPServerName,
				Version: MCPServerVersion,
			},
		},
	}
	return s.sendResponse(resp)
}

func (s *MCPStdioServer) handleRequest(req MCPRequest) {
	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "notifications/initialized":
		s.initialized = true
	case "tools/list":
		s.handleToolsList(req)
	case "tools/call":
		s.handleToolsCall(req)
	default:
		s.sendError(req.ID, -32601, "Method not found: "+req.Method)
	}
}

func (s *MCPStdioServer) handleInitialize(req MCPRequest) {
	s.initialized = true
	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: MCPInitializeResponse{
			ProtocolVersion: MCPProtocolVersion,
			Capabilities: map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			ServerInfo: MCPInfo{
				Name:    MCPServerName,
				Version: MCPServerVersion,
			},
		},
	}
	s.sendResponse(resp)
}

func (s *MCPStdioServer) handleToolsList(req MCPRequest) {
	tools := protocol.AllTools()
	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: MCPToolsListResponse{
			Tools: tools,
		},
	}
	s.sendResponse(resp)
}

func (s *MCPStdioServer) handleToolsCall(req MCPRequest) {
	var params MCPToolCallRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, -32602, "Invalid params: "+err.Error())
		return
	}

	result, err := s.callTool(params.Name, params.Arguments)
	if err != nil {
		resp := MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32000,
				Message: err.Error(),
			},
		}
		s.sendResponse(resp)
		return
	}

	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
	s.sendResponse(resp)
}

func (s *MCPStdioServer) callTool(name string, args map[string]interface{}) (*MCPToolCallResponse, error) {
	switch name {
	case "task_get":
		return s.callTaskGet(args)
	case "task_complete":
		return s.callTaskComplete(args)
	case "task_block":
		return s.callTaskBlock(args)
	case "task_status":
		return s.callTaskStatus(args)
	case "project_register":
		return s.callProjectRegister(args)
	case "project_list":
		return s.callProjectList()
	case "task_dispatch":
		return s.callTaskDispatch(args)
	case "task_advance":
		return s.callTaskAdvance(args)
	case "agent_list":
		return s.callAgentList()
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func (s *MCPStdioServer) callTaskGet(args map[string]interface{}) (*MCPToolCallResponse, error) {
	projectName, ok := args["project"].(string)
	if !ok || projectName == "" {
		return nil, fmt.Errorf("missing or invalid project")
	}

	task, err := s.scheduler.GetCurrentTask(projectName)
	if err != nil {
		return nil, err
	}

	if task == nil {
		return &MCPToolCallResponse{
			Content: []MCPContent{
				{Type: "text", Text: `{"status": "no_pending_tasks", "task": null}`},
			},
		}, nil
	}

	taskJSON, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return nil, err
	}

	return &MCPToolCallResponse{
		Content: []MCPContent{
			{Type: "text", Text: fmt.Sprintf(`{"status": "ok", "task": %s}`, string(taskJSON))},
		},
	}, nil
}

func (s *MCPStdioServer) callTaskComplete(args map[string]interface{}) (*MCPToolCallResponse, error) {
	projectName, _ := args["project"].(string)
	taskNum, _ := args["task_num"].(string)
	summary, _ := args["summary"].(string)
	commit, _ := args["commit"].(string)
	testOutput, _ := args["test_output"].(string)
	issues, _ := args["issues"].(string)

	if projectName == "" || taskNum == "" || summary == "" {
		return nil, fmt.Errorf("missing required fields: project, task_num, summary")
	}

	report := summary
	if testOutput != "" {
		report += "\n\nTest Output:\n" + testOutput
	}
	if issues != "" {
		report += "\n\nIssues:\n" + issues
	}

	if err := s.scheduler.CompleteTask(projectName, taskNum, commit, report); err != nil {
		return nil, err
	}

	return &MCPToolCallResponse{
		Content: []MCPContent{
			{Type: "text", Text: `{"status": "ok"}`},
		},
	}, nil
}

func (s *MCPStdioServer) callTaskBlock(args map[string]interface{}) (*MCPToolCallResponse, error) {
	projectName, _ := args["project"].(string)
	taskNum, _ := args["task_num"].(string)
	reason, _ := args["reason"].(string)
	details, _ := args["details"].(string)

	if projectName == "" || taskNum == "" || reason == "" {
		return nil, fmt.Errorf("missing required fields: project, task_num, reason")
	}

	fullReason := reason
	if details != "" {
		fullReason += ": " + details
	}

	if err := s.scheduler.BlockTask(projectName, taskNum, fullReason); err != nil {
		return nil, err
	}

	return &MCPToolCallResponse{
		Content: []MCPContent{
			{Type: "text", Text: `{"status": "ok"}`},
		},
	}, nil
}

func (s *MCPStdioServer) callTaskStatus(args map[string]interface{}) (*MCPToolCallResponse, error) {
	projectName, _ := args["project"].(string)
	status, _ := args["status"].(string)
	taskNum, _ := args["task_num"].(string)

	if projectName == "" {
		return nil, fmt.Errorf("missing required field: project")
	}

	if err := s.scheduler.UpdateTaskStatus(projectName, taskNum, protocol.TaskStatus(status)); err != nil {
		return nil, err
	}

	return &MCPToolCallResponse{
		Content: []MCPContent{
			{Type: "text", Text: `{"status": "ok"}`},
		},
	}, nil
}

func (s *MCPStdioServer) callProjectRegister(args map[string]interface{}) (*MCPToolCallResponse, error) {
	name, _ := args["name"].(string)
	dir, _ := args["dir"].(string)
	tech, _ := args["tech"].(string)
	testCmd, _ := args["test_cmd"].(string)
	executor, _ := args["executor"].(string)
	timeoutMinutes, _ := args["timeout_minutes"].(int)
	notifyChannel, _ := args["notify_channel"].(string)
	notifyTarget, _ := args["notify_target"].(string)

	if name == "" || dir == "" {
		return nil, fmt.Errorf("missing required fields: name, dir")
	}

	proj := &protocol.Project{
		Name:           name,
		Dir:            dir,
		Tech:           tech,
		TestCmd:        testCmd,
		Executor:       executor,
		TimeoutMinutes: timeoutMinutes,
		NotifyChannel:  notifyChannel,
		NotifyTarget:   notifyTarget,
	}

	if err := s.store.SaveProject(proj); err != nil {
		return nil, err
	}

	s.eventBus.Publish(event.Event{
		Type:      event.EventProjectRegistered,
		Project:   name,
		Timestamp: time.Now(),
	})

	return &MCPToolCallResponse{
		Content: []MCPContent{
			{Type: "text", Text: `{"status": "ok"}`},
		},
	}, nil
}

func (s *MCPStdioServer) callProjectList() (*MCPToolCallResponse, error) {
	projects, err := s.store.ListProjects()
	if err != nil {
		return nil, err
	}

	tasksByProject := make(map[string][]*protocol.Task)
	for _, proj := range projects {
		tasks, err := s.store.ListTasks(proj.Name)
		if err != nil {
			continue
		}
		tasksByProject[proj.Name] = tasks
	}

	result := map[string]interface{}{
		"projects": projects,
		"tasks":    tasksByProject,
	}

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, err
	}

	return &MCPToolCallResponse{
		Content: []MCPContent{
			{Type: "text", Text: string(resultJSON)},
		},
	}, nil
}

func (s *MCPStdioServer) callTaskDispatch(args map[string]interface{}) (*MCPToolCallResponse, error) {
	projectName, _ := args["project"].(string)
	taskData, ok := args["task"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid task")
	}

	if projectName == "" {
		return nil, fmt.Errorf("missing required field: project")
	}

	task := &protocol.Task{
		ID:        fmt.Sprintf("%s:%s", projectName, taskData["num"]),
		Project:   projectName,
		Num:       taskData["num"].(string),
		Name:      taskData["name"].(string),
		UpdatedAt: time.Now(),
	}

	if desc, ok := taskData["description"].(string); ok {
		task.Description = desc
	}
	if criteria, ok := taskData["acceptance_criteria"].([]interface{}); ok {
		for _, c := range criteria {
			if cs, ok := c.(string); ok {
				task.AcceptanceCriteria = append(task.AcceptanceCriteria, cs)
			}
		}
	}
	if files, ok := taskData["context_files"].([]interface{}); ok {
		for _, f := range files {
			if fs, ok := f.(string); ok {
				task.ContextFiles = append(task.ContextFiles, fs)
			}
		}
	}
	if timeout, ok := taskData["timeout_minutes"].(float64); ok {
		task.TimeoutMinutes = int(timeout)
	}

	if err := s.scheduler.Dispatch(projectName, task); err != nil {
		return nil, err
	}

	return &MCPToolCallResponse{
		Content: []MCPContent{
			{Type: "text", Text: `{"status": "ok"}`},
		},
	}, nil
}

func (s *MCPStdioServer) callTaskAdvance(args map[string]interface{}) (*MCPToolCallResponse, error) {
	projectName, _ := args["project"].(string)

	if projectName == "" {
		return nil, fmt.Errorf("missing required field: project")
	}

	if err := s.scheduler.Advance(projectName); err != nil {
		return nil, err
	}

	return &MCPToolCallResponse{
		Content: []MCPContent{
			{Type: "text", Text: `{"status": "ok"}`},
		},
	}, nil
}

func (s *MCPStdioServer) callAgentList() (*MCPToolCallResponse, error) {
	agents := s.agentMgr.ListAgents()

	result := map[string]interface{}{
		"agents": agents,
	}

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, err
	}

	return &MCPToolCallResponse{
		Content: []MCPContent{
			{Type: "text", Text: string(resultJSON)},
		},
	}, nil
}

func (s *MCPStdioServer) sendResponse(resp MCPResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(s.writer, "%s\n", data)
	if err != nil {
		return err
	}

	if f, ok := s.writer.(interface{ Sync() error }); ok {
		return f.Sync()
	}

	return nil
}

func (s *MCPStdioServer) sendError(id interface{}, code int, message string) {
	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
		},
	}
	s.sendResponse(resp)
}
