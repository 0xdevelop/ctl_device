package server

// Dashboard serves the web dashboard.
type Dashboard struct {
	addr string
}

// NewDashboard creates a new Dashboard server.
func NewDashboard(addr string) *Dashboard {
	return &Dashboard{addr: addr}
}

// Start starts the dashboard HTTP server (stub).
func (d *Dashboard) Start() error {
	return nil
}
