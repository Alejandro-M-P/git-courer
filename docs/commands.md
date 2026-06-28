# Commands Reference

Complete reference for all git-courer MCP tools and their arguments.

## Overview

git-courer exposes **13 MCP tools** for AI assistants. Every tool returns structured JSON — no unstructured text to parse, no pager hangs, no guessing.

| Tool | Category | Purpose |
|------|----------|---------|
| `status` | Unique / Read | Complete repository state in one call |
| `diff` | Unique / Read | AST-labeled diffs with semantic tags |
| `commit` | Unique / Write | 3-phase (AI) or 2-phase (Non-AI) commit pipeline |
| `pr-review` | Unique / Read | Pre-PR gate: runs tests, checks conflicts and divergence |
| `backup` | Unique / Utility | Manage backups and undo mutations |
| `session` | Unique / Write | Manage isolated sessions (git worktrees) for parallel agents |
| `branch` | Replacement / Write | Branch lifecycle and Switch (with auto-stash) |
| `stage` | Replacement / Write | Stage, unstage, or clean untracked files |
| `stash` | Replacement / Write | Save, pop, and show stashed changes |
| `history` | Replacement / Read | Commit log, reflog, and per-line blame |
| `rewrite` | Replacement / Write | Amend, revert, soft reset, and hard reset |
| `integrate` | Replacement / Write | Merge, rebase, cherry-pick, and abort/continue |
| `sync` | Replacement / Write | Push, pull, and fetch remote changes |

---

## Core Unique Tools

These tools represent capabilities that are either impossible or highly complex to achieve with raw CLI git commands.

### status
Returns the COMPLETE repo state in a single call, combining ahead/behind count, staged/unstaged/untracked files, stashes, conflict list, and user configuration.

* **Nudge**: Call this BEFORE any write operation to ensure you know the repo state.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `filter` | string | No | File path pattern to filter results |
| `limit` | number | No | Max file entries to return (pagination) |
| `offset` | number | No | Start index for pagination |

### diff
Returns AST-labeled diffs. Hunks are semantically annotated (e.g., `[NEW_FUNC]`, `[MOD_SIG ⚠BREAKING]`, `[DEPS]`, `[DEL]`) using Tree-sitter.

* **Nudge**: Call this before committing or pushing to review changes.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `target_paths` | string | No | Space-separated file paths to diff |
| `staged` | boolean | No | Show staged changes (`--cached`) |
| `branch` | string | No | Compare against a branch name (symmetric diff `branch...HEAD`) |
| `filter` | string | No | File path pattern to filter diff output |
| `limit` | number | No | Max diff lines to return |
| `offset` | number | No | Start line offset for pagination |
| `include_untracked` | boolean | No | Include untracked files in the diff output |

### commit
Handles the multi-phase commit pipeline. Supports both AI mode (uses a local/remote LLM to generate description and message) and Non-AI mode (offline mode with manual messages).

* **AI Mode Workflow**: 
  1. `command="PREVIEW"` + `why="..."` -> generates plan.
  2. Review plan with user.
  3. `command="APPLY"` + `job_id="..."` -> applies plan.
* **Non-AI Mode Workflow**: 
  1. `command="PREVIEW"` + `message="..."` -> prepares plan.
  2. `command="APPLY"` -> executes commit.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | **Yes** | `PREVIEW`, `APPLY`, or `STATUS` (poll job state) |
| `why` | string | **Yes** (AI mode) | The business/technical reason for this change. Describe the *why*, not what the code does. |
| `message` | string | **Yes** (Non-AI mode) | Manual commit message to use. |
| `target_paths` | string | No | Space-separated paths to stage before preview (use `.` for all) |
| `job_id` | string | No | Job ID for STATUS polling or plumbing-based APPLY execution |
| `push_after` | boolean | No | Auto-push commits after successful APPLY |
| `type` | string | No | Commit type prefix override (e.g., `feat`, `fix`, `chore`) |

### pr-review
A pre-PR validation gate that runs the project's `test_command`, detects merge conflicts with the target branch, shows diff stats, and checks branch divergence.

* **Nudge**: Call this BEFORE creating a PR. No exceptions.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `to` | string | No | Target branch to compare against (default: `main`) |

### backup
Manages automated git state backups. Every write operation automatically creates a backup.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | **Yes** | `RESTORE` (undo last mutation) or `LIST` (list backups) |
| `ref` | string | No | Target backup reference for RESTORE. Defaults to latest |

### session
Manages isolated git worktrees and branches to allow agents to work in parallel on the same repository without conflicts.

* **Workflow**: `start` (creates worktree) -> `select` (redirects all MCP tools) -> work -> `finish` (cleans up worktree, leaves branch alive).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | **Yes** | `start`, `finish`, `status`, `select`, `discard` |
| `agent` | string | No | Agent name (required for `start`) |
| `goal` | string | No | Goal description (required for `start`; used to name the branch) |
| `branch` | string | No | Custom branch name (optional; used for `start`) |
| `session_id` | string | No | Slug identifier (required for `finish`, `select`, `discard`; optional for `status`) |
| `confirmed` | boolean | No | Authorize destructive operations (required for `discard`) |

