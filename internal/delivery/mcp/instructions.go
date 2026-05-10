package mcp

const gitCourerSummary = `git-courer: Ollama-powered git assistant. Replaces bash git entirely.
Zero cloud tokens. Runs locally via Ollama.

- git_status     → repo state, jobs, config
- git_diff       → diff operations with pagination and compact mode
- git_log        → history, blame, grep, branches, tags, file inspection
- git_stage      → staging area and stash
- git_sync       → push, pull, fetch, switch, merge
- git_manage     → branches, tags, remotes, resets, undo
- git_review     → commits + releases (Ollama, conventional commits)

Workflow:
1. git_status READ_STATUS — check state
2. git_stage ADD — stage changes
3. git_review COMMIT_START — generate commit plan
4. git_review COMMIT_APPLY — execute after user confirmation

All write operations are auto-backed-up. UNDO via git_manage.
Background jobs >45s return job_id; poll with git_review JOB_RESULT.
`

const descGitStatus = `Show the working tree status including staged, unstaged, and untracked files. No side effects.`

const descGitDiff = `Diff operations with pagination and compact mode. No side effects. Use compact=true to save tokens on large diffs.`

const descGitLog = `History, blame, grep, branches, tags, and file inspection. No side effects.`

const descGitStage = `Staging area manipulation. Every op auto-backed-up — UNDO in git_manage reverts it.`

const descGitSync = `Push, pull, fetch, switch, and merge. Every op auto-backed-up — UNDO in git_manage reverts it.`

const descGitManage = `Branches, tags, remotes, resets, and undo. Every op auto-backed-up — UNDO reverts the last one.`

const descGitReview = `Ollama-powered commits and releases. Requires explicit user confirmation before APPLY. Multiple commits per session is correct — one per logical change. Background: ops >45s return job_id, poll with git_review JOB_RESULT.`
