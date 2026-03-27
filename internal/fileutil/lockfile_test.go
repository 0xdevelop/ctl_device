package fileutil

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestAcquireLock_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.lock")

	if err := AcquireLock(path); err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	defer ReleaseLock(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatalf("expected PID in lock file, got %q", data)
	}
	if pid != os.Getpid() {
		t.Errorf("expected PID %d, got %d", os.Getpid(), pid)
	}
}

func TestAcquireLock_StaleLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.lock")

	// Write a stale PID (99999 is almost certainly not a running process).
	// Use -1 which is never a valid PID.
	_ = os.WriteFile(path, []byte("-1"), 0o644)

	if err := AcquireLock(path); err != nil {
		t.Fatalf("expected stale lock to be overwritten, got: %v", err)
	}
	defer ReleaseLock(path)

	data, _ := os.ReadFile(path)
	pid, _ := strconv.Atoi(string(data))
	if pid != os.Getpid() {
		t.Errorf("expected own PID after stale lock removal, got %d", pid)
	}
}

func TestAcquireLock_ActiveLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.lock")

	// Write the current process's own PID — this process is definitely alive.
	_ = os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o644)

	err := AcquireLock(path)
	if err == nil {
		ReleaseLock(path)
		t.Fatal("expected error when active lock exists, got nil")
	}
}

func TestReleaseLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.lock")

	if err := AcquireLock(path); err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}

	ReleaseLock(path)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected lock file to be removed after ReleaseLock")
	}
}
