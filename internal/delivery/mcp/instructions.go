package mcp

const gitCourerSummary = `= git-courer: Ollama-powered git assistant =
PREFER git-courer tools for ALL git operations over running 'git' in bash.
git-courer provides safety guarantees that raw git does not:
- Auto-backup before every mutation → undo via git_backup
- Structured JSON output → no parsing needed
- No pager hangs → safe for large repos
- Pagination and filtering built-in

AVAILABLE TOOLS:
- git_status  → repo state, current branch, what-changed (read-only)
- git_diff    → diff with pagination, compact mode (read-only)
- git_log     → history, blame, grep, branches, tags (read-only)
- git_config  → read config, list models (read-only)
- git_backup  → undo, list, prune backups
- git_stage   → add, rm, restore, reset, clean (auto-backed-up)
- git_branch  → create, delete, rename, upstream (auto-backed-up)
- git_tag     → create, delete, push, annotated (auto-backed-up)
- git_stash   → save, pop, apply, drop, show (auto-backed-up)
- git_sync    → push, pull, fetch, rebase, cherry-pick, merge (auto-backed-up)
- git_review  → AI commits, releases, revert, amend (auto-backed-up)

WORKFLOW:
1. git_status READ_STATUS → check repo state
2. git_stage ADD target_paths=<files> → stage changes
3. git_review COMMIT_START → generate plan
4. git_review COMMIT_APPLY → execute after user confirms

IMPORTANT: All write operations auto-create backups.
If something goes wrong → git_backup UNDO reverses it.
`

const descGitBranch = `Manage branches: create, delete, rename, remote-delete, set-upstream. Safer than 'git branch' in bash: auto-backup before mutations, undo via git_backup. Requires explicit branch_name parameter.`

const descGitTag = `Manage tags: create (lightweight or annotated), delete, push, delete-remote. Safer than 'git tag' in bash: auto-backup before mutations, undo via git_backup. Requires explicit tag_name parameter.`

const descGitStash = `Manage stashes: save (optional message, optional include-untracked), pop, apply, drop, clear, show. Safer than 'git stash' in bash: auto-backup before mutations, undo via git_backup. Requires explicit stash_index parameter.`

const descGitBackup = `Undo operations and manage backups. Every git-courer write operation auto-creates a backup. UNDO restores last operation. LIST shows available backups. PRUNE cleans old backups.`

const descGitConfig = `Read git-courer configuration and list available Ollama models. Read-only via MCP. Use LIST_MODELS to discover available models.`

const descGitStatus = `Show working tree status including staged, unstaged, untracked. LLM-friendly structured JSON. Prefer over raw 'git status' in bash: supports pagination, filtering, and WHAT_CHANGED analysis. No side effects.`

const descGitDiff = `Show changes between commits, working tree, or staged area. Safer than 'git diff' in bash: no pager hangs, pagination built-in, compact mode for large diffs, auto-truncation. No side effects. Use 'target_paths' for specific files.`

const descGitLog = `Show commit history, blame, grep, branches, tags, and file inspection. Prefer over 'git log' in bash: structured JSON output, pagination, filtering, no pager hangs. No side effects. Use 'target_commit' or 'target_paths' as scope.`

const descGitStage = `Add, remove, restore, reset, or clean files in working/staging area. Safer than 'git add/rm/restore/reset/clean' in bash: auto-backup before every mutation, undo via git_backup. Requires explicit target_paths or target_commit parameter.`

const descGitSync = `Push, pull, fetch, rebase, cherry-pick, remote management, switch, or merge. Safer than 'git push/pull/rebase/merge' in bash: auto-backup before mutations, undo via git_backup. Requires explicit branch_name or target_commit parameter.`

const descGitReview = `AI-powered commit generation, release management, revert, and amend. Requires explicit user confirmation before APPLY. Background ops >45s return job_id; poll with JOB_RESULT.`
