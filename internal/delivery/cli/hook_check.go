// Package cli provides CLI delivery adapters for git-courer subcommands.
//
// This file implements the hook-check subcommand: a thin adapter that
// classifies a shell command via the gitcmd classifier and emits the Result
// as JSON on stdout so the calling agent can decide which tool to use.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/blak0p/git-courer/internal/classifier/gitcmd"
)

// CodexHookInput is the JSON that Codex CLI sends via stdin to a PreToolUse hook.
type CodexHookInput struct {
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

// CodexHookOutput is the JSON the hook must emit on stdout for Codex CLI.
type CodexHookOutput struct {
	HookSpecificOutput struct {
		HookEventName             string `json:"hookEventName"`
		PermissionDecision        string `json:"permissionDecision"`
		PermissionDecisionReason  string `json:"permissionDecisionReason"`
	} `json:"hookSpecificOutput"`
}

// HookCheckCommand implements the hook-check CLI subcommand.
//
// It supports two modes:
//   - CLI mode (args provided): classifies the command and emits Result JSON.
//   - Stdin mode (no args): reads a Codex hook payload from stdin, classifies
//     the command, and emits CodexHookOutput JSON with permissionDecision.
//
// All classification logic lives in the gitcmd package so it can be
// unit-tested independently.
type HookCheckCommand struct {
	Stdin  io.Reader // defaults to os.Stdin; mockable in tests
	Stdout io.Writer // defaults to os.Stdout; mockable in tests
}

// Run classifies a shell command and emits the result as JSON.
//
// When args are provided, it uses CLI mode (existing behavior) and emits the
// gitcmd.Result struct. When no args are provided, it reads a Codex hook
// payload from Stdin, classifies the command, and emits CodexHookOutput.
func (c HookCheckCommand) Run(args []string) error {
	stdin := c.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	stdout := c.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}

	// CLI mode: args provided
	if len(args) > 0 {
		command := args[0]
		if command == "" {
			return fmt.Errorf("usage: git-courer hook-check <command>")
		}

		result := gitcmd.Classify(command)
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			return fmt.Errorf("hook-check: failed to encode result: %w", err)
		}
		return nil
	}

	// No args and no Stdin set — programmer error, not stdin mode.
	if c.Stdin == nil {
		return fmt.Errorf("usage: git-courer hook-check <command>")
	}

	// Stdin mode: no args, read Codex hook payload from stdin
	return c.runStdinMode(stdin, stdout)
}

// runStdinMode reads a Codex hook payload from stdin, classifies the command,
// and emits CodexHookOutput JSON. On parse errors, it falls back to allow
// (safe fallback per spec).
func (c HookCheckCommand) runStdinMode(stdin io.Reader, stdout io.Writer) error {
	data, err := io.ReadAll(stdin)
	if err != nil {
		// Can't read stdin — safe fallback to allow
		return emitCodexAllow(stdout, "failed to read stdin")
	}

	if len(data) == 0 {
		return emitCodexAllow(stdout, "empty stdin")
	}

	var input CodexHookInput
	if err := json.Unmarshal(data, &input); err != nil {
		// Malformed JSON — safe fallback to allow
		return emitCodexAllow(stdout, "failed to parse stdin JSON")
	}

	if input.ToolInput.Command == "" {
		return emitCodexAllow(stdout, "empty command")
	}

	result := gitcmd.Classify(input.ToolInput.Command)

	output := CodexHookOutput{}
	output.HookSpecificOutput.HookEventName = "PreToolUse"

	switch result.Decision {
	case "ask":
		output.HookSpecificOutput.PermissionDecision = "deny"
		output.HookSpecificOutput.PermissionDecisionReason = result.Reason
	default:
		output.HookSpecificOutput.PermissionDecision = "allow"
		output.HookSpecificOutput.PermissionDecisionReason = result.Reason
	}

	return json.NewEncoder(stdout).Encode(output)
}

// emitCodexAllow writes a CodexHookOutput with permissionDecision=allow and
// the given reason. Used as safe fallback on parse errors.
func emitCodexAllow(stdout io.Writer, reason string) error {
	output := CodexHookOutput{}
	output.HookSpecificOutput.HookEventName = "PreToolUse"
	output.HookSpecificOutput.PermissionDecision = "allow"
	output.HookSpecificOutput.PermissionDecisionReason = reason
	return json.NewEncoder(stdout).Encode(output)
}
