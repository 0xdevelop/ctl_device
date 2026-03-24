package server

// JSONRPCHandler handles JSON-RPC 2.0 requests.
type JSONRPCHandler struct{}

// NewJSONRPCHandler creates a new JSON-RPC handler.
func NewJSONRPCHandler() *JSONRPCHandler {
	return &JSONRPCHandler{}
}

// Handle processes a raw JSON-RPC request and returns a raw JSON response (stub).
func (h *JSONRPCHandler) Handle(req []byte) ([]byte, error) {
	return nil, nil
}
