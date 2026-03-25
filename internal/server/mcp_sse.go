package server

import (
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
)

type MCPSSEServer struct {
	addr       string
	scheduler  *project.Scheduler
	agentMgr   *agent.Manager
	store      *project.FileStore
	eventBus   *event.Bus
	httpServer *http.Server
	sessions   map[string]*MCPSession
	sessionsMu sync.RWMutex
}

type MCPSession struct {
	ID          string
	CreatedAt   time.Time
	LastActive  time.Time
	EventChan   chan MCPSSEEvent
	mu          sync.RWMutex
	initialized bool
}

type MCPSSEEvent struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

type MCPSSEMessageRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type MCPSSEMessageResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

func NewMCPSSEServer(addr string, scheduler *project.Scheduler, agentMgr *agent.Manager, store *project.FileStore, eventBus *event.Bus) *MCPSSEServer {
	s := &MCPSSEServer{
		addr:      addr,
		scheduler: scheduler,
		agentMgr:  agentMgr,
		store:     store,
		eventBus:  eventBus,
		sessions:  make(map[string]*MCPSession),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/sse", s.handleSSE)
	mux.HandleFunc("/message", s.handleMessage)

	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return s
}

func (s *MCPSSEServer) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *MCPSSEServer) Shutdown(ctx context.Context) error {
	s.sessionsMu.Lock()
	for _, session := range s.sessions {
		close(session.EventChan)
	}
	s.sessions = make(map[string]*MCPSession)
	s.sessionsMu.Unlock()

	return s.httpServer.Shutdown(ctx)
}

func (s *MCPSSEServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.URL.Query().Get("session")
	if sessionID == "" {
		sessionID = generateSessionID()
	}

	session := &MCPSession{
		ID:         sessionID,
		CreatedAt:  time.Now(),
		LastActive: time.Now(),
		EventChan:  make(chan MCPSSEEvent, 100),
	}

	s.sessionsMu.Lock()
	s.sessions[sessionID] = session
	s.sessionsMu.Unlock()

	defer func() {
		s.sessionsMu.Lock()
		delete(s.sessions, sessionID)
		s.sessionsMu.Unlock()
		close(session.EventChan)
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()

	session.EventChan <- MCPSSEEvent{
		Type:      "endpoint",
		Data:      "/message?session=" + sessionID,
		Timestamp: time.Now(),
	}

	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-session.EventChan:
			if !ok {
				return
			}

			data, err := json.Marshal(evt)
			if err != nil {
				continue
			}

			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (s *MCPSSEServer) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.URL.Query().Get("session")
	if sessionID == "" {
		http.Error(w, "Missing session parameter", http.StatusBadRequest)
		return
	}

	s.sessionsMu.RLock()
	session, exists := s.sessions[sessionID]
	s.sessionsMu.RUnlock()

	if !exists {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	session.mu.Lock()
	session.LastActive = time.Now()
	session.mu.Unlock()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.sendErrorResponse(session, nil, -32700, "Failed to read request body")
		return
	}
	defer r.Body.Close()

	var req MCPSSEMessageRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.sendErrorResponse(session, nil, -32700, "Parse error: "+err.Error())
		return
	}

	if req.JSONRPC != "2.0" {
		s.sendErrorResponse(session, req.ID, -32600, "Invalid Request: invalid jsonrpc version")
		return
	}

	switch req.Method {
	case "initialize":
		s.handleInitialize(session, req)
	case "notifications/initialized":
		session.mu.Lock()
		session.initialized = true
		session.mu.Unlock()
	case "tools/list":
		s.handleToolsList(session, req)
	case "tools/call":
		s.handleToolsCall(session, req)
	default:
		s.sendErrorResponse(session, req.ID, -32601, "Method not found: "+req.Method)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "accepted",
	})
}

func (s *MCPSSEServer) handleInitialize(session *MCPSession, req MCPSSEMessageRequest) {
	session.mu.Lock()
	session.initialized = true
	session.mu.Unlock()

	resp := MCPSSEMessageResponse{
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

	session.EventChan <- MCPSSEEvent{
		Type:      "response",
		Data:      resp,
		Timestamp: time.Now(),
	}
}

func (s *MCPSSEServer) handleToolsList(session *MCPSession, req MCPSSEMessageRequest) {
	tools := protocol.AllTools()

	resp := MCPSSEMessageResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: MCPToolsListResponse{
			Tools: tools,
		},
	}

	session.EventChan <- MCPSSEEvent{
		Type:      "response",
		Data:      resp,
		Timestamp: time.Now(),
	}
}

func (s *MCPSSEServer) handleToolsCall(session *MCPSession, req MCPSSEMessageRequest) {
	var params MCPToolCallRequest
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendErrorResponse(session, req.ID, -32602, "Invalid params: "+err.Error())
		return
	}

	stdioServer := NewMCPStdioServer(s.scheduler, s.agentMgr, s.store, s.eventBus)
	result, err := stdioServer.callTool(params.Name, params.Arguments)

	var resp MCPSSEMessageResponse
	if err != nil {
		resp = MCPSSEMessageResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32000,
				Message: err.Error(),
			},
		}
	} else {
		resp = MCPSSEMessageResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}
	}

	session.EventChan <- MCPSSEEvent{
		Type:      "response",
		Data:      resp,
		Timestamp: time.Now(),
	}
}

func (s *MCPSSEServer) sendErrorResponse(session *MCPSession, id interface{}, code int, message string) {
	resp := MCPSSEMessageResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
		},
	}

	session.EventChan <- MCPSSEEvent{
		Type:      "error",
		Data:      resp,
		Timestamp: time.Now(),
	}
}

func generateSessionID() string {
	return fmt.Sprintf("sess_%d", time.Now().UnixNano())
}

type MCPSSEHandler struct {
	server *MCPSSEServer
}

func NewMCPSSEHandler(server *MCPSSEServer) *MCPSSEHandler {
	return &MCPSSEHandler{server: server}
}

func (h *MCPSSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/sse") {
		h.server.handleSSE(w, r)
	} else if strings.HasSuffix(r.URL.Path, "/message") {
		h.server.handleMessage(w, r)
	} else {
		http.NotFound(w, r)
	}
}
