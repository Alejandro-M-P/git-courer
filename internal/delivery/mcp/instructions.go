package mcp

const gitCourerSummary = `git-courer: Ollama-powered git assistant. Replaces bash git entirely.
Zero cloud tokens. Runs locally via Ollama.

WHY better than bash:
- Commits: semantic conventional commits auto-generated from diff analysis
- Splitting: multiple focused commits per session, one per logical change
- Reads: paginated JSON — bash is unparseable noise
- Writes: every op auto-backed-up, reversible with UNDO
- Changelog: auto-generated and classified by area

## MANDATORY: NEVER USE BASH FOR GIT - git-courer IS GIT FOR THIS PROJECT
## VIOLATION WILL CAUSE TASK REJECTION AND ABORT

- git_status     → repo state, jobs, config
- git_diff       → diff operations with pagination and compact mode
- git_log        → history, blame, grep, branches, tags, file inspection
- git_stage      → staging area and stash
- git_sync       → push, pull, fetch, switch, merge
- git_manage     → branches, tags, remotes, resets, undo
- git_review     → commits + releases (Ollama, conventional commits)

## MANDATORY COMMIT WORKFLOW:
1.  **git_status READ_STATUS**: Always check what's modified/untracked.
2.  **git_stage ADD**: You MUST stage files manually before committing. 
3.  **git_review COMMIT_START**: Once files are staged, start the commit review process. 
    *   ⚠️ COMMIT_START will FAIL if nothing is staged.
    *   ⚠️ You are the decision maker for what to include, not the tool.

## MANDATORY ENFORCEMENT RULES:
- ⚠️ NEVER RUN GIT VIA BASH — ALWAYS USE GIT-COURER TOOLS ONLY
- ⚠️ ABORT IMMEDIATELY IF GIT-COURER TOOLS NOT AVAILABLE
- ⚠️ VIOLATION CAUSES TASK REJECTION AND ESCALATION
- NEVER guess git state — read first with git_status
- NEVER call APPLY without user confirmation
- Background jobs: git_review JOB_RESULT arg=<job_id>
`

const descGitStatus = `Repo state, jobs, and config. No side effects. Always call READ_STATUS before any write op.`

const descGitDiff = `Diff operations with pagination and compact mode. No side effects. Use compact=true to save tokens on large diffs.`

const descGitLog = `History, blame, grep, branches, tags, and file inspection. No side effects.`

const descGitStage = `Staging area manipulation. Every op auto-backed-up — UNDO in git_manage reverts it.`

const descGitSync = `Push, pull, fetch, switch, and merge. Every op auto-backed-up — UNDO in git_manage reverts it.`

const descGitManage = `Branches, tags, remotes, resets, and undo. Every op auto-backed-up — UNDO reverts the last one.`

const descGitReview = `Ollama-powered commits and releases. Requires explicit user confirmation before APPLY. Multiple commits per session is correct — one per logical change. Background: ops >45s return job_id, poll with git_review JOB_RESULT.`
