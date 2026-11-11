// Package output provides safe local file output.
package output

import (
	"fmt"
	"os"
	"path/filepath"
)

// File writes content to path, creating parent directories as needed.
func File(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o600)
}

// NewFile writes content to path only when it does not already exist.
func NewFile(path string, content []byte) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("output file %q already exists", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	return File(path, content)
}
