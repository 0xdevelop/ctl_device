package server

// MCPSSEServer handles MCP connections over Server-Sent Events (SSE).
type MCPSSEServer struct {
	addr string
}

// NewMCPSSEServer creates a new SSE-based MCP server.
func NewMCPSSEServer(addr string) *MCPSSEServer {
	return &MCPSSEServer{addr: addr}
}

// Start starts the SSE server (stub).
func (s *MCPSSEServer) Start() error {
	return nil
}
