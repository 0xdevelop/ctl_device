package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Mode != "full" {
		t.Errorf("Expected Mode 'full', got %s", cfg.Mode)
	}
	if cfg.Connect != "" {
		t.Errorf("Expected empty Connect, got %s", cfg.Connect)
	}
	if cfg.Server.MCPPort != 3710 {
		t.Errorf("Expected MCPPort 3710, got %d", cfg.Server.MCPPort)
	}
	if cfg.Server.JSONRPCPort != 3711 {
		t.Errorf("Expected JSONRPCPort 3711, got %d", cfg.Server.JSONRPCPort)
	}
	if cfg.Server.DashboardPort != 3712 {
		t.Errorf("Expected DashboardPort 3712, got %d", cfg.Server.DashboardPort)
	}
	if cfg.Server.Bind != "0.0.0.0" {
		t.Errorf("Expected Bind '0.0.0.0', got %s", cfg.Server.Bind)
	}
	if cfg.Client.Role != "executor" {
		t.Errorf("Expected Client.Role 'executor', got %s", cfg.Client.Role)
	}
	if cfg.Client.Capabilities == nil {
		t.Error("Expected Client.Capabilities to be initialized, got nil")
	}
	if cfg.Notify.Channel != "none" {
		t.Errorf("Expected Notify.Channel 'none', got %s", cfg.Notify.Channel)
	}
}

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
mode: full
connect: ""
server:
  mcp_port: 3720
  jsonrpc_port: 3721
  dashboard_port: 3722
  bind: "127.0.0.1"
  token: "test-token"
  state_dir: "` + tmpDir + `"
client:
  agent_id: "test-agent"
  role: "executor"
  capabilities: ["go"]
notify:
  channel: "webhook"
  webhook_url: "https://example.com/webhook"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Mode != "full" {
		t.Errorf("Expected Mode 'full', got %s", cfg.Mode)
	}
	if cfg.Server.MCPPort != 3720 {
		t.Errorf("Expected MCPPort 3720, got %d", cfg.Server.MCPPort)
	}
	if cfg.Server.JSONRPCPort != 3721 {
		t.Errorf("Expected JSONRPCPort 3721, got %d", cfg.Server.JSONRPCPort)
	}
	if cfg.Server.Token != "test-token" {
		t.Errorf("Expected Token 'test-token', got %s", cfg.Server.Token)
	}
	if cfg.Client.AgentID != "test-agent" {
		t.Errorf("Expected Client.AgentID 'test-agent', got %s", cfg.Client.AgentID)
	}
	if cfg.Notify.Channel != "webhook" {
		t.Errorf("Expected Notify.Channel 'webhook', got %s", cfg.Notify.Channel)
	}
}

func TestLoadConfig_ConnectSetsClientMode(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
connect: "http://192.168.1.100:3711"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Mode != "client" {
		t.Errorf("Expected Mode 'client' when connect is set, got %s", cfg.Mode)
	}
	if cfg.Connect != "http://192.168.1.100:3711" {
		t.Errorf("Expected Connect 'http://192.168.1.100:3711', got %s", cfg.Connect)
	}
}

func TestDefaultServerConfig(t *testing.T) {
	cfg := DefaultServerConfig()
	
	if cfg.Server.MCPPort != 3710 {
		t.Errorf("Expected MCPPort 3710, got %d", cfg.Server.MCPPort)
	}
	
	if cfg.Server.JSONRPCPort != 3711 {
		t.Errorf("Expected JSONRPCPort 3711, got %d", cfg.Server.JSONRPCPort)
	}
	
	if cfg.Server.DashboardPort != 3712 {
		t.Errorf("Expected DashboardPort 3712, got %d", cfg.Server.DashboardPort)
	}
	
	if cfg.Server.Bind != "0.0.0.0" {
		t.Errorf("Expected Bind '0.0.0.0', got %s", cfg.Server.Bind)
	}
	
	if cfg.Server.Token != "" {
		t.Errorf("Expected empty Token, got %s", cfg.Server.Token)
	}
	
	if cfg.Server.TLS.Enabled != false {
		t.Errorf("Expected TLS disabled, got %v", cfg.Server.TLS.Enabled)
	}
	
	if cfg.Notify.Channel != "none" {
		t.Errorf("Expected Notify channel 'none', got %s", cfg.Notify.Channel)
	}
}

