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

// TestHookCheckStdin_GitCommand verifies stdin mode emits a deny decision for
// git commands (Codex shape), pointing the agent at the git-courer MCP tool.
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
	if output.HookSpecificOutput.PermissionDecision != "deny" {
		t.Errorf("PermissionDecision: got %q, want %q", output.HookSpecificOutput.PermissionDecision, "deny")
	}
	if !strings.Contains(output.HookSpecificOutput.PermissionDecisionReason, "git-courer/status") {
		t.Errorf("PermissionDecisionReason missing MCP tool: %q", output.HookSpecificOutput.PermissionDecisionReason)
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

// TestHookCheckStdin_ClaudeCode_GitCommand verifies stdin mode emits a deny
// decision for git commands in the Claude Code stdin shape.
func TestHookCheckStdin_ClaudeCode_GitCommand(t *testing.T) {
	input := `{"tool_name":"Bash","tool_input":{"command":"git status"}}`
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

	if output.HookSpecificOutput.PermissionDecision != "deny" {
		t.Errorf("PermissionDecision: got %q, want %q", output.HookSpecificOutput.PermissionDecision, "deny")
	}
	if !strings.Contains(output.HookSpecificOutput.PermissionDecisionReason, "git-courer/status") {
		t.Errorf("PermissionDecisionReason missing MCP tool: %q", output.HookSpecificOutput.PermissionDecisionReason)
	}
}

// TestHookCheckStdin_ClaudeCode_NonGitCommand verifies stdin mode produces no
// output for non-git commands in the Claude Code stdin shape.
func TestHookCheckStdin_ClaudeCode_NonGitCommand(t *testing.T) {
	input := `{"tool_name":"Bash","tool_input":{"command":"ls -la"}}`
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

// TestHookCheckStdin_Antigravity_GitCommand verifies stdin mode emits an
// Antigravity deny (allow_tool: false) for git commands.
func TestHookCheckStdin_Antigravity_GitCommand(t *testing.T) {
	input := `{"toolCall":{"name":"run_command","args":{"CommandLine":"git status"}}}`
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

	var output antigravityHookOutput
	if jsonErr := json.Unmarshal(stdoutBuf.Bytes(), &output); jsonErr != nil {
		t.Fatalf("stdout is not valid JSON: %v\noutput: %q", jsonErr, stdoutBuf.String())
	}

	if output.AllowTool {
		t.Errorf("AllowTool: got true, want false")
	}
	if !strings.Contains(output.DenyReason, "git-courer/status") {
		t.Errorf("DenyReason missing MCP tool: %q", output.DenyReason)
	}
}

// TestHookCheckStdin_Antigravity_NonGitCommand verifies stdin mode produces
// no output for non-git commands in the Antigravity stdin shape.
func TestHookCheckStdin_Antigravity_NonGitCommand(t *testing.T) {
	input := `{"toolCall":{"name":"run_command","args":{"CommandLine":"ls -la"}}}`
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

// TestPreInvocationHookCommand_ReturnsGoldenRules verifies pre-invocation-hook
// returns golden rules as additionalContext with HookEventName=PreInvocation
// and no permissionDecision.
func TestPreInvocationHookCommand_ReturnsGoldenRules(t *testing.T) {
	var stdinBuf bytes.Buffer
	stdinBuf.WriteString(`{"event":{"input":{"command":"model"}}}`)

	var stdoutBuf bytes.Buffer
	cmd := PreInvocationHookCommand{
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

	if output.HookSpecificOutput.HookEventName != "PreInvocation" {
		t.Errorf("HookEventName: got %q, want %q", output.HookSpecificOutput.HookEventName, "PreInvocation")
	}
	if !strings.Contains(output.HookSpecificOutput.AdditionalContext, "Golden Rules") {
		t.Errorf("AdditionalContext missing golden rules: %q", output.HookSpecificOutput.AdditionalContext)
	}
	if output.HookSpecificOutput.PermissionDecision != "" {
		t.Errorf("PermissionDecision should be empty, got %q", output.HookSpecificOutput.PermissionDecision)
	}
}

// TestPreInvocationHookCommand_EmptyStdin verifies pre-invocation-hook works
// with an empty stdin payload (stdin is read and discarded).
func TestPreInvocationHookCommand_EmptyStdin(t *testing.T) {
	var stdinBuf bytes.Buffer
	var stdoutBuf bytes.Buffer
	cmd := PreInvocationHookCommand{
		Stdin:  &stdinBuf,
		Stdout: &stdoutBuf,
	}

	err := cmd.Run([]string{})
	if err != nil {
		t.Fatalf("Run returned error on empty stdin: %v", err)
	}

	var output codexHookOutput
	if jsonErr := json.Unmarshal(stdoutBuf.Bytes(), &output); jsonErr != nil {
		t.Fatalf("stdout is not valid JSON: %v\noutput: %q", jsonErr, stdoutBuf.String())
	}
	if output.HookSpecificOutput.HookEventName != "PreInvocation" {
		t.Errorf("HookEventName: got %q, want PreInvocation", output.HookSpecificOutput.HookEventName)
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
