package project

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/0xdevelop/ctl_device/internal/event"
	"github.com/0xdevelop/ctl_device/pkg/protocol"
)

func newTestStore(t *testing.T) *FileStore {
	t.Helper()
	fs, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	return fs
}

func TestSaveLoadProject(t *testing.T) {
	fs := newTestStore(t)
	p := &protocol.Project{
		Name:           "alpha",
		Dir:            "/tmp/alpha",
		Tech:           "go",
		TimeoutMinutes: 30,
	}
	if err := fs.SaveProject(p); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	got, err := fs.LoadProject("alpha")
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if got == nil {
		t.Fatal("LoadProject returned nil")
	}
	if got.Name != p.Name || got.Dir != p.Dir || got.Tech != p.Tech || got.TimeoutMinutes != p.TimeoutMinutes {
		t.Errorf("mismatch: got %+v want %+v", got, p)
	}

	// Loading a non-existent project returns nil without error.
	missing, err := fs.LoadProject("nonexistent")
	if err != nil {
		t.Fatalf("LoadProject missing: %v", err)
	}
	if missing != nil {
		t.Errorf("expected nil for missing project, got %+v", missing)
	}
}

func TestAtomicWrite(t *testing.T) {
	fs := newTestStore(t)
	if err := fs.SaveProject(&protocol.Project{Name: "proj"}); err != nil {
		t.Fatal(err)
	}

	const n = 5
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			task := &protocol.Task{
				ID:      fmt.Sprintf("t%d", i),
				Project: "proj",
				Num:     fmt.Sprintf("%d", i),
				Name:    fmt.Sprintf("task-%d", i),
				Status:  protocol.TaskPending,
			}
			if err := fs.SaveTask(task); err != nil {
				t.Errorf("SaveTask %d: %v", i, err)
			}
		}()
	}
	wg.Wait()

	tasks, err := fs.ListTasks("proj")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != n {
		t.Errorf("expected %d tasks, got %d", n, len(tasks))
	}
}

func TestSnapshotRoundtrip(t *testing.T) {
	fs := newTestStore(t)

	snap := &Snapshot{
		Version: "1",
		SavedAt: time.Now().Round(time.Second),
		Projects: []*protocol.Project{
			{Name: "p1", Dir: "/p1", Tech: "go"},
		},
		Tasks: []*protocol.Task{
			{ID: "t1", Project: "p1", Num: "1", Name: "task1", Status: protocol.TaskPending},
		},
	}
	if err := fs.SaveSnapshot(snap); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	got, err := fs.LoadSnapshot()
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if got == nil {
		t.Fatal("LoadSnapshot returned nil")
	}
	if got.Version != snap.Version {
		t.Errorf("version: got %q want %q", got.Version, snap.Version)
	}
	if len(got.Projects) != 1 || got.Projects[0].Name != "p1" {
		t.Errorf("projects mismatch: %+v", got.Projects)
	}
	if len(got.Tasks) != 1 || got.Tasks[0].Num != "1" {
		t.Errorf("tasks mismatch: %+v", got.Tasks)
	}
	if !got.SavedAt.Equal(snap.SavedAt) {
		t.Errorf("saved_at: got %v want %v", got.SavedAt, snap.SavedAt)
	}

	// Loading when file is absent returns nil without error.
	fs2 := newTestStore(t)
	absent, err := fs2.LoadSnapshot()
	if err != nil {
		t.Fatalf("LoadSnapshot absent: %v", err)
	}
	if absent != nil {
		t.Errorf("expected nil for absent snapshot, got %+v", absent)
	}
}

func TestSchedulerCompleteTask(t *testing.T) {
	fs := newTestStore(t)
	bus := event.NewBus()
	sched := NewScheduler(fs, bus)

	if err := fs.SaveProject(&protocol.Project{Name: "myproject", TimeoutMinutes: 30}); err != nil {
		t.Fatal(err)
	}

	// Dispatch task
	task := &protocol.Task{
		ID:   "task-01",
		Num:  "01",
		Name: "do something",
	}
	if err := sched.Dispatch("myproject", task); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	// Verify saved as pending
	loaded, err := fs.LoadTask("myproject", "01")
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil {
		t.Fatal("task not found after dispatch")
	}
	if loaded.Status != protocol.TaskPending {
		t.Errorf("expected pending after dispatch, got %s", loaded.Status)
	}

	// Update to executing
	if err := sched.UpdateTaskStatus("myproject", "01", protocol.TaskExecuting); err != nil {
		t.Fatalf("UpdateTaskStatus: %v", err)
	}
	loaded, err = fs.LoadTask("myproject", "01")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status != protocol.TaskExecuting {
		t.Errorf("expected executing, got %s", loaded.Status)
	}
	if loaded.StartedAt.IsZero() {
		t.Error("StartedAt should be set when status becomes executing")
	}

	// Complete task
	if err := sched.CompleteTask("myproject", "01", "abc123", "all good"); err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}
	loaded, err = fs.LoadTask("myproject", "01")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status != protocol.TaskCompleted {
		t.Errorf("expected completed, got %s", loaded.Status)
	}
	if loaded.Commit != "abc123" {
		t.Errorf("expected commit abc123, got %s", loaded.Commit)
	}
	if loaded.Report != "all good" {
		t.Errorf("expected report 'all good', got %s", loaded.Report)
	}
}
