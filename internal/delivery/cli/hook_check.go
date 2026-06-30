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
// When no args are provided and stdin is a pipe, it reads multi-agent hook
// JSON from stdin (Codex, Claude Code, or Antigravity shape), detects the
// calling agent from the top-level JSON keys, extracts the command, and
// classifies it. For git commands it emits the agent-specific deny output so
// the agent is blocked from running raw git and is pointed at the git-courer
// MCP tool. Non-git commands exit cleanly with no output.
type HookCheckCommand struct {
	Stdin  io.Reader  // for testing; nil = os.Stdin
	Stdout io.Writer  // for testing; nil = os.Stdout
}

// agentType identifies the calling agent from its stdin JSON shape.
type agentType int

const (
	agentUnknown     agentType = iota
	agentCodex                 // {"event": {...}}
	agentClaude                // {"tool_name": "..."}
	agentAntigravity           // {"toolCall": {...}}
)

// hookInput represents the three supported stdin shapes. Each pointer
// field is nil when the corresponding JSON key is absent, so a single
// json.Unmarshal enables agent detection via nil checks. Only one shape is
// expected per invocation.
type hookInput struct {
	Event *struct {
		Input *struct {
			Command string `json:"command"`
		} `json:"input"`
	} `json:"event"`

	ToolName string `json:"tool_name"`
	ToolInput *struct {
		Command string `json:"command"`
	} `json:"tool_input"`

	ToolCall *struct {
		Name string `json:"name"`
		Args *struct {
			CommandLine string `json:"CommandLine"`
		} `json:"args"`
	} `json:"toolCall"`
}

// antigravityHookOutput is the top-level deny format Antigravity expects:
// a flat {"allow_tool": false, "deny_reason": "..."} object (not nested
// under hookSpecificOutput like Codex/Claude).
type antigravityHookOutput struct {
	AllowTool   bool   `json:"allow_tool"`
	DenyReason  string `json:"deny_reason"`
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

// runStdinMode reads multi-agent hook JSON from stdin, detects the calling
// agent from the top-level JSON keys (event → Codex, tool_name → Claude Code,
// toolCall → Antigravity), extracts the command, and emits the agent-specific
// deny output for git commands. Unrecognized or unparseable stdin falls back
// to exit 0 with no output (safe fallback). Non-git commands exit cleanly
// with no output.
func (c HookCheckCommand) runStdinMode(stdin io.Reader, stdout io.Writer) error {
	data, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("hook-check: failed to read stdin: %w", err)
	}

	var input hookInput
	if err := json.Unmarshal(data, &input); err != nil {
		// Unparseable stdin — exit cleanly (safe fallback).
		return nil
	}

	var command string
	var agent agentType

	switch {
	case input.Event != nil && input.Event.Input != nil:
		command = input.Event.Input.Command
		agent = agentCodex
	case input.ToolName != "":
		if input.ToolInput != nil {
			command = input.ToolInput.Command
		}
		agent = agentClaude
	case input.ToolCall != nil && input.ToolCall.Args != nil:
		command = input.ToolCall.Args.CommandLine
		agent = agentAntigravity
	default:
		return nil
	}

	if command == "" {
		return nil
	}

	result := gitcmd.Classify(command)

	// Only emit output for git commands (Decision == "ask").
	if result.Decision != "ask" {
		return nil
	}

	reason := fmt.Sprintf("Use git-courer/%s instead of bash %s", result.MCPTool, command)

	switch agent {
	case agentCodex, agentClaude:
		output := codexHookOutput{}
		output.HookSpecificOutput.HookEventName = "PreToolUse"
		output.HookSpecificOutput.PermissionDecision = "deny"
		output.HookSpecificOutput.PermissionDecisionReason = reason
		if err := json.NewEncoder(stdout).Encode(output); err != nil {
			return fmt.Errorf("hook-check: failed to encode output: %w", err)
		}
	case agentAntigravity:
		output := antigravityHookOutput{
			AllowTool:  false,
			DenyReason: reason,
		}
		if err := json.NewEncoder(stdout).Encode(output); err != nil {
			return fmt.Errorf("hook-check: failed to encode output: %w", err)
		}
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

// Run reads stdin (ignored) and emits golden rules as Codex hook output with
// HookEventName=PreInvocation. No permissionDecision is set.
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

	output := codexHookOutput{}
	output.HookSpecificOutput.HookEventName = "PreInvocation"
	output.HookSpecificOutput.AdditionalContext = installer.GoldenRulesAdditionalContext

	return json.NewEncoder(stdout).Encode(output)
}

