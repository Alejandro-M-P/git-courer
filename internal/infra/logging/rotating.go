// Package logging provides log rotation for git-courer.
package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RotatingLogWriter implements log rotation (max N lines).
type RotatingLogWriter struct {
	path     string
	maxLines int
	lines    []string
}

// NewRotatingLogWriter creates a new rotating log writer.
func NewRotatingLogWriter(path string, maxLines int) (*RotatingLogWriter, error) {
	// Use filepath.Dir to safely extract directory - handles paths with/without slashes
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}
	}

	var lines []string
	if data, err := os.ReadFile(path); err == nil {
		lines = strings.Split(string(data), "\n")
		if len(lines) > maxLines {
			lines = lines[len(lines)-maxLines:]
		}
	}

	return &RotatingLogWriter{
		path:     path,
		maxLines: maxLines,
		lines:    lines,
	}, nil
}

// Write implements io.Writer with log rotation.
func (w *RotatingLogWriter) Write(p []byte) (n int, err error) {
	w.lines = append(w.lines, string(p))
	if len(w.lines) > w.maxLines {
		w.lines = w.lines[len(w.lines)-w.maxLines:]
	}
	if err := os.WriteFile(w.path, []byte(strings.Join(w.lines, "")), 0644); err != nil {
		return 0, err
	}
	return len(p), nil
}
