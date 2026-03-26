package project

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/0xdevelop/ctl_device/internal/fileutil"
	"github.com/0xdevelop/ctl_device/pkg/protocol"
)

// Snapshot is the global state snapshot written to state.json.
type Snapshot struct {
	Version  string              `json:"version"`
	SavedAt  time.Time           `json:"saved_at"`
	Projects []*protocol.Project `json:"projects"`
	Tasks    []*protocol.Task    `json:"tasks"`
}

// FileStore persists projects and tasks to the filesystem.
//
// Directory layout:
//
//	dir/
//	  projects/{name}.json
//	  tasks/{project}/{num}.json
//	  state.json
type FileStore struct {
	dir string
	mu  sync.RWMutex
}

// NewFileStore creates a FileStore.
// If dir is empty the path is resolved from CTL_DEVICE_STATE_DIR or $HOME/.config/ctl_device.
func NewFileStore(dir string) (*FileStore, error) {
	if dir == "" {
		if d := os.Getenv("CTL_DEVICE_STATE_DIR"); d != "" {
			dir = d
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			dir = filepath.Join(home, ".config", "ctl_device")
		}
	}
	for _, sub := range []string{"projects", "tasks"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return nil, err
		}
	}
	return &FileStore{dir: dir}, nil
}

// Dir returns the state directory.
func (fs *FileStore) Dir() string {
	return fs.dir
}

// --- helpers ---



// --- Project ---

func (fs *FileStore) SaveProject(p *protocol.Project) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fileutil.AtomicWrite(filepath.Join(fs.dir, "projects", p.Name+".json"), p)
}

func (fs *FileStore) LoadProject(name string) (*protocol.Project, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	var p protocol.Project
	if err := fileutil.ReadJSON(filepath.Join(fs.dir, "projects", name+".json"), &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, nil // file not found
	}
	return &p, nil
}

func (fs *FileStore) ListProjects() ([]*protocol.Project, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	entries, err := os.ReadDir(filepath.Join(fs.dir, "projects"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []*protocol.Project
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		var p protocol.Project
		if err := fileutil.ReadJSON(filepath.Join(fs.dir, "projects", e.Name()), &p); err != nil {
			return nil, err
		}
		if p.Name != "" {
			out = append(out, &p)
		}
	}
	return out, nil
}

func (fs *FileStore) DeleteProject(name string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	path := filepath.Join(fs.dir, "projects", name+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// --- Task ---

func (fs *FileStore) SaveTask(t *protocol.Task) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fileutil.AtomicWrite(filepath.Join(fs.dir, "tasks", t.Project, t.Num+".json"), t)
}

func (fs *FileStore) LoadTask(projectName, taskNum string) (*protocol.Task, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	var t protocol.Task
	if err := fileutil.ReadJSON(filepath.Join(fs.dir, "tasks", projectName, taskNum+".json"), &t); err != nil {
		return nil, err
	}
	if t.Num == "" {
		return nil, nil // file not found
	}
	return &t, nil
}

func (fs *FileStore) ListTasks(projectName string) ([]*protocol.Task, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	dir := filepath.Join(fs.dir, "tasks", projectName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []*protocol.Task
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		var t protocol.Task
		if err := fileutil.ReadJSON(filepath.Join(dir, e.Name()), &t); err != nil {
			return nil, err
		}
		if t.Num != "" {
			out = append(out, &t)
		}
	}
	return out, nil
}

func (fs *FileStore) DeleteTask(projectName, taskNum string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	path := filepath.Join(fs.dir, "tasks", projectName, taskNum+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// --- Snapshot ---

func (fs *FileStore) SaveSnapshot(s *Snapshot) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fileutil.AtomicWrite(filepath.Join(fs.dir, "state.json"), s)
}

func (fs *FileStore) LoadSnapshot() (*Snapshot, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	var s Snapshot
	if err := fileutil.ReadJSON(filepath.Join(fs.dir, "state.json"), &s); err != nil {
		return nil, err
	}
	if s.Version == "" {
		return nil, nil // file not found
	}
	return &s, nil
}
