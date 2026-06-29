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
	"github.com/blak0p/git-courer/internal/installer"
)

// isStdinPipe returns true when os.Stdin is connected to a pipe (not a
// terminal). This is used to detect Codex hook stdin mode.
var isStdinPipe = func() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) == 0
}

// HookCheckCommand implements the hook-check CLI subcommand.
//
// It receives the shell command the agent is about to run, classifies it via
// gitcmd.Classify, and prints the classification Result as JSON to stdout.
// This is a thin adapter — all classification logic lives in the gitcmd
// package so it can be unit-tested independently.
//
// When no args are provided and stdin is a pipe, it reads Codex hook JSON
// from stdin, extracts the command, and classifies it. For git commands it
// emits additionalContext suggesting the git-courer MCP tool. Non-git
// commands exit cleanly with no output.
type HookCheckCommand struct {
	Stdin  io.Reader // for testing; nil = os.Stdin
	Stdout io.Writer // for testing; nil = os.Stdout
}

// codexHookInput represents the JSON structure Codex sends via stdin
// for PreToolUse hooks.
type codexHookInput struct {
	Event struct {
		Input struct {
			Command string `json:"command"`
		} `json:"input"`
	} `json:"event"`
}

// codexHookOutput represents the JSON structure Codex expects as output
// from a hook. PreToolUse uses permissionDecision + permissionDecisionReason.
// SessionStart/SubagentStart use additionalContext with golden rules.
type codexHookOutput struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision,omitempty"`
		PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
		AdditionalContext        string `json:"additionalContext,omitempty"`
	} `json:"hookSpecificOutput"`
}

// Run classifies args[0] and emits the Result as JSON on stdout.
// If no args are provided and stdin is a pipe, it reads Codex hook JSON
// from stdin and classifies the embedded command.
func (c HookCheckCommand) Run(args []string) error {
	stdin := c.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	stdout := c.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}

	// Stdin mode: no args, and either Stdin is explicitly set (testing) or
	// os.Stdin is a pipe (Codex hook mode).
	if len(args) == 0 {
		if c.Stdin != nil || isStdinPipe() {
			return c.runStdinMode(stdin, stdout)
		}
		return fmt.Errorf("usage: git-courer hook-check <command>")
	}

	command := args[0]
	if command == "" {
		return fmt.Errorf("usage: git-courer hook-check <command>")
	}

	result := gitcmd.Classify(command)

	// Emit as JSON — consistent with existing delivery patterns.
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		return fmt.Errorf("hook-check: failed to encode result: %w", err)
	}
	return nil
}

// runStdinMode reads Codex hook JSON from stdin, extracts the command,
// and emits Codex hook output. For git commands it denies permission and
// redirects to the git-courer MCP tool.
// Non-git commands exit cleanly with no output.
func (c HookCheckCommand) runStdinMode(stdin io.Reader, stdout io.Writer) error {
	data, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("hook-check: failed to read stdin: %w", err)
	}

	var input codexHookInput
	if err := json.Unmarshal(data, &input); err != nil {
		// If we can't parse the input, exit cleanly (safe fallback).
		return nil
	}

	command := input.Event.Input.Command
	if command == "" {
		return nil
	}

	result := gitcmd.Classify(command)

	// Only emit output for git commands (Decision == "ask").
	if result.Decision != "ask" {
		return nil
	}

	output := codexHookOutput{}
	output.HookSpecificOutput.HookEventName = "PreToolUse"
	output.HookSpecificOutput.PermissionDecision = "deny"
	output.HookSpecificOutput.PermissionDecisionReason = fmt.Sprintf("Use git-courer/%s instead of bash %s", result.MCPTool, command)

	if err := json.NewEncoder(stdout).Encode(output); err != nil {
		return fmt.Errorf("hook-check: failed to encode output: %w", err)
	}
	return nil
}

// SessionStartHookCommand implements the session-start-hook CLI subcommand.
// It reads stdin (ignored) and returns golden rules as additionalContext.
type SessionStartHookCommand struct {
	Stdin  io.Reader // for testing; nil = os.Stdin
	Stdout io.Writer // for testing; nil = os.Stdout
}

// Run reads stdin (ignored) and emits golden rules as Codex hook output.
func (c SessionStartHookCommand) Run(args []string) error {
	stdout := c.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}

	// Read and discard stdin (Codex sends event JSON but we don't need it).
	stdin := c.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	_, _ = io.ReadAll(stdin)

	output := codexHookOutput{}
	output.HookSpecificOutput.HookEventName = "SessionStart"
	output.HookSpecificOutput.AdditionalContext = installer.GoldenRulesAdditionalContext

	return json.NewEncoder(stdout).Encode(output)
}

// SubagentStartHookCommand implements the subagent-start-hook CLI subcommand.
// It reads stdin (ignored) and returns golden rules as additionalContext.
type SubagentStartHookCommand struct {
	Stdin  io.Reader // for testing; nil = os.Stdin
	Stdout io.Writer // for testing; nil = os.Stdout
}

// Run reads stdin (ignored) and emits golden rules as Codex hook output.
func (c SubagentStartHookCommand) Run(args []string) error {
	stdout := c.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}

	// Read and discard stdin (Codex sends event JSON but we don't need it).
	stdin := c.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	_, _ = io.ReadAll(stdin)

	output := codexHookOutput{}
	output.HookSpecificOutput.HookEventName = "SubagentStart"
	output.HookSpecificOutput.AdditionalContext = installer.GoldenRulesAdditionalContext

	return json.NewEncoder(stdout).Encode(output)
}

// PreInvocationHookCommand implements the pre-invocation-hook CLI subcommand.
// It reads stdin (ignored) and returns golden rules as additionalContext,
// mirroring SessionStartHookCommand exactly. Invoked by the Antigravity
// PreInvocation hook before every model call to inject golden rules into the
// agent's context.
type PreInvocationHookCommand struct {
	Stdin  io.Reader // for testing; nil = os.Stdin
	Stdout io.Writer // for testing; nil = os.Stdout
}

// Run reads stdin (ignored) and emits golden rules as Codex hook output.
// It accepts an optional first argument in args to override the HookEventName
// from the default "PreInvocation".
func (c PreInvocationHookCommand) Run(args []string) error {
	stdout := c.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}

	// Read and discard stdin (Antigravity sends event JSON but we don't need it).
	stdin := c.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	_, _ = io.ReadAll(stdin)

	eventName := "PreInvocation"
	if len(args) > 0 && args[0] != "" {
		eventName = args[0]
	}

	output := codexHookOutput{}
	output.HookSpecificOutput.HookEventName = eventName
	output.HookSpecificOutput.AdditionalContext = installer.GoldenRulesAdditionalContext

	return json.NewEncoder(stdout).Encode(output)
}

