/*
Package config 配置文件
*/
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ProjectName        = "ctl_device"
	ProjectVersion     = "v0.0.6"
	ProjectDescription = "ctl_device"
)

// ClientSection holds client-mode agent settings within the unified config.
type ClientSection struct {
	AgentID      string   `yaml:"agent_id"`
	Role         string   `yaml:"role"`
	Capabilities []string `yaml:"capabilities"`
}

// UnifiedConfig is the single config structure for both full and client modes.
type UnifiedConfig struct {
	Mode     string          `yaml:"mode"`
	Connect  string          `yaml:"connect"`
	Server   ServerSection   `yaml:"server"`
	Client   ClientSection   `yaml:"client"`
	Notify   NotifySection   `yaml:"notify"`
	Projects []ProjectConfig `yaml:"projects"`
}

// DefaultConfig returns a UnifiedConfig with all default values.
func DefaultConfig() *UnifiedConfig {
	homeDir, _ := os.UserHomeDir()
	stateDir := filepath.Join(homeDir, ".config", "ctl_device")

	return &UnifiedConfig{
		Mode:    "full",
		Connect: "",
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
		Client: ClientSection{
			AgentID:      "",
			Role:         "executor",
			Capabilities: []string{},
		},
		Notify: NotifySection{
			Channel:    "none",
			Target:     "",
			WebhookURL: "",
		},
		Projects: []ProjectConfig{},
	}
}

// LoadConfig loads a UnifiedConfig from the given YAML path.
// If connect is set, mode is automatically "client".
func LoadConfig(path string) (*UnifiedConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if cfg.Connect != "" {
		cfg.Mode = "client"
	}

	if cfg.Server.StateDir == "" {
		homeDir, _ := os.UserHomeDir()
		cfg.Server.StateDir = filepath.Join(homeDir, ".config", "ctl_device")
	} else if strings.HasPrefix(cfg.Server.StateDir, "~/") {
		homeDir, _ := os.UserHomeDir()
		cfg.Server.StateDir = filepath.Join(homeDir, cfg.Server.StateDir[2:])
	}

	for i := range cfg.Projects {
		proj := &cfg.Projects[i]
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

	return cfg, nil
}
