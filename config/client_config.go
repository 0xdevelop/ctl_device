package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type ClientConfig struct {
	Server       string   `yaml:"server"`
	Token        string   `yaml:"token"`
	AgentID      string   `yaml:"agent_id"`
	Role         string   `yaml:"role"`
	Capabilities []string `yaml:"capabilities"`
}

func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		Server:       "http://localhost:3711",
		Token:        "",
		AgentID:      "",
		Role:         "executor",
		Capabilities: []string{},
	}
}

func LoadClientConfig(configPath string) (*ClientConfig, error) {
	searchPaths := []string{}

	if configPath != "" {
		searchPaths = append(searchPaths, configPath)
	}

	cwd, err := os.Getwd()
	if err == nil {
		searchPaths = append(searchPaths, filepath.Join(cwd, "client.yaml"))
	}

	homeDir, err := os.UserHomeDir()
	if err == nil {
		searchPaths = append(searchPaths, filepath.Join(homeDir, ".config", "ctl_device", "client.yaml"))
	}

	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return loadClientConfigFromFile(path)
		}
	}

	if len(searchPaths) > 0 {
		return loadClientConfigFromFile(searchPaths[0])
	}

	return DefaultClientConfig(), nil
}

func loadClientConfigFromFile(path string) (*ClientConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultClientConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := DefaultClientConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if config.Server == "" {
		config.Server = "http://localhost:3711"
	}

	if config.Role == "" {
		config.Role = "executor"
	}

	return config, nil
}

func ApplyClientConfigOverrides(config *ClientConfig, serverURL, token, agentID string) {
	if envServer := os.Getenv("CTL_DEVICE_SERVER"); envServer != "" {
		config.Server = envServer
	}
	if envToken := os.Getenv("CTL_DEVICE_TOKEN"); envToken != "" {
		config.Token = envToken
	}
	if envAgentID := os.Getenv("CTL_DEVICE_AGENT_ID"); envAgentID != "" {
		config.AgentID = envAgentID
	}

	if serverURL != "" {
		config.Server = serverURL
	}
	if token != "" {
		config.Token = token
	}
	if agentID != "" {
		config.AgentID = agentID
	}

	if strings.HasPrefix(config.Server, "~/") {
		homeDir, _ := os.UserHomeDir()
		config.Server = filepath.Join(homeDir, config.Server[2:])
	}
}
