// Package gitcmd_test verifies the git command classifier behavior.
package gitcmd

import (
	"strings"
	"testing"
)

// TestClassify_KnownGitSubcommands verifies each known git subcommand maps
// to the expected MCP tool with decision "ask" and a reason that references
// the suggested tool.
func TestClassify_KnownGitSubcommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		command    string
		wantTool   string
		wantReason string
	}{
		{"status", "git status", "status", "git-courer/status"},
		{"diff", "git diff", "diff", "git-courer/diff"},
		{"commit", "git commit", "commit", "git-courer/commit"},
		{"log", "git log", "history", "git-courer/history"},
		{"branch", "git branch", "branch", "git-courer/branch"},
		{"merge", "git merge", "integrate", "git-courer/integrate"},
		{"rebase", "git rebase", "integrate", "git-courer/integrate"},
		{"cherry-pick", "git cherry-pick", "integrate", "git-courer/integrate"},
		{"revert", "git revert", "rewrite", "git-courer/rewrite"},
		{"reset", "git reset", "rewrite", "git-courer/rewrite"},
		{"stash", "git stash", "stash", "git-courer/stash"},
		{"push", "git push", "sync", "git-courer/sync"},
		{"pull", "git pull", "sync", "git-courer/sync"},
		{"fetch", "git fetch", "sync", "git-courer/sync"},
		{"show", "git show", "history", "git-courer/history"},
		{"blame", "git blame", "history", "git-courer/history"},
		{"remote", "git remote", "sync", "git-courer/sync"},
		{"config", "git config", "config", "git-courer/config"},
		{"add", "git add", "stage", "git-courer/stage"},
		{"restore", "git restore", "stage", "git-courer/stage"},
		{"clean", "git clean", "stage", "git-courer/stage"},
		{"rm", "git rm", "stage", "git-courer/stage"},
		{"mv", "git mv", "stage", "git-courer/stage"},
		{"switch", "git switch", "branch", "git-courer/branch"},
		{"checkout", "git checkout", "branch", "git-courer/branch"},
		{"worktree", "git worktree", "branch", "git-courer/branch"},
		{"shortlog", "git shortlog", "history", "git-courer/history"},
		{"describe", "git describe", "history", "git-courer/history"},
		{"reflog", "git reflog", "history", "git-courer/history"},
		{"notes", "git notes", "history", "git-courer/history"},
		{"archive", "git archive", "history", "git-courer/history"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Classify(tt.command)
			if r.Command != tt.command {
				t.Errorf("Command: got %q, want %q", r.Command, tt.command)
			}
			if r.Decision != "ask" {
				t.Errorf("Decision: got %q, want %q", r.Decision, "ask")
			}
			if r.MCPTool != tt.wantTool {
				t.Errorf("MCPTool: got %q, want %q", r.MCPTool, tt.wantTool)
			}
			if r.Reason == "" {
				t.Error("Reason is empty — expected a non-empty reason")
			}
			if !strings.Contains(r.Reason, tt.wantReason) {
				t.Errorf("Reason: got %q, want it to contain %q", r.Reason, tt.wantReason)
			}
		})
	}
}

// TestClassify_MaintenanceCommands verifies maintenance/info commands that
// have no MCP equivalent are allowed directly.
func TestClassify_MaintenanceCommands(t *testing.T) {
	t.Parallel()

	commands := []string{
		"git gc",
		"git fsck",
		"git prune",
		"git repack",
		"git maintenance",
		"git help",
		"git version",
		"git init",
		"git clone",
	}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			r := Classify(cmd)
			if r.Decision != "allow" {
				t.Errorf("Decision: got %q, want allow", r.Decision)
			}
			if r.MCPTool != "" {
				t.Errorf("MCPTool: got %q, want empty (no MCP equivalent)", r.MCPTool)
			}
			if r.Reason != "" {
				t.Errorf("Reason: got %q, want empty for allow", r.Reason)
			}
		})
	}
}

// TestClassify_NonGitCommands verifies non-git commands are allowed.
func TestClassify_NonGitCommands(t *testing.T) {
	t.Parallel()

	commands := []string{
		"ls -la",
		"echo hello",
		"npm install",
		"go test ./...",
		"cat file.txt",
		"grep pattern file",
	}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			r := Classify(cmd)
			if r.Command != cmd {
				t.Errorf("Command: got %q, want %q", r.Command, cmd)
			}
			if r.Decision != "allow" {
				t.Errorf("Decision: got %q, want allow", r.Decision)
			}
			if r.MCPTool != "" {
				t.Errorf("MCPTool: got %q, want empty", r.MCPTool)
			}
			if r.Reason != "" {
				t.Errorf("Reason: got %q, want empty for non-git allow", r.Reason)
			}
		})
	}
}

// TestClassify_UnknownGitSubcommand verifies an unknown git subcommand
// falls back to ask with a generic reason.
func TestClassify_UnknownGitSubcommand(t *testing.T) {
	t.Parallel()

	r := Classify("git totally-made-up")
	if r.Command != "git totally-made-up" {
		t.Errorf("Command: got %q, want %q", r.Command, "git totally-made-up")
	}
	if r.Decision != "ask" {
		t.Errorf("Decision: got %q, want ask", r.Decision)
	}
	if r.MCPTool != "" {
		t.Errorf("MCPTool: got %q, want empty for unknown subcommand", r.MCPTool)
	}
	if r.Reason == "" {
		t.Error("Reason is empty — expected a generic reason for unknown git subcommand")
	}
}

// TestClassify_EmptyInput verifies empty input is treated as non-git (allow).
func TestClassify_EmptyInput(t *testing.T) {
	t.Parallel()

	r := Classify("")
	if r.Command != "" {
		t.Errorf("Command: got %q, want empty", r.Command)
	}
	if r.Decision != "allow" {
		t.Errorf("Decision: got %q, want allow", r.Decision)
	}
	if r.MCPTool != "" {
		t.Errorf("MCPTool: got %q, want empty", r.MCPTool)
	}
	if r.Reason != "" {
		t.Errorf("Reason: got %q, want empty", r.Reason)
	}
}

// TestClassify_GitWithSubcommandArgs verifies the classifier extracts the
// subcommand correctly even when extra arguments follow.
func TestClassify_GitWithSubcommandArgs(t *testing.T) {
	t.Parallel()

	r := Classify("git status --short --branch")
	if r.Decision != "ask" {
		t.Errorf("Decision: got %q, want ask", r.Decision)
	}
	if r.MCPTool != "status" {
		t.Errorf("MCPTool: got %q, want status", r.MCPTool)
	}
}

// TestClassify_BareGit verifies a bare "git" with no subcommand is allowed
// (no mutation risk — usually a help printout).
func TestClassify_BareGit(t *testing.T) {
	t.Parallel()

	r := Classify("git")
	if r.Decision != "allow" {
		t.Errorf("Decision: got %q, want allow (bare git prints help)", r.Decision)
	}
}