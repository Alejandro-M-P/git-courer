package mcp

const gitCourerSummary = `git-courer: Ollama-powered git assistant. Replaces bash git entirely.
Zero cloud tokens. Runs locally via Ollama.

- git_branch    → create, delete, rename, remote-delete branches
- git_tag       → create, delete, push, delete-remote tags
- git_stash     → save, pop, apply, drop, clear, show stashes
- git_backup    → undo operations, list/prune backups
- git_config    → read config, list models (read-only via MCP)
- git_sync      → push, pull, fetch, switch, merge
- git_stage     → add, rm, reset-soft (staging area)
- git_diff      → diff operations with pagination and compact mode
- git_log       → history, blame, grep, branches, tags, file inspection
- git_status    → repo state, current branch, what-changed
- git_review    → commits + releases (Ollama, conventional commits)

Workflow:
1. git_status READ_STATUS — check state
2. git_stage ADD paths=<files> — stage changes
3. git_review COMMIT_START — generate commit plan
4. git_review COMMIT_APPLY — execute after user confirmation

All write operations are auto-backed-up. UNDO via git_backup.
Background jobs >45s return job_id; poll with git_review JOB_RESULT.
`

const descGitBranch = `Manage branches. Commands: CREATE (name required), DELETE (name required, optional force), RENAME (name+new_name required), REMOTE_DELETE (name required). No 'arg' parameter — use explicit named params only.`

const descGitTag = `Manage tags. Commands: CREATE (name required), DELETE (name required), PUSH (name required), DELETE_REMOTE (name required). No 'arg' parameter — use explicit named params only.`

const descGitStash = `Manage stashes. Commands: SAVE (optional message), POP, APPLY (optional index), DROP (index required), CLEAR, SHOW. No 'arg' parameter — use explicit named params only.`

const descGitBackup = `Undo operations and manage backups. Commands: UNDO (restores last operation), LIST, PRUNE (optional days, default 7). No 'arg' parameter — use explicit named params only.`

const descGitConfig = `Read git-courer configuration. Commands: READ, LIST_MODELS. UPDATE_CONFIG is NOT available via MCP for security. No 'arg' parameter — use explicit named params only.`

const descGitStatus = `Show the working tree status including staged, unstaged, and untracked files. Commands: READ_STATUS, CURRENT_BRANCH, IS_REPO, REMOTE_INFO, WHAT_CHANGED. No side effects. No 'arg' parameter.`

const descGitDiff = `Diff operations with pagination and compact mode. Commands: READ_DIFF, READ_DIFF_STATS, READ_DIFF_STAGED, READ_DIFF_ALL, STASH_DIFF. No side effects. No 'arg' parameter — use 'path' instead.`

const descGitLog = `History, blame, grep, branches, tags, and file inspection. Commands: READ_LOG, READ_BRANCHES, READ_TAGS, BLAME, SHOW, REFLOG, MERGE_BASE, READ_SEARCH, CAT_FILE, LIST_TREE. No side effects. No 'arg' parameter — use 'revision', 'path', or 'pattern' instead.`

const descGitStage = `Staging area manipulation. Commands: ADD (paths required), RM (paths required), RESET_SOFT (commit required). Every op auto-backed-up — UNDO via git_backup. No 'arg' parameter — use 'paths' or 'commit' instead.`

const descGitSync = `Push, pull, fetch, switch, and merge. Commands: FETCH, PULL (optional remote), PUSH (optional remote), MERGE (branch required), SWITCH (branch required). Every op auto-backed-up — UNDO via git_backup. No 'arg' parameter — use 'branch' instead.`

const descGitReview = `Ollama-powered commits and releases. Requires explicit user confirmation before APPLY. Multiple commits per session is correct — one per logical change. Background: ops >45s return job_id, poll with git_review JOB_RESULT.`
