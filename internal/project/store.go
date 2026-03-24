package project

import (
	"sync"

	"github.com/0xdevelop/ctl_device/pkg/protocol"
)

// Store persists projects and tasks.
type Store struct {
	mu       sync.RWMutex
	projects map[string]*protocol.Project
	tasks    map[string]*protocol.Task
}

// NewStore creates a new in-memory Store.
func NewStore() *Store {
	return &Store{
		projects: make(map[string]*protocol.Project),
		tasks:    make(map[string]*protocol.Task),
	}
}

// SaveProject saves or updates a project (stub).
func (s *Store) SaveProject(p *protocol.Project) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.projects[p.Name] = p
	return nil
}

// GetProject retrieves a project by name (stub).
func (s *Store) GetProject(name string) (*protocol.Project, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.projects[name]
	return p, ok
}

// SaveTask saves or updates a task (stub).
func (s *Store) SaveTask(t *protocol.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[t.ID] = t
	return nil
}

// GetTask retrieves a task by ID (stub).
func (s *Store) GetTask(id string) (*protocol.Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[id]
	return t, ok
}