---

## Replacement Tools

These tools serve as structured replacements for standard git commands. They output clean, parseable JSON and build safety nets around mutations.

### branch
Branch management tool. Switch operation automatically stashes and unstashes a dirty working tree.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | **Yes** | `CREATE`, `DELETE`, `RENAME`, `SWITCH`, `LIST` |
| `branch_name` | string | No | Branch name (required for CREATE, DELETE, RENAME, SWITCH) |
| `new_branch_name` | string | No | New name (required for RENAME) |
| `force` | boolean | No | Force operation (caution: bypasses safety checks) |
| `confirmed` | boolean | No | Required for DELETE |
| `switch` | boolean | No | If true, CREATE switches to the new branch immediately |
| `filter` | string | No | Filter for LIST: `LOCAL`, `REMOTE`, or `ALL` |

### stage
Stages (add), unstages (restore), or cleans untracked files.

* **Nudge**: Do NOT use this for committing; use the `commit` tool.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | **Yes** | `RM` (remove from index), `RESTORE` (unstage), `CLEAN` (remove untracked) |
| `target_paths` | string | No | Space-separated file paths (required for RM and RESTORE) |
| `dry_run` | boolean | No | Preview files affected before executing |
| `confirmed` | boolean | No | Required for CLEAN |

### stash
Saves, pops, or shows stashes.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | **Yes** | `SAVE`, `POP`, `SHOW` |
| `commit_message` | string | No | Stash description (used with SAVE) |
| `stash_index` | string | No | Reference (e.g., `stash@{0}`) (used with POP and SHOW) |
| `include_untracked` | boolean | No | Stash untracked files alongside modifications |
| `diff` | boolean | No | SHOW returns diff content instead of a summary |

### history
Commit history, operation reflogs, and line-by-line file attribution.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | **Yes** | `LOG`, `REFLOG`, `BLAME` |
| `target_commit` | string | No | Starting commit hash or reference (defaults to `HEAD`) |
| `target_paths` | string | No | Paths to filter history, or target file to blame (required for `BLAME`) |
| `pattern` | string | No | Filter commit messages (LOG only) |
| `filter` | string | No | Filter file paths (LOG only) |
| `limit` | number | No | Max entries to return |
| `offset` | number | No | Pagination offset |

### rewrite
Destructive history modifications. Every rewrite creates a backup first.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | **Yes** | `AMEND` (amend last commit), `REVERT` (revert a commit), `SOFT` (soft reset HEAD), `HARD` (hard reset, discards all modifications) |
| `commit_message` | string | No | New commit message (AMEND only) |
| `target_paths` | string | No | Paths to include in amend (AMEND only) |
| `target_commit` | string | No | Commit hash (required for REVERT, SOFT, HARD) |
| `confirmed` | boolean | No | Required to execute (always required for HARD) |
| `dry_run` | boolean | No | Preview changes without modifying state |

### integrate
Executes integrations (merge, rebase, cherry-pick) and resolves conflicts.

* **Workflow on Conflict**: Returns `{status: "conflict", conflicted_files: [...]}`. Resolve files, `stage` them, then call `integrate command="CONTINUE"`.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | **Yes** | `MERGE`, `UPDATE` (rebase), `PICK` (cherry-pick), `CONTINUE`, `ABORT` |
| `branch_name` | string | No | Target branch name (required for MERGE and UPDATE) |
| `target_commit` | string | No | Commit hash (required for PICK) |
| `into_branch` | string | No | Switch to this branch before merging (MERGE only) |
| `delete_source` | boolean | No | Delete branch after successful merge (MERGE only) |
| `push_after` | boolean | No | Auto-push to remote after success (MERGE only) |
| `new_branch` | string | No | Create and switch to this branch after success (MERGE only) |
| `onto` | string | No | Base branch to rebase onto (UPDATE only) |

### sync
Pushes, pulls, or fetches from remotes.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | **Yes** | `PUSH` (send local commits), `PULL` (fetch and merge), `FETCH` (download changes, safest) |
| `confirmed` | boolean | No | Required for PUSH |
| `dry_run` | boolean | No | Preview changes before pushing/pulling |
| `remote_name` | string | No | Remote name (defaults to `origin`) |
| `branch` | string | No | Specific branch to sync |

---

## Error Codes

When a safety check blocks an operation, one of the following codes is returned:

| Code | Trigger | Solution |
|------|---------|----------|
| `SECRET_DETECTED` | Credentials or keys found in the diff | Remove secrets before staging or committing |
| `INTEGRITY ALERT` | Workspace files changed during review | Start the commit process again |
| `plan expired` | The PREVIEW commit plan timed out | Run `commit command="PREVIEW"` again |
| `confirmed required` | Mutating operation without confirmation | Pass `confirmed=true` after explaining to the user |
| `blocked` | Safety constraint failed | Review the hint in the JSON response |
