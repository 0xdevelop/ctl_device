package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/0xdevelop/ctl_device/internal/agent"
	"github.com/0xdevelop/ctl_device/internal/event"
	"github.com/0xdevelop/ctl_device/internal/project"
	"github.com/0xdevelop/ctl_device/pkg/protocol"
	"golang.org/x/crypto/acme/autocert"
)

// JSON-RPC error codes
const (
	ErrCodeInvalidParams   = -32602
	ErrCodeUnauthorized    = -32001
	ErrCodeProjectNotFound = -32002
	ErrCodeTaskNotFound    = -32003
	ErrCodeAgentNotFound   = -32004
	ErrCodeNoExecutor      = -32005
)

// Server is a JSON-RPC HTTP server.
type Server struct {
	addr        string
	token       string
	mux         *http.ServeMux
	manager     *agent.Manager
	scheduler   *project.Scheduler
	store       *project.FileStore
	eventBus    *event.Bus
	ctx         context.Context
	cancel      context.CancelFunc
	sseSubs     map[chan EventSubscription]*sseSubscription
	sseMu       sync.RWMutex
	tlsEnabled  bool
	certFile    string
	keyFile     string
	autoTLS     bool
	domain      string
}

type sseSubscription struct {
	project string
	ch      chan EventSubscription
}

