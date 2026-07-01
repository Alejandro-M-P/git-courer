// Package gitcmd classifies shell commands to determine whether a git
// subcommand should be redirected to the corresponding git-courer MCP tool.
//
// Classify is a pure function: it inspects the command string only and does
// not touch the filesystem or execute anything. The returned Result tells
// the hook-check adapter what to emit to the agent so it uses the safer,
// structured MCP tool instead of raw git.
package gitcmd

import "strings"

// Result is the outcome of classifying a command.
//
// Decision is one of:
//   - "allow" — the command is safe to run as-is (non-git, maintenance, etc.)
//   - "deny"  — the command is a git subcommand covered by a git-courer MCP
//     tool; the agent should use that tool instead of raw git.
//   - "ask"   — the command is an unknown git subcommand with no known MCP
//     equivalent; the safe default is to ask before running.
//
// MCPTool is the git-courer MCP tool name that should be used in place of the
// raw git subcommand. It is empty when Decision is "allow" or "ask".
//
// Reason is a human-readable explanation shown to the agent. For "deny"
// decisions it follows the format "Use git-courer/{tool} instead of bash
// git {subcommand}".
type Result struct {
	Command  string
	Decision string
	MCPTool  string
	Reason   string
}

// mcpTools maps a git subcommand to the git-courer MCP tool that should be
// used instead. Only the 23 subcommands covered by a git-courer MCP tool are
// listed here; Classify returns "deny" for these. Maintenance and
// informational commands are intentionally absent — they are allowed directly
// by Classify.
var mcpTools = map[string]string{
	"status":      "status",
	"diff":        "diff",
	"commit":      "commit",
	"log":         "history",
	"branch":      "branch",
	"merge":       "integrate",
	"rebase":      "integrate",
	"cherry-pick": "integrate",
	"revert":      "rewrite",
	"reset":       "rewrite",
	"stash":       "stash",
	"push":        "sync",
	"pull":        "sync",
	"fetch":       "sync",
	"blame":       "history",
	"add":         "stage",
	"restore":     "stage",
	"clean":       "stage",
	"rm":          "stage",
	"switch":      "branch",
	"checkout":    "branch",
	"worktree":    "branch",
	"reflog":      "history",
}

// allowedSubcommands are git subcommands with no MCP equivalent that are safe
// to run directly (maintenance, informational, one-time setup, and the 8
// subcommands that git-courer does not yet cover).
var allowedSubcommands = map[string]bool{
	"gc":            true,
	"fsck":          true,
	"init":          true,
	"clone":         true,
	"show":          true,
	"remote":        true,
	"config":        true,
	"mv":            true,
	"shortlog":      true,
	"describe":      true,
	"notes":         true,
	"archive":       true,
	"submodule":     true,
	"bisect":        true,
	"verify-commit": true,
	"verify-tag":    true,
	"grep":          true,
	"cherry":        true,
	"ls-files":      true,
	"ls-tree":       true,
	"ls-remote":     true,
	"tag":           true,
}

// Classify inspects a command string and returns a Result indicating whether
// the agent should run it directly ("allow"), use the corresponding
// git-courer MCP tool instead ("deny"), or prompt before running ("ask" — the
// safe default for unknown git subcommands).
//
// Classify is a pure function — it has no side effects and is deterministic
// for a given input.
func Classify(command string) Result {
	r := Result{Command: command, Decision: "allow"}

	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		// Empty input — nothing to run, treat as allow.
		return r
	}

	if !strings.HasPrefix(trimmed, "git") {
		// Non-git command — allow.
		return r
	}

	// Extract the subcommand token after "git".
	fields := strings.Fields(trimmed)
	if len(fields) <= 1 {
		// Bare "git" (no subcommand) — prints help, allow.
		return r
	}

	sub := fields[1]

	if allowedSubcommands[sub] {
		// Maintenance/informational — no MCP equivalent, allow.
		return r
	}

	if tool, ok := mcpTools[sub]; ok {
		r.Decision = "deny"
		r.MCPTool = tool
		r.Reason = "Use git-courer/" + tool + " instead of bash git " + sub
		return r
	}

	// Unknown git subcommand — safe default is ask with a generic reason.
	r.Decision = "ask"
	r.MCPTool = ""
	r.Reason = "Unknown git subcommand '" + sub + "' — check for a git-courer MCP tool before running raw git"
	return r
}
