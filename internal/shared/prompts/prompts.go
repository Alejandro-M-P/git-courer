// Package prompts provides LLM prompt templates for git-courer operations.
// Each operation has its own focused prompt — no generic one-size-fits-all.
package prompts

import (
	"bytes"
	"strings"
	"text/template"
)

// --- Commit prompts ---

const GenerateMessage = `Generate a conventional commit message for the following changes.

Files changed: {{.Files}}

Diff:
{{.Diff}}
{{- if .RejectedMessage}}

The previous message was rejected by the user: "{{.RejectedMessage}}"
Generate a DIFFERENT and better message.
{{- end}}

Rules:
- Conventional commits format: type(scope): description
- Types: feat, fix, refactor, docs, test, chore, perf, ci, build, style
- Max 72 characters
- Be specific — mention what changed, not just "update files"
- Respond with ONLY the commit message, nothing else`

const DecideCommit = `You are a git expert deciding which files to include in a commit.

User instruction: "{{.Instruction}}"

Git status:
{{.GitStatus}}

Untracked files (not yet tracked by git):
{{.Untracked}}

Modified tracked files:
{{.Modified}}

Deleted files:
{{.Deleted}}

Based on the instruction, should untracked files be staged? Is there a file filter?
Respond with ONLY a JSON object:
{"include_untracked": false, "filter": ""}`

// --- Per-operation prompts for git_write_review ---

const BranchCreate = `You are a git expert creating a new branch.

User instruction: "{{.Instruction}}"

Current branch: {{.CurrentBranch}}
Existing branches:
{{.Branches}}

Generate a branch name following these conventions:
- feat/   → new features
- fix/    → bug fixes
- refactor/ → refactoring
- docs/   → documentation
- test/   → tests
- chore/  → maintenance

Use kebab-case, keep it short and descriptive.
Do NOT include an existing branch name.

Respond with ONLY a JSON object:
{"branch": "feat/short-description"}`

const BranchDelete = `You are a git expert deleting a branch.

User instruction: "{{.Instruction}}"

Available branches:
{{.Branches}}

Identify which branch the user wants to delete.

Respond with ONLY a JSON object:
{"branch": "branch-name"}`

const BranchRename = `You are a git expert renaming a branch.

User instruction: "{{.Instruction}}"

Current branch: {{.CurrentBranch}}
Available branches:
{{.Branches}}

Identify the old name and generate the new name.
Follow the same naming conventions (feat/, fix/, etc.).

Respond with ONLY a JSON object:
{"old_name": "old-branch", "new_name": "feat/new-name"}`

const Merge = `You are a git expert merging branches.

User instruction: "{{.Instruction}}"

Current branch: {{.CurrentBranch}}
Available branches:
{{.Branches}}

Identify which branch should be merged INTO the current branch.

Respond with ONLY a JSON object:
{"branch": "branch-to-merge"}`

const Rebase = `You are a git expert rebasing branches.

User instruction: "{{.Instruction}}"

Current branch: {{.CurrentBranch}}
Available branches:
{{.Branches}}

Recent commits on current branch:
{{.RecentCommits}}

Identify which branch the current branch should be rebased onto.

Respond with ONLY a JSON object:
{"branch": "branch-to-rebase-onto"}`

const ResetHard = `You are a git expert performing a hard reset.

User instruction: "{{.Instruction}}"

Recent commit history:
{{.RecentCommits}}

Identify the target for the reset. Common values:
- HEAD~1, HEAD~2, HEAD~3 (go back N commits)
- A specific commit hash from the log above

Respond with ONLY a JSON object:
{"target": "HEAD~1"}`

const CherryPick = `You are a git expert cherry-picking a commit.

User instruction: "{{.Instruction}}"

Recent commit history:
{{.RecentCommits}}

Identify which commit hash the user wants to cherry-pick.
Use the exact short hash from the log.

Respond with ONLY a JSON object:
{"commit": "abc1234"}`

const Revert = `You are a git expert reverting a commit.

User instruction: "{{.Instruction}}"

Recent commit history:
{{.RecentCommits}}

Identify which commit hash the user wants to revert.
This creates a NEW commit that undoes the changes.
Use the exact short hash from the log.

Respond with ONLY a JSON object:
{"commit": "abc1234"}`

