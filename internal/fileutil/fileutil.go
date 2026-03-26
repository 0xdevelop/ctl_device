// Package fileutil provides shared file I/O helpers used across internal packages.
package fileutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// AtomicWrite marshals v to JSON and writes it atomically via a tmp→rename,
// so a crash mid-write never leaves a corrupt file.
func AtomicWrite(path string, v interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	return os.Rename(tmp, path)
}

// ReadJSON reads and unmarshals JSON from path into v.
// Returns nil, nil if the file does not exist.
func ReadJSON(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", path, err)
	}
	return json.Unmarshal(data, v)
}
