package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRun(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "report-cli-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a dummy jsonl file so it's not empty
	jsonlContent := `{"timestamp":"2026-05-01T21:33:45Z","operation":"GenerateCommitMessage","model":"llama3","latency":1000000000,"prompt":"test","response":"feat: test","success":true}` + "\n"
	telemetryDir := filepath.Join(tempDir, ".gcourer", "telemetry")
	err = os.MkdirAll(telemetryDir, 0755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(telemetryDir, "test.jsonl"), []byte(jsonlContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("default directory", func(t *testing.T) {
		// Change working directory to tempDir
		oldWd, _ := os.Getwd()
		os.Chdir(tempDir)
		defer os.Chdir(oldWd)

		err := run([]string{})
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("explicit directory", func(t *testing.T) {
		err := run([]string{telemetryDir})
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("missing directory returns error", func(t *testing.T) {
		err := run([]string{"/non/existent/dir"})
		if err == nil {
			t.Error("expected error for non-existent directory, got nil")
		}
	})
}
