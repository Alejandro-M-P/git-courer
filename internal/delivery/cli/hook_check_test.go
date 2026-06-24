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

	var buf bytes.Buffer
	cmd.Stdout = &buf

	err := cmd.Run([]string{"git status"})

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
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

	var buf bytes.Buffer
	cmd.Stdout = &buf

	err := cmd.Run([]string{"ls -la"})

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
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

	var buf bytes.Buffer
	cmd.Stdout = &buf

	err := cmd.Run([]string{})
	if err == nil {
		t.Fatal("expected error when no args provided, got nil")
	}
}

// TestHookCheckRun_EmptyArg verifies Run returns an error when the single
// argument is empty (nothing to classify).
func TestHookCheckRun_EmptyArg(t *testing.T) {
	cmd := HookCheckCommand{}

	var buf bytes.Buffer
	cmd.Stdout = &buf

	err := cmd.Run([]string{""})
	if err == nil {
		t.Fatal("expected error when arg is empty, got nil")
	}
}

// TestHookCheckStdin_GitCommand verifies stdin mode emits additionalContext
// suggesting the MCP tool for git commands.
func TestHookCheckStdin_GitCommand(t *testing.T) {
	input := `{"event":{"input":{"command":"git status"}}}`
	var stdinBuf bytes.Buffer
	stdinBuf.WriteString(input)

	var stdoutBuf bytes.Buffer
	cmd := HookCheckCommand{
		Stdin:  &stdinBuf,
		Stdout: &stdoutBuf,
	}

	err := cmd.Run([]string{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if stdoutBuf.Len() == 0 {
		t.Fatal("expected output for git command, got empty")
	}

	var output codexHookOutput
	if jsonErr := json.Unmarshal(stdoutBuf.Bytes(), &output); jsonErr != nil {
		t.Fatalf("stdout is not valid JSON: %v\noutput: %q", jsonErr, stdoutBuf.String())
	}

	if output.HookSpecificOutput.HookEventName != "PreToolUse" {
		t.Errorf("HookEventName: got %q, want %q", output.HookSpecificOutput.HookEventName, "PreToolUse")
	}
	if !strings.Contains(output.HookSpecificOutput.AdditionalContext, "git-courer/status") {
		t.Errorf("AdditionalContext missing MCP tool suggestion: %q", output.HookSpecificOutput.AdditionalContext)
	}
	if output.HookSpecificOutput.PermissionDecision != "" {
		t.Errorf("PermissionDecision should be empty (suggest, not deny), got %q", output.HookSpecificOutput.PermissionDecision)
	}
}

// TestHookCheckStdin_NonGitCommand verifies stdin mode produces no output
// for non-git commands.
func TestHookCheckStdin_NonGitCommand(t *testing.T) {
	input := `{"event":{"input":{"command":"ls -la"}}}`
	var stdinBuf bytes.Buffer
	stdinBuf.WriteString(input)

	var stdoutBuf bytes.Buffer
	cmd := HookCheckCommand{
		Stdin:  &stdinBuf,
		Stdout: &stdoutBuf,
	}

	err := cmd.Run([]string{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if stdoutBuf.Len() > 0 {
		t.Errorf("expected no output for non-git command, got: %s", stdoutBuf.String())
	}
}

// TestHookCheckStdin_InvalidJSON verifies stdin mode handles invalid JSON
// gracefully (safe fallback, no error).
func TestHookCheckStdin_InvalidJSON(t *testing.T) {
	var stdinBuf bytes.Buffer
	stdinBuf.WriteString("not json")

	var stdoutBuf bytes.Buffer
	cmd := HookCheckCommand{
		Stdin:  &stdinBuf,
		Stdout: &stdoutBuf,
	}

	err := cmd.Run([]string{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if stdoutBuf.Len() > 0 {
		t.Errorf("expected no output for invalid JSON, got: %s", stdoutBuf.String())
	}
}

// TestSessionStartHookCommand_ReturnsGoldenRules verifies session-start-hook
// returns golden rules as additionalContext.
func TestSessionStartHookCommand_ReturnsGoldenRules(t *testing.T) {
	var stdinBuf bytes.Buffer
	stdinBuf.WriteString(`{"event":{"input":{"command":"startup"}}}`)

	var stdoutBuf bytes.Buffer
	cmd := SessionStartHookCommand{
		Stdin:  &stdinBuf,
		Stdout: &stdoutBuf,
	}

	err := cmd.Run([]string{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var output codexHookOutput
	if jsonErr := json.Unmarshal(stdoutBuf.Bytes(), &output); jsonErr != nil {
		t.Fatalf("stdout is not valid JSON: %v\noutput: %q", jsonErr, stdoutBuf.String())
	}

	if output.HookSpecificOutput.HookEventName != "SessionStart" {
		t.Errorf("HookEventName: got %q, want %q", output.HookSpecificOutput.HookEventName, "SessionStart")
	}
	if !strings.Contains(output.HookSpecificOutput.AdditionalContext, "Golden Rules") {
		t.Errorf("AdditionalContext missing golden rules: %q", output.HookSpecificOutput.AdditionalContext)
	}
}

// TestSubagentStartHookCommand_ReturnsGoldenRules verifies subagent-start-hook
// returns golden rules as additionalContext.
func TestSubagentStartHookCommand_ReturnsGoldenRules(t *testing.T) {
	var stdinBuf bytes.Buffer
	stdinBuf.WriteString(`{"event":{"input":{"command":"spawn"}}}`)

	var stdoutBuf bytes.Buffer
	cmd := SubagentStartHookCommand{
		Stdin:  &stdinBuf,
		Stdout: &stdoutBuf,
	}

	err := cmd.Run([]string{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var output codexHookOutput
	if jsonErr := json.Unmarshal(stdoutBuf.Bytes(), &output); jsonErr != nil {
		t.Fatalf("stdout is not valid JSON: %v\noutput: %q", jsonErr, stdoutBuf.String())
	}

	if output.HookSpecificOutput.HookEventName != "SubagentStart" {
		t.Errorf("HookEventName: got %q, want %q", output.HookSpecificOutput.HookEventName, "SubagentStart")
	}
	if !strings.Contains(output.HookSpecificOutput.AdditionalContext, "Golden Rules") {
		t.Errorf("AdditionalContext missing golden rules: %q", output.HookSpecificOutput.AdditionalContext)
	}
}

// TestHookCheckRun_GitCommand_OldStdout verifies backward compatibility with
// the old os.Stdout-based output (for the existing test pattern).
func TestHookCheckRun_GitCommand_OldStdout(t *testing.T) {
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
}
