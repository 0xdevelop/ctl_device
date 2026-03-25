package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/0xdevelop/ctl_device/pkg/protocol"
)

// Registry persists agent configurations to disk.
// Online status is managed in-memory by Manager.
type Registry struct {
	dir string
	mu  sync.RWMutex
}

// NewRegistry creates a new Registry.
// If dir is empty the path is resolved from CTL_DEVICE_STATE_DIR or $HOME/.config/ctl_device.
func NewRegistry(dir string) (*Registry, error) {
	if dir == "" {
		if d := os.Getenv("CTL_DEVICE_STATE_DIR"); d != "" {
			dir = d
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			dir = filepath.Join(home, ".config", "ctl_device", "agents")
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Registry{dir: dir}, nil
}

// Save saves an agent configuration to disk.
func (r *Registry) Save(agent *protocol.Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return atomicWrite(filepath.Join(r.dir, agent.ID+".json"), agent)
}

// Load loads an agent configuration from disk.
func (r *Registry) Load(agentID string) (*protocol.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var a protocol.Agent
	if err := readJSON(filepath.Join(r.dir, agentID+".json"), &a); err != nil {
		return nil, err
	}
	if a.ID == "" {
		return nil, nil
	}
	return &a, nil
}

// LoadAll loads all agent configurations from disk.
func (r *Registry) LoadAll() ([]*protocol.Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []*protocol.Agent
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		var a protocol.Agent
		if err := readJSON(filepath.Join(r.dir, e.Name()), &a); err != nil {
			return nil, err
		}
		if a.ID != "" {
			out = append(out, &a)
		}
	}
	return out, nil
}

// Delete removes an agent configuration from disk.
func (r *Registry) Delete(agentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	path := filepath.Join(r.dir, agentID+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// atomicWrite marshals v to JSON and writes it atomically via a .tmp rename.
func atomicWrite(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// readJSON reads and unmarshals JSON from path.
// If the file does not exist it returns nil error and leaves v unchanged.
func readJSON(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, v)
}