func TestDefaultClientConfig(t *testing.T) {
	cfg := DefaultClientConfig()
	
	if cfg.Server != "http://localhost:3711" {
		t.Errorf("Expected Server 'http://localhost:3711', got %s", cfg.Server)
	}
	
	if cfg.Token != "" {
		t.Errorf("Expected empty Token, got %s", cfg.Token)
	}
	
	if cfg.AgentID != "" {
		t.Errorf("Expected empty AgentID, got %s", cfg.AgentID)
	}
	
	if cfg.Role != "executor" {
		t.Errorf("Expected Role 'executor', got %s", cfg.Role)
	}
	
	if cfg.Capabilities == nil {
		t.Error("Expected Capabilities to be initialized, got nil")
	}
}

func TestLoadServerConfig_Complete(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")
	
	configContent := `
server:
  mcp_port: 3720
  jsonrpc_port: 3721
  dashboard_port: 3722
  bind: "127.0.0.1"
  token: "test-token"
  state_dir: "` + tmpDir + `"
  snapshot_interval_seconds: 60
  heartbeat_timeout_seconds: 90
  tls:
    enabled: true
    cert_file: "/path/to/cert"
    key_file: "/path/to/key"
    auto_tls: false
    domain: "example.com"

notify:
  channel: "webhook"
  webhook_url: "https://example.com/webhook"

projects:
  - name: test-project
    dir: ` + tmpDir + `/test-project
    tech: go
    test_cmd: "go test ./..."
    executor: "any"
    timeout_minutes: 60
`
	
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
	
	cfg, err := LoadServerConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	
	if cfg.Server.MCPPort != 3720 {
		t.Errorf("Expected MCPPort 3720, got %d", cfg.Server.MCPPort)
	}
	
	if cfg.Server.JSONRPCPort != 3721 {
		t.Errorf("Expected JSONRPCPort 3721, got %d", cfg.Server.JSONRPCPort)
	}
	
	if cfg.Server.Bind != "127.0.0.1" {
		t.Errorf("Expected Bind '127.0.0.1', got %s", cfg.Server.Bind)
	}
	
	if cfg.Server.Token != "test-token" {
		t.Errorf("Expected Token 'test-token', got %s", cfg.Server.Token)
	}
	
	if cfg.Server.TLS.Enabled != true {
		t.Errorf("Expected TLS enabled, got %v", cfg.Server.TLS.Enabled)
	}
	
	if cfg.Server.TLS.CertFile != "/path/to/cert" {
		t.Errorf("Expected CertFile '/path/to/cert', got %s", cfg.Server.TLS.CertFile)
	}
	
	if len(cfg.Projects) != 1 {
		t.Fatalf("Expected 1 project, got %d", len(cfg.Projects))
	}
	
	if cfg.Projects[0].Name != "test-project" {
		t.Errorf("Expected project name 'test-project', got %s", cfg.Projects[0].Name)
	}
}

func TestLoadServerConfig_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")
	
	configContent := `
server:
  mcp_port: 3710
  jsonrpc_port: 3711
`
	
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
	
	cfg, err := LoadServerConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	
	if cfg.Server.DashboardPort != 3712 {
		t.Errorf("Expected default DashboardPort 3712, got %d", cfg.Server.DashboardPort)
	}
	
	if cfg.Server.Bind != "0.0.0.0" {
		t.Errorf("Expected default Bind '0.0.0.0', got %s", cfg.Server.Bind)
	}
}

func TestLoadClientConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "client.yaml")
	
	configContent := `
server: "http://192.168.1.100:3711"
token: "client-token"
agent_id: "test-agent"
role: "executor"
capabilities: ["go", "python"]
`
	
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
	
	cfg, err := LoadClientConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load client config: %v", err)
	}
	
	if cfg.Server != "http://192.168.1.100:3711" {
		t.Errorf("Expected Server 'http://192.168.1.100:3711', got %s", cfg.Server)
	}
	
	if cfg.Token != "client-token" {
		t.Errorf("Expected Token 'client-token', got %s", cfg.Token)
	}
	
	if cfg.AgentID != "test-agent" {
		t.Errorf("Expected AgentID 'test-agent', got %s", cfg.AgentID)
	}
	
	if len(cfg.Capabilities) != 2 {
		t.Errorf("Expected 2 capabilities, got %d", len(cfg.Capabilities))
	}
}

func TestLoadClientConfig_NonExistent(t *testing.T) {
	cfg, err := LoadClientConfig("/non/existent/path.yaml")
	
	if err != nil {
		t.Errorf("Expected no error for non-existent file (returns default), got: %v", err)
	}
	
	if cfg == nil {
		t.Fatal("Expected default config to be returned, got nil")
	}
	
	if cfg.Server != "http://localhost:3711" {
		t.Errorf("Expected default Server, got %s", cfg.Server)
	}
}

func TestExpandTilde(t *testing.T) {
	homeDir, _ := os.UserHomeDir()
	
	result := ExpandTilde("~/test/path")
	expected := filepath.Join(homeDir, "test/path")
	
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
	
	result = ExpandTilde("/absolute/path")
	if result != "/absolute/path" {
		t.Errorf("Expected /absolute/path, got %s", result)
	}
}

func TestApplyClientConfigOverrides(t *testing.T) {
	cfg := DefaultClientConfig()
	
	ApplyClientConfigOverrides(cfg, "http://override:3711", "override-token", "override-agent")
	
	if cfg.Server != "http://override:3711" {
		t.Errorf("Expected Server 'http://override:3711', got %s", cfg.Server)
	}
	
	if cfg.Token != "override-token" {
		t.Errorf("Expected Token 'override-token', got %s", cfg.Token)
	}
	
	if cfg.AgentID != "override-agent" {
		t.Errorf("Expected AgentID 'override-agent', got %s", cfg.AgentID)
	}
}

func TestApplyClientConfigOverrides_Env(t *testing.T) {
	os.Setenv("CTL_DEVICE_SERVER", "http://env:3711")
	os.Setenv("CTL_DEVICE_TOKEN", "env-token")
	os.Setenv("CTL_DEVICE_AGENT_ID", "env-agent")
	defer func() {
		os.Unsetenv("CTL_DEVICE_SERVER")
		os.Unsetenv("CTL_DEVICE_TOKEN")
		os.Unsetenv("CTL_DEVICE_AGENT_ID")
	}()
	
	cfg := DefaultClientConfig()
	ApplyClientConfigOverrides(cfg, "", "", "")
	
	if cfg.Server != "http://env:3711" {
		t.Errorf("Expected Server 'http://env:3711', got %s", cfg.Server)
	}
	
	if cfg.Token != "env-token" {
		t.Errorf("Expected Token 'env-token', got %s", cfg.Token)
	}
	
	if cfg.AgentID != "env-agent" {
		t.Errorf("Expected AgentID 'env-agent', got %s", cfg.AgentID)
	}
}

func TestApplyClientConfigOverrides_Priority(t *testing.T) {
	os.Setenv("CTL_DEVICE_SERVER", "http://env:3711")
	os.Setenv("CTL_DEVICE_TOKEN", "env-token")
	defer func() {
		os.Unsetenv("CTL_DEVICE_SERVER")
		os.Unsetenv("CTL_DEVICE_TOKEN")
	}()
	
	cfg := DefaultClientConfig()
	ApplyClientConfigOverrides(cfg, "http://cli:3711", "cli-token", "")
	
	if cfg.Server != "http://cli:3711" {
		t.Errorf("Expected CLI override to take priority, got %s", cfg.Server)
	}
	
	if cfg.Token != "cli-token" {
		t.Errorf("Expected CLI override to take priority, got %s", cfg.Token)
	}
}
