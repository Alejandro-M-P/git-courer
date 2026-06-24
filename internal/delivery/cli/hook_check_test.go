// Package cli_test verifies the hook-check CLI adapter.
package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
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

// TestHookCheckRun_NoArgs_NoStdin verifies Run returns an error when no args
// are provided AND Stdin is nil (no stream to read from — programmer error).
// With a real Stdin set, no-args means stdin mode (see TestHookCheck_Stdin_*).
func TestHookCheckRun_NoArgs_NoStdin(t *testing.T) {
	cmd := HookCheckCommand{Stdout: &bytes.Buffer{}}

	err := cmd.Run([]string{})
	if err == nil {
		t.Fatal("expected error when no args and no Stdin provided, got nil")
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

// --- stdin mode (Codex hook protocol) ---

// runStdin invokes HookCheckCommand.Run in stdin mode (no CLI args) with the
// given stdin payload and returns the captured stdout bytes. It uses the
// mockable Stdin/Stdout fields so the test never touches real OS streams.
func runStdin(t *testing.T, stdin string) []byte {
	t.Helper()
	cmd := HookCheckCommand{
		Stdin:  strings.NewReader(stdin),
		Stdout: &bytes.Buffer{},
	}
	if err := cmd.Run(nil); err != nil {
		t.Fatalf("Run(stdin) returned error: %v", err)
	}
	if buf, ok := cmd.Stdout.(*bytes.Buffer); ok {
		return buf.Bytes()
	}
	t.Fatal("Stdout was not a *bytes.Buffer")
	return nil
}

// TestHookCheck_Stdin_GitCommand verifies stdin mode emits a Codex
// permissionDecision deny for a git command, with the MCP tool in the reason.
func TestHookCheck_Stdin_GitCommand(t *testing.T) {
	out := runStdin(t, `{"tool_input":{"command":"git status"}}`)

	var parsed CodexHookOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("stdout not valid CodexHookOutput: %v\noutput: %q", err, out)
	}
	if parsed.HookSpecificOutput.HookEventName != "PreToolUse" {
		t.Errorf("hookEventName: got %q, want %q", parsed.HookSpecificOutput.HookEventName, "PreToolUse")
	}
	if parsed.HookSpecificOutput.PermissionDecision != "deny" {
		t.Errorf("permissionDecision: got %q, want %q", parsed.HookSpecificOutput.PermissionDecision, "deny")
	}
	if !strings.Contains(parsed.HookSpecificOutput.PermissionDecisionReason, "status") {
		t.Errorf("permissionDecisionReason should mention the MCP tool 'status': got %q", parsed.HookSpecificOutput.PermissionDecisionReason)
	}
}

// TestHookCheck_Stdin_NonGitCommand verifies stdin mode emits allow for a
// non-git command.
func TestHookCheck_Stdin_NonGitCommand(t *testing.T) {
	out := runStdin(t, `{"tool_input":{"command":"ls -la"}}`)

	var parsed CodexHookOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("stdout not valid CodexHookOutput: %v\noutput: %q", err, out)
	}
	if parsed.HookSpecificOutput.PermissionDecision != "allow" {
		t.Errorf("permissionDecision: got %q, want %q", parsed.HookSpecificOutput.PermissionDecision, "allow")
	}
}

// TestHookCheck_Stdin_MalformedJSON verifies stdin mode falls back to allow
// when the JSON cannot be parsed (safe fallback per spec).
func TestHookCheck_Stdin_MalformedJSON(t *testing.T) {
	out := runStdin(t, "garbage-not-json")

	var parsed CodexHookOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("malformed stdin should still emit valid CodexHookOutput: %v\noutput: %q", err, out)
	}
	if parsed.HookSpecificOutput.PermissionDecision != "allow" {
		t.Errorf("permissionDecision on parse error: got %q, want %q (safe fallback)", parsed.HookSpecificOutput.PermissionDecision, "allow")
	}
}

// TestHookCheck_Stdin_Empty verifies stdin mode emits allow when stdin is
// empty (nothing to classify → safe fallback).
func TestHookCheck_Stdin_Empty(t *testing.T) {
	out := runStdin(t, "")

	var parsed CodexHookOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("empty stdin should still emit valid CodexHookOutput: %v\noutput: %q", err, out)
	}
	if parsed.HookSpecificOutput.PermissionDecision != "allow" {
		t.Errorf("permissionDecision on empty stdin: got %q, want %q", parsed.HookSpecificOutput.PermissionDecision, "allow")
	}
}

// TestHookCheck_CLIArgs_Unchanged verifies that when args are provided, Run
// still emits the existing Result JSON (CLI mode preserved).
func TestHookCheck_CLIArgs_Unchanged(t *testing.T) {
	var buf bytes.Buffer
	cmd := HookCheckCommand{
		Stdout: &buf,
	}
	if err := cmd.Run([]string{"git status"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var result map[string]string
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("CLI mode stdout not valid JSON: %v\noutput: %q", err, buf.String())
	}
	if result["Decision"] != "ask" {
		t.Errorf("CLI mode Decision: got %q, want %q", result["Decision"], "ask")
	}
	if result["MCPTool"] != "status" {
		t.Errorf("CLI mode MCPTool: got %q, want %q", result["MCPTool"], "status")
	}
}