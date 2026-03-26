package config

import (
	"os"
	"path/filepath"
	"testing"
)

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
		t.Errorf("Expected TLS Enabled false, got %v", cfg.Server.TLS.Enabled)
	}
	if cfg.Server.SnapshotIntervalSecs != 30 {
		t.Errorf("Expected SnapshotIntervalSecs 30, got %d", cfg.Server.SnapshotIntervalSecs)
	}
	if cfg.Server.HeartbeatTimeoutSecs != 45 {
		t.Errorf("Expected HeartbeatTimeoutSecs 45, got %d", cfg.Server.HeartbeatTimeoutSecs)
	}
	if cfg.Notify.Channel != "none" {
		t.Errorf("Expected Notify Channel 'none', got %s", cfg.Notify.Channel)
	}
}

func TestLoadServerConfig_Complete(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.yaml")

	yamlContent := `
server:
  mcp_port: 3720
  jsonrpc_port: 3721
  dashboard_port: 3722
  bind: "127.0.0.1"
  token: "testtoken"
  tls:
    enabled: true
    cert_file: "/path/to/cert"
    key_file: "/path/to/key"
    auto_tls: false
    domain: "example.com"
  state_dir: "` + tmpDir + `"
  snapshot_interval_seconds: 60
  heartbeat_timeout_seconds: 90

notify:
  channel: "webhook"
  target: "test"
  webhook_url: "http://example.com/webhook"

projects:
  - name: test-project
    dir: ` + tmpDir + `
    tech: go
    test_cmd: "go test ./..."
    executor: "any"
    timeout_minutes: 180
`

	if err := os.WriteFile(configFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadServerConfig(configFile)
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
	if cfg.Server.Token != "testtoken" {
		t.Errorf("Expected Token 'testtoken', got %s", cfg.Server.Token)
	}
	if cfg.Server.TLS.Enabled != true {
		t.Errorf("Expected TLS Enabled true, got %v", cfg.Server.TLS.Enabled)
	}
	if cfg.Server.TLS.CertFile != "/path/to/cert" {
		t.Errorf("Expected CertFile '/path/to/cert', got %s", cfg.Server.TLS.CertFile)
	}
	if cfg.Server.SnapshotIntervalSecs != 60 {
		t.Errorf("Expected SnapshotIntervalSecs 60, got %d", cfg.Server.SnapshotIntervalSecs)
	}
	if cfg.Notify.Channel != "webhook" {
		t.Errorf("Expected Notify Channel 'webhook', got %s", cfg.Notify.Channel)
	}
	if len(cfg.Projects) != 1 {
		t.Errorf("Expected 1 project, got %d", len(cfg.Projects))
	}
	if cfg.Projects[0].Name != "test-project" {
		t.Errorf("Expected project name 'test-project', got %s", cfg.Projects[0].Name)
	}
	if cfg.Projects[0].TimeoutMinutes != 180 {
		t.Errorf("Expected TimeoutMinutes 180, got %d", cfg.Projects[0].TimeoutMinutes)
	}
}

func TestLoadServerConfig_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.yaml")

	yamlContent := `
server:
  state_dir: "` + tmpDir + `"
projects:
  - name: minimal-project
    dir: ` + tmpDir + `
`

	if err := os.WriteFile(configFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadServerConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Server.MCPPort != 3710 {
		t.Errorf("Expected default MCPPort 3710, got %d", cfg.Server.MCPPort)
	}
	if cfg.Server.Bind != "0.0.0.0" {
		t.Errorf("Expected default Bind '0.0.0.0', got %s", cfg.Server.Bind)
	}
	if cfg.Projects[0].TimeoutMinutes != 120 {
		t.Errorf("Expected default TimeoutMinutes 120, got %d", cfg.Projects[0].TimeoutMinutes)
	}
	if cfg.Projects[0].Executor != "any" {
		t.Errorf("Expected default Executor 'any', got %s", cfg.Projects[0].Executor)
	}
}

func TestLoadServerConfig_TildeExpansion(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.yaml")

	homeDir, _ := os.UserHomeDir()
	yamlContent := `
server:
  state_dir: "~/.config/ctl_device"
projects:
  - name: tilde-project
    dir: "` + homeDir + `/test"
`

	if err := os.WriteFile(configFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadServerConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Server.StateDir != filepath.Join(homeDir, ".config/ctl_device") {
		t.Errorf("Expected StateDir with tilde expanded, got %s", cfg.Server.StateDir)
	}
}

func TestLoadServerConfig_MissingProjectDir(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.yaml")

	yamlContent := `
server:
  state_dir: "` + tmpDir + `"
projects:
  - name: no-dir-project
    tech: go
`

	if err := os.WriteFile(configFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := LoadServerConfig(configFile)
	if err == nil {
		t.Error("Expected error for missing project dir, got nil")
	}
}

func TestLoadClientConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "client.yaml")

	yamlContent := `
server: "http://192.168.1.100:3711"
token: "clienttoken"
agent_id: "test-agent"
role: "executor"
capabilities: ["go", "python"]
`

	if err := os.WriteFile(configFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := loadClientConfigFromFile(configFile)
	if err != nil {
		t.Fatalf("Failed to load client config: %v", err)
	}

	if cfg.Server != "http://192.168.1.100:3711" {
		t.Errorf("Expected Server 'http://192.168.1.100:3711', got %s", cfg.Server)
	}
	if cfg.Token != "clienttoken" {
		t.Errorf("Expected Token 'clienttoken', got %s", cfg.Token)
	}
	if cfg.AgentID != "test-agent" {
		t.Errorf("Expected AgentID 'test-agent', got %s", cfg.AgentID)
	}
	if cfg.Role != "executor" {
		t.Errorf("Expected Role 'executor', got %s", cfg.Role)
	}
	if len(cfg.Capabilities) != 2 {
		t.Errorf("Expected 2 capabilities, got %d", len(cfg.Capabilities))
	}
}

func TestApplyClientConfigOverrides_Flags(t *testing.T) {
	cfg := DefaultClientConfig()
	ApplyClientConfigOverrides(cfg, "http://override:3711", "overridetoken", "override-agent")

	if cfg.Server != "http://override:3711" {
		t.Errorf("Expected Server overridden, got %s", cfg.Server)
	}
	if cfg.Token != "overridetoken" {
		t.Errorf("Expected Token overridden, got %s", cfg.Token)
	}
	if cfg.AgentID != "override-agent" {
		t.Errorf("Expected AgentID overridden, got %s", cfg.AgentID)
	}
}

func TestApplyClientConfigOverrides_EnvVars(t *testing.T) {
	os.Setenv("CTL_DEVICE_SERVER", "http://env:3711")
	os.Setenv("CTL_DEVICE_TOKEN", "envtoken")
	os.Setenv("CTL_DEVICE_AGENT_ID", "env-agent")
	defer func() {
		os.Unsetenv("CTL_DEVICE_SERVER")
		os.Unsetenv("CTL_DEVICE_TOKEN")
		os.Unsetenv("CTL_DEVICE_AGENT_ID")
	}()

	cfg := DefaultClientConfig()
	ApplyClientConfigOverrides(cfg, "", "", "")

	if cfg.Server != "http://env:3711" {
		t.Errorf("Expected Server from env, got %s", cfg.Server)
	}
	if cfg.Token != "envtoken" {
		t.Errorf("Expected Token from env, got %s", cfg.Token)
	}
	if cfg.AgentID != "env-agent" {
		t.Errorf("Expected AgentID from env, got %s", cfg.AgentID)
	}
}

func TestApplyClientConfigOverrides_FlagPriority(t *testing.T) {
	os.Setenv("CTL_DEVICE_SERVER", "http://env:3711")
	os.Setenv("CTL_DEVICE_TOKEN", "envtoken")
	defer func() {
		os.Unsetenv("CTL_DEVICE_SERVER")
		os.Unsetenv("CTL_DEVICE_TOKEN")
	}()

	cfg := DefaultClientConfig()
	ApplyClientConfigOverrides(cfg, "http://flag:3711", "flagtoken", "")

	if cfg.Server != "http://flag:3711" {
		t.Errorf("Expected Server from flag (higher priority), got %s", cfg.Server)
	}
	if cfg.Token != "flagtoken" {
		t.Errorf("Expected Token from flag (higher priority), got %s", cfg.Token)
	}
}
