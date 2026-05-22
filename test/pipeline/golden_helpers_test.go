// Package pipeline provides file-based pipeline testing infrastructure for
// the commit workflow. Golden file helpers are shared across build modes.
package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

// goldenPath returns the path to a golden file for a given scenario and filename.
func goldenPath(scenario, filename string) string {
	return filepath.Join("golden", scenario, filename)
}

// writeGolden writes test data to a golden file.
func writeGolden(t *testing.T, scenario, filename string, data []byte) {
	t.Helper()
	path := goldenPath(scenario, filename)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// readGolden reads test data from a golden file.
func readGolden(t *testing.T, scenario, filename string) []byte {
	t.Helper()
	path := goldenPath(scenario, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}