type EventSubscription struct {
	Type      event.EventType `json:"type"`
	Project   string          `json:"project,omitempty"`
	AgentID   string          `json:"agent_id,omitempty"`
	TaskID    string          `json:"task_id,omitempty"`
	Payload   interface{}     `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
}

// NewServer creates a new JSON-RPC server.
func NewServer(addr string, token string, manager *agent.Manager, scheduler *project.Scheduler, store *project.FileStore, eventBus *event.Bus) (*Server, error) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{
		addr:      addr,
		token:     token,
		manager:   manager,
		scheduler: scheduler,
		store:     store,
		eventBus:  eventBus,
		ctx:       ctx,
		cancel:    cancel,
		sseSubs:   make(map[chan EventSubscription]*sseSubscription),
		tlsEnabled: false,
		certFile:  "",
		keyFile:   "",
		autoTLS:   false,
		domain:    "",
	}

	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/rpc", s.handleRPC)
	s.mux.HandleFunc("/events", s.handleEvents)

	return s, nil
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	server := &http.Server{
		Addr:    s.addr,
		Handler: s.authMiddleware(s.mux),
	}
	
	if s.tlsEnabled && s.autoTLS && s.domain != "" {
		return s.startAutoTLS(server)
	}
	
	if s.tlsEnabled && s.certFile != "" && s.keyFile != "" {
		return server.ListenAndServeTLS(s.certFile, s.keyFile)
	}
	
	return server.ListenAndServe()
}

// startAutoTLS starts the server with automatic TLS using Let's Encrypt.
func (s *Server) startAutoTLS(server *http.Server) error {
	certManager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(s.domain),
		Cache:      autocert.DirCache("~/.config/ctl_device/certs"),
	}
	
	server.TLSConfig = certManager.TLSConfig()
	
	return server.ListenAndServeTLS("", "")
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.cancel()
	server := &http.Server{
		Addr:    s.addr,
		Handler: s.mux,
	}
	return server.Shutdown(ctx)
}

// SetTLSConfig configures TLS settings for the server.
func (s *Server) SetTLSConfig(enabled bool, certFile, keyFile string, autoTLS bool, domain string) {
	s.tlsEnabled = enabled
	s.certFile = certFile
	s.keyFile = keyFile
	s.autoTLS = autoTLS
	s.domain = domain
}

// authMiddleware handles token authentication.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.token == "" {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" && parts[1] == s.token {
				next.ServeHTTP(w, r)
				return
			}
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}
		r.Body.Close()

		var req protocol.Request
		if err := json.Unmarshal(body, &req); err == nil && req.Auth != nil && req.Auth.Token == s.token {
			r.Body = io.NopCloser(bytes.NewBuffer(body))
			next.ServeHTTP(w, r)
			return
		}

		r.Body = io.NopCloser(bytes.NewBuffer(body))
		next.ServeHTTP(w, r)
	})
}

// handleRPC handles POST /rpc requests.
func (s *Server) handleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Method not allowed",
		})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeError(w, nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Failed to read request body",
		})
		return
	}
	defer r.Body.Close()

	var req protocol.Request
	if err := json.Unmarshal(body, &req); err != nil {
		s.writeError(w, nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Invalid JSON-RPC request",
		})
		return
	}

	if req.JSONRPC != "2.0" {
		s.writeError(w, req.ID, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Invalid JSON-RPC version",
		})
		return
	}

	result, err := s.handleMethod(req.Method, req.Params)
	if err != nil {
		rpcErr, ok := err.(protocol.RPCError)
		if !ok {
			rpcErr = protocol.RPCError{
				Code:    ErrCodeInvalidParams,
				Message: err.Error(),
			}
		}
		s.writeError(w, req.ID, rpcErr)
		return
	}

	s.writeResponse(w, req.ID, result)
}

// handleMethod routes RPC methods to handlers.
func (s *Server) handleMethod(method string, params interface{}) (interface{}, error) {
	switch method {
	case "bridge.task.get":
		return s.handleTaskGet(params)
	case "bridge.task.status":
		return s.handleTaskStatus(params)
	case "bridge.task.complete":
		return s.handleTaskComplete(params)
	case "bridge.task.block":
		return s.handleTaskBlock(params)
	case "bridge.agent.register":
		return s.handleAgentRegister(params)
	case "bridge.agent.heartbeat":
		return s.handleAgentHeartbeat(params)
	case "bridge.agent.list":
		return s.handleAgentList(params)
	case "bridge.project.register":
		return s.handleProjectRegister(params)
	case "bridge.project.list":
		return s.handleProjectList(params)
	case "bridge.task.dispatch":
		return s.handleTaskDispatch(params)
	case "bridge.task.advance":
		return s.handleTaskAdvance(params)
	default:
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Method not found: " + method,
		}
	}
}

// handleTaskGet handles bridge.task.get
func (s *Server) handleTaskGet(params interface{}) (interface{}, error) {
	p, ok := params.(map[string]interface{})
	if !ok {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Invalid params",
		}
	}

	projectName, ok := p["project"].(string)
	if !ok || projectName == "" {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Missing or invalid project",
		}
	}

	task, err := s.scheduler.GetCurrentTask(projectName)
	if err != nil {
		return nil, protocol.RPCError{
			Code:    ErrCodeTaskNotFound,
			Message: err.Error(),
		}
	}

	if task == nil {
		return map[string]interface{}{
			"task":   nil,
			"status": "no_pending_tasks",
		}, nil
	}

	return map[string]interface{}{
		"task":   task,
		"status": "ok",
	}, nil
}

// handleTaskStatus handles bridge.task.status
func (s *Server) handleTaskStatus(params interface{}) (interface{}, error) {
	p, ok := params.(map[string]interface{})
	if !ok {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Invalid params",
		}
	}

	projectName, ok := p["project"].(string)
	if !ok || projectName == "" {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Missing or invalid project",
		}
	}

	taskNum, ok := p["task_num"].(string)
	if !ok || taskNum == "" {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Missing or invalid task_num",
		}
	}

	statusStr, ok := p["status"].(string)
	if !ok || statusStr == "" {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Missing or invalid status",
		}
	}

	status := protocol.TaskStatus(statusStr)
	if err := s.scheduler.UpdateTaskStatus(projectName, taskNum, status); err != nil {
		return nil, protocol.RPCError{
			Code:    ErrCodeTaskNotFound,
			Message: err.Error(),
		}
	}

	return map[string]interface{}{
		"status": "ok",
	}, nil
}

// handleTaskComplete handles bridge.task.complete
func (s *Server) handleTaskComplete(params interface{}) (interface{}, error) {
	p, ok := params.(map[string]interface{})
	if !ok {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Invalid params",
		}
	}

	projectName, ok := p["project"].(string)
	if !ok || projectName == "" {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Missing or invalid project",
		}
	}

	taskNum, ok := p["task_num"].(string)
	if !ok || taskNum == "" {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Missing or invalid task_num",
		}
	}

	summary, ok := p["summary"].(string)
	if !ok {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Missing or invalid summary",
		}
	}

	commit, _ := p["commit"].(string)
	testOutput, _ := p["test_output"].(string)
	issues, _ := p["issues"].(string)

	report := summary
	if testOutput != "" {
		report += "\n\nTest Output:\n" + testOutput
	}
	if issues != "" {
		report += "\n\nIssues:\n" + issues
	}

	if err := s.scheduler.CompleteTask(projectName, taskNum, commit, report); err != nil {
		return nil, protocol.RPCError{
			Code:    ErrCodeTaskNotFound,
			Message: err.Error(),
		}
	}

	return map[string]interface{}{
		"status": "ok",
	}, nil
}

// handleTaskBlock handles bridge.task.block
func (s *Server) handleTaskBlock(params interface{}) (interface{}, error) {
	p, ok := params.(map[string]interface{})
	if !ok {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Invalid params",
		}
	}

	projectName, ok := p["project"].(string)
	if !ok || projectName == "" {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Missing or invalid project",
		}
	}

	taskNum, ok := p["task_num"].(string)
	if !ok || taskNum == "" {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Missing or invalid task_num",
		}
	}

	reason, ok := p["reason"].(string)
	if !ok || reason == "" {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Missing or invalid reason",
		}
	}

	details, _ := p["details"].(string)

	fullReason := reason
	if details != "" {
		fullReason += ": " + details
	}

	if err := s.scheduler.BlockTask(projectName, taskNum, fullReason); err != nil {
		return nil, protocol.RPCError{
			Code:    ErrCodeTaskNotFound,
			Message: err.Error(),
		}
	}

	return map[string]interface{}{
		"status": "ok",
	}, nil
}

// handleAgentRegister handles bridge.agent.register
func (s *Server) handleAgentRegister(params interface{}) (interface{}, error) {
	p, ok := params.(map[string]interface{})
	if !ok {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Invalid params",
		}
	}

	agentID, ok := p["agent_id"].(string)
	if !ok || agentID == "" {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Missing or invalid agent_id",
		}
	}

	role, ok := p["role"].(string)
	if !ok || role == "" {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Missing or invalid role",
		}
	}

	capabilities, _ := p["capabilities"].([]interface{})
	var caps []string
	for _, c := range capabilities {
		if capStr, ok := c.(string); ok {
			caps = append(caps, capStr)
		}
	}

	projects, _ := p["projects"].([]interface{})
	var projs []string
	for _, pr := range projects {
		if projStr, ok := pr.(string); ok {
			projs = append(projs, projStr)
		}
	}

	resume, _ := p["resume"].(bool)

	req := &agent.RegisterRequest{
		AgentID:      agentID,
		Role:         role,
		Capabilities: caps,
		Projects:     projs,
		Resume:       resume,
	}

	resp, err := s.manager.Register(req)
	if err != nil {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: err.Error(),
		}
	}

	return resp, nil
}

// handleAgentHeartbeat handles bridge.agent.heartbeat
func (s *Server) handleAgentHeartbeat(params interface{}) (interface{}, error) {
	p, ok := params.(map[string]interface{})
	if !ok {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Invalid params",
		}
	}

	agentID, ok := p["agent_id"].(string)
	if !ok || agentID == "" {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Missing or invalid agent_id",
		}
	}

	if err := s.manager.Heartbeat(agentID); err != nil {
		return nil, protocol.RPCError{
			Code:    ErrCodeAgentNotFound,
			Message: err.Error(),
		}
	}

	return map[string]interface{}{
		"status": "ok",
	}, nil
}

// handleAgentList handles bridge.agent.list
func (s *Server) handleAgentList(params interface{}) (interface{}, error) {
	agents := s.manager.ListAgents()
	return map[string]interface{}{
		"agents": agents,
	}, nil
}

// handleProjectRegister handles bridge.project.register
func (s *Server) handleProjectRegister(params interface{}) (interface{}, error) {
	p, ok := params.(map[string]interface{})
	if !ok {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Invalid params",
		}
	}

	name, ok := p["name"].(string)
	if !ok || name == "" {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Missing or invalid name",
		}
	}

	dir, ok := p["dir"].(string)
	if !ok || dir == "" {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Missing or invalid dir",
		}
	}

	tech, _ := p["tech"].(string)
	testCmd, _ := p["test_cmd"].(string)
	executor, _ := p["executor"].(string)
	timeoutMinutes, _ := p["timeout_minutes"].(int)
	notifyChannel, _ := p["notify_channel"].(string)
	notifyTarget, _ := p["notify_target"].(string)

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
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: err.Error(),
		}
	}

	s.eventBus.Publish(event.Event{
		Type:      event.EventProjectRegistered,
		Project:   name,
		Timestamp: time.Now(),
	})

	return map[string]interface{}{
		"status": "ok",
	}, nil
}

// handleProjectList handles bridge.project.list
func (s *Server) handleProjectList(params interface{}) (interface{}, error) {
	projects, err := s.store.ListProjects()
	if err != nil {
		return nil, protocol.RPCError{
			Code:    ErrCodeProjectNotFound,
			Message: err.Error(),
		}
	}

	tasksByProject := make(map[string][]*protocol.Task)
	for _, proj := range projects {
		tasks, err := s.store.ListTasks(proj.Name)
		if err != nil {
			continue
		}
		tasksByProject[proj.Name] = tasks
	}

	return map[string]interface{}{
		"projects": projects,
		"tasks":    tasksByProject,
	}, nil
}

// handleTaskDispatch handles bridge.task.dispatch
func (s *Server) handleTaskDispatch(params interface{}) (interface{}, error) {
	p, ok := params.(map[string]interface{})
	if !ok {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Invalid params",
		}
	}

	projectName, ok := p["project"].(string)
	if !ok || projectName == "" {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Missing or invalid project",
		}
	}

	taskData, ok := p["task"].(map[string]interface{})
	if !ok {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Missing or invalid task",
		}
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
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: err.Error(),
		}
	}

	return map[string]interface{}{
		"status": "ok",
	}, nil
}

// handleTaskAdvance handles bridge.task.advance
func (s *Server) handleTaskAdvance(params interface{}) (interface{}, error) {
	p, ok := params.(map[string]interface{})
	if !ok {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Invalid params",
		}
	}

	projectName, ok := p["project"].(string)
	if !ok || projectName == "" {
		return nil, protocol.RPCError{
			Code:    ErrCodeInvalidParams,
			Message: "Missing or invalid project",
		}
	}

	if err := s.scheduler.Advance(projectName); err != nil {
		return nil, protocol.RPCError{
			Code:    ErrCodeTaskNotFound,
			Message: err.Error(),
		}
	}

	return map[string]interface{}{
		"status": "ok",
	}, nil
}

// handleEvents handles GET /events for SSE streaming.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectFilter := r.URL.Query().Get("project")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	eventCh := make(chan EventSubscription, 100)
	sub := &sseSubscription{
		project: projectFilter,
		ch:      eventCh,
	}

	s.sseMu.Lock()
	s.sseSubs[eventCh] = sub
	s.sseMu.Unlock()

	defer func() {
		s.sseMu.Lock()
		delete(s.sseSubs, eventCh)
		s.sseMu.Unlock()
		close(eventCh)
	}()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.ctx.Done():
			return
		case evt := <-eventCh:
			data, err := json.Marshal(evt)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// SubscribeToEvents subscribes to event bus and forwards to SSE subscribers.
func (s *Server) SubscribeToEvents() {
	eventCh := make(chan event.Event, 100)
	s.eventBus.Subscribe(eventCh)

	go func() {
		for evt := range eventCh {
			s.sseMu.RLock()
			for _, sub := range s.sseSubs {
				if sub.project != "" && evt.Project != "" && sub.project != evt.Project {
					continue
				}
				select {
				case sub.ch <- EventSubscription{
					Type:      evt.Type,
					Project:   evt.Project,
					AgentID:   evt.AgentID,
					TaskID:    evt.TaskID,
					Payload:   evt.Payload,
					Timestamp: evt.Timestamp,
				}:
				default:
				}
			}
			s.sseMu.RUnlock()
		}
	}()
}

func (s *Server) writeResponse(w http.ResponseWriter, id interface{}, result interface{}) {
	resp := protocol.Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) writeError(w http.ResponseWriter, id interface{}, err protocol.RPCError) {
	resp := protocol.Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &err,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
