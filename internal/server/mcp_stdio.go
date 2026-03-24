package server

// MCPStdioServer handles MCP connections over stdio.
type MCPStdioServer struct{}

// NewMCPStdioServer creates a new stdio-based MCP server.
func NewMCPStdioServer() *MCPStdioServer {
	return &MCPStdioServer{}
}

// Run runs the stdio server (stub).
func (s *MCPStdioServer) Run() error {
	return nil
}
