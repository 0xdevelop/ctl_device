package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
	AutoTLS  bool   `yaml:"auto_tls"`
	Domain   string `yaml:"domain"`
}

type ServerSection struct {
	MCPPort               int       `yaml:"mcp_port"`
	JSONRPCPort           int       `yaml:"jsonrpc_port"`
	DashboardPort         int       `yaml:"dashboard_port"`
	GRPCPort              int       `yaml:"grpc_port"`
	Bind                  string    `yaml:"bind"`
	Token                 string    `yaml:"token"`
	TLS                   TLSConfig `yaml:"tls"`
	StateDir              string    `yaml:"state_dir"`
	SnapshotIntervalSecs  int       `yaml:"snapshot_interval_seconds"`
	HeartbeatTimeoutSecs  int       `yaml:"heartbeat_timeout_seconds"`
}

type NotifySection struct {
	Channel    string `yaml:"channel"`
	Target     string `yaml:"target"`
	WebhookURL string `yaml:"webhook_url"`
}

type ProjectConfig struct {
	Name           string `yaml:"name"`
	Dir            string `yaml:"dir"`
	Tech           string `yaml:"tech"`
	TestCmd        string `yaml:"test_cmd"`
	Executor       string `yaml:"executor"`
	TimeoutMinutes int    `yaml:"timeout_minutes"`
	NotifyChannel  string `yaml:"notify_channel"`
	NotifyTarget   string `yaml:"notify_target"`
}

type ServerConfig struct {
	Server   ServerSection   `yaml:"server"`
	Notify   NotifySection   `yaml:"notify"`
	Projects []ProjectConfig `yaml:"projects"`
}

func DefaultServerConfig() *ServerConfig {
	homeDir, _ := os.UserHomeDir()
	stateDir := filepath.Join(homeDir, ".config", "ctl_device")

	return &ServerConfig{
		Server: ServerSection{
			MCPPort:              3710,
			JSONRPCPort:          3711,
			DashboardPort:        3712,
			GRPCPort:             3713,
			Bind:                 "0.0.0.0",
			Token:                "",
			StateDir:             stateDir,
			SnapshotIntervalSecs: 30,
			HeartbeatTimeoutSecs: 45,
			TLS: TLSConfig{
				Enabled:  false,
				CertFile: "",
				KeyFile:  "",
				AutoTLS:  false,
				Domain:   "",
			},
		},
		Notify: NotifySection{
			Channel:    "none",
			Target:     "",
			WebhookURL: "",
		},
		Projects: []ProjectConfig{},
	}
}

func LoadServerConfig(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := DefaultServerConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if config.Server.StateDir == "" {
		homeDir, _ := os.UserHomeDir()
		config.Server.StateDir = filepath.Join(homeDir, ".config", "ctl_device")
	}

	if config.Server.StateDir == "~/.config/ctl_device" || strings.HasPrefix(config.Server.StateDir, "~/") {
		homeDir, _ := os.UserHomeDir()
		config.Server.StateDir = filepath.Join(homeDir, config.Server.StateDir[2:])
	}

	for i := range config.Projects {
	 proj := &config.Projects[i]
		if proj.Dir == "" {
	  return nil, fmt.Errorf("project %q: dir is required", proj.Name)
	 }

		if strings.HasPrefix(proj.Dir, "~/") {
	  homeDir, _ := os.UserHomeDir()
	  proj.Dir = filepath.Join(homeDir, proj.Dir[2:])
	 }

	 if proj.TimeoutMinutes == 0 {
	  proj.TimeoutMinutes = 120
	 }

	 if proj.Executor == "" {
	  proj.Executor = "any"
	 }
	}

	return config, nil
}



// ExpandTilde expands a leading "~/" in path to the user's home directory.
func ExpandTilde(path string) string {
	if len(path) >= 2 && path[:2] == "~/" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(homeDir, path[2:])
	}
	return path
}

// WriteDefaultConfig writes the default unified config to path as YAML.
// Creates parent directories as needed.
func WriteDefaultConfig(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	cfg := DefaultConfig()
	// Use human-friendly tilde path in the generated file
	cfg.Server.StateDir = "~/.config/ctl_device"
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	header := []byte("# ctl_device configuration\n# mode: full (default) | client\n# When connect is set, mode is automatically \"client\"\n# Auto-generated. Edit as needed.\n# grpc_port: 3713\n\n")
	return os.WriteFile(path, append(header, data...), 0644)
}
