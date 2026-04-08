// Package logging provides log rotation for git-courer.
package logging

import (
	"os"
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
	os.MkdirAll(path[:strings.LastIndex(path, "/")], 0755)

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
	os.WriteFile(w.path, []byte(strings.Join(w.lines, "")), 0644)
	return len(p), nil
}
