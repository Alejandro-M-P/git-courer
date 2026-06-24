// Package cli_test verifies the hook-check CLI adapter.
package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

// TestHookCheckRun_GitCommand verifies a git command is classified and the
// Result is emitted as JSON on stdout.
func TestHookCheckRun_GitCommand(t *testing.T) {
	cmd := HookCheckCommand{}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	err = cmd.Run([]string{"git status"})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var buf bytes.Buffer
	if _, copyErr := buf.ReadFrom(r); copyErr != nil {
		t.Fatalf("read pipe: %v", copyErr)
	}

	var result map[string]string
	if jsonErr := json.Unmarshal(buf.Bytes(), &result); jsonErr != nil {
		t.Fatalf("stdout is not valid JSON: %v\noutput: %q", jsonErr, buf.String())
	}

	if result["Command"] != "git status" {
		t.Errorf("Command: got %q, want %q", result["Command"], "git status")
	}
	if result["Decision"] != "ask" {
		t.Errorf("Decision: got %q, want %q", result["Decision"], "ask")
	}
	if result["MCPTool"] != "status" {
		t.Errorf("MCPTool: got %q, want %q", result["MCPTool"], "status")
	}
	if result["Reason"] == "" {
		t.Error("Reason is empty — expected non-empty reason")
	}
}

// TestHookCheckRun_NonGitCommand verifies a non-git command is classified as
// allow and emitted as JSON on stdout.
func TestHookCheckRun_NonGitCommand(t *testing.T) {
	cmd := HookCheckCommand{}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	err = cmd.Run([]string{"ls -la"})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var buf bytes.Buffer
	if _, copyErr := buf.ReadFrom(r); copyErr != nil {
		t.Fatalf("read pipe: %v", copyErr)
	}

	var result map[string]string
	if jsonErr := json.Unmarshal(buf.Bytes(), &result); jsonErr != nil {
		t.Fatalf("stdout is not valid JSON: %v\noutput: %q", jsonErr, buf.String())
	}

	if result["Command"] != "ls -la" {
		t.Errorf("Command: got %q, want %q", result["Command"], "ls -la")
	}
	if result["Decision"] != "allow" {
		t.Errorf("Decision: got %q, want %q", result["Decision"], "allow")
	}
	if result["MCPTool"] != "" {
		t.Errorf("MCPTool: got %q, want empty", result["MCPTool"])
	}
}

// TestHookCheckRun_NoArgs verifies Run returns an error when no args provided.
func TestHookCheckRun_NoArgs(t *testing.T) {
	cmd := HookCheckCommand{}

	err := cmd.Run([]string{})
	if err == nil {
		t.Fatal("expected error when no args provided, got nil")
	}
}

// TestHookCheckRun_EmptyArg verifies Run returns an error when the single
// argument is empty (nothing to classify).
func TestHookCheckRun_EmptyArg(t *testing.T) {
	cmd := HookCheckCommand{}

	err := cmd.Run([]string{""})
	if err == nil {
		t.Fatal("expected error when arg is empty, got nil")
	}
}