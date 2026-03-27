package fileutil

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// AcquireLock creates a lock file at path containing the current PID.
// Returns an error if another live process already holds the lock.
// Call ReleaseLock on graceful shutdown.
func AcquireLock(path string) error {
	// Check if an existing lock is alive.
	if data, err := os.ReadFile(path); err == nil {
		pidStr := strings.TrimSpace(string(data))
		if pid, parseErr := strconv.Atoi(pidStr); parseErr == nil {
			if isProcessAlive(pid) {
				return fmt.Errorf("another full instance is already running (PID: %d). Use --connect to join as client", pid)
			}
			// Stale lock — remove it.
			_ = os.Remove(path)
		}
	}

	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o644)
}

// ReleaseLock removes the lock file.
func ReleaseLock(path string) {
	_ = os.Remove(path)
}

// isProcessAlive returns true if the given PID is a running process.
func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 checks existence without sending a real signal.
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