const Clean = `You are a git expert cleaning untracked files.

User instruction: "{{.Instruction}}"

Untracked files that would be removed:
{{.UntrackedFiles}}

Should directories also be removed? (default: false, only remove untracked files)

Respond with ONLY a JSON object:
{"dirs": "false"}`

const TagCreate = `You are a git expert creating a git tag.

User instruction: "{{.Instruction}}"

Existing tags:
{{.Tags}}

Recent commits:
{{.RecentCommits}}

Generate an appropriate tag name. Follow semantic versioning if existing tags suggest it (v1.0.0, v1.1.0, etc.).

Respond with ONLY a JSON object:
{"name": "v1.0.0"}`

const TagDelete = `You are a git expert deleting a git tag.

User instruction: "{{.Instruction}}"

Existing tags:
{{.Tags}}

Identify which tag the user wants to delete.

Respond with ONLY a JSON object:
{"name": "tag-name"}`

const RemoteAdd = `You are a git expert adding a remote.

User instruction: "{{.Instruction}}"

Extract the remote name (usually "origin") and the URL from the instruction.

Respond with ONLY a JSON object:
{"name": "origin", "url": "https://github.com/user/repo.git"}`

const RemoteRemove = `You are a git expert removing a remote.

User instruction: "{{.Instruction}}"

Identify which remote to remove (usually "origin").

Respond with ONLY a JSON object:
{"name": "origin"}`

const Clone = `You are a git expert cloning a repository.

User instruction: "{{.Instruction}}"

Extract the repository URL from the instruction.

Respond with ONLY a JSON object:
{"url": "https://github.com/user/repo.git"}`

// --- Template params ---

type MessageParams struct {
	Files           string
	Diff            string
	RejectedMessage string
}

type DecideParams struct {
	Instruction string
	GitStatus   string
	Untracked   string
	Modified    string
	Deleted     string
}

// OpParams holds the context for any per-operation prompt.
// Not all fields are used by every operation.
type OpParams struct {
	Instruction   string
	CurrentBranch string
	Branches      string
	Tags          string
	RecentCommits string
	UntrackedFiles string
}

// --- Builders ---

func BuildMessageParams(files []string, diff string) MessageParams {
	return MessageParams{Files: strings.Join(files, ", "), Diff: diff}
}

func BuildMessageParamsWithRetry(files []string, diff, rejected string) MessageParams {
	return MessageParams{Files: strings.Join(files, ", "), Diff: diff, RejectedMessage: rejected}
}

func BuildDecideParams(instruction, gitStatus, untracked, modified, deleted string) DecideParams {
	return DecideParams{
		Instruction: instruction, GitStatus: gitStatus,
		Untracked: untracked, Modified: modified, Deleted: deleted,
	}
}

// TemplateFor returns the prompt template for the given operation.
// Falls back to a minimal generic prompt for unknown operations.
func TemplateFor(op string) string {
	switch op {
	case "branch_create":
		return BranchCreate
	case "branch_delete":
		return BranchDelete
	case "branch_rename":
		return BranchRename
	case "merge":
		return Merge
	case "rebase":
		return Rebase
	case "reset_hard":
		return ResetHard
	case "cherry_pick":
		return CherryPick
	case "revert":
		return Revert
	case "clean":
		return Clean
	case "tag_create":
		return TagCreate
	case "tag_delete":
		return TagDelete
	case "remote_add":
		return RemoteAdd
	case "remote_remove":
		return RemoteRemove
	case "clone":
		return Clone
	default:
		return `Interpret this git instruction: "{{.Instruction}}"
Respond with ONLY a JSON object with the relevant arguments.`
	}
}

// BuildOpParams constructs OpParams from a context map (as provided by prepare.go).
func BuildOpParams(instruction string, ctx map[string]string) OpParams {
	return OpParams{
		Instruction:    instruction,
		CurrentBranch:  ctx["current_branch"],
		Branches:       ctx["branches"],
		Tags:           ctx["tags"],
		RecentCommits:  ctx["recent_commits"],
		UntrackedFiles: ctx["untracked_files"],
	}
}

// Render renders a template string with the given data.
func Render(tmpl string, data interface{}) (string, error) {
	t, err := template.New("prompt").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
