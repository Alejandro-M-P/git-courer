# Commands Reference

Complete reference for all git-courer MCP tools and their arguments.

## Overview

git-courer exposes **22 MCP tools** for AI assistants. Every tool returns structured JSON — no unstructured text to parse, no pager hangs, no guessing.

| Tool | Category | Purpose |
|------|----------|---------|
| `status` | Read | Complete repo state in one call |
| `diff` | Read | Annotated diff with AST labels |
| `commit` | Write | 3-phase commit pipeline |
| `amend` | Write | Fix the last commit safely |
| `revert` | Write | Undo a commit with backup |
| `cherry_pick` | Write | Apply a commit selectively |
| `branch` | Write | Branch lifecycle |
| `merge` | Write | Merge with structured conflicts |
| `rebase` | Write | Rebase with structured conflicts |
| `cherry_pick` | Write | Apply a commit selectively |
| `stage` | Write | Stage/unstage/clean |
| `reset` | Write | Undo commits at safety levels |
| `stash` | Write | Save/restore temporary state |
| `history` | Read | Commit history / reflog |
| `blame` | Read | Line-by-line attribution |
| `sync` | Write | Push/pull/fetch |
| `pr-review` | Read | Pre-PR gate: tests + conflicts + divergence |
| `config` | Utility | Read/update configuration |
| `backup` | Utility | Manage backups |
| `undo` | Utility | Restore latest backup |
| `remotes` | Utility | Add/remove remotes |
| `tag` | Write | Tag lifecycle |
| `commit-jobs` | Read | List active background jobs |

---

## Core Tools

### status
Returns COMPLETE repo state in ONE call — branch, ahead/behind, staged, unstaged, untracked, conflicted files, stash count, in-progress operations, last commit.

**Why the LLM cannot do this with raw git:** `git status` gives unstructured text. You'd need 5+ separate calls and text parsing to get the same data. One `status` call replaces all of them.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `filter` | string | No | File path pattern to filter results |
| `limit` | number | No | Max file entries to return |
| `offset` | number | No | Start index for pagination |

**Nudge:** Call this BEFORE any write operation — always know the repo state before mutating.

### diff
Annotated diff with AST labels in `@@` headers — see WHAT changed at symbol level, not raw lines.

**Why the LLM cannot do this with raw git:** `git diff` is unstructured text. You cannot tell whether a hunk is a new function, a breaking signature change, or just imports. git-courer annotates each hunk via tree-sitter. Paginated — no pager hangs.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `target_paths` | string | No | Space-separated file paths to diff |
| `staged` | boolean | No | Show staged changes (`--cached`) |
| `branch` | string | No | Compare against a branch |
| `filter` | string | No | File path pattern to filter output |
| `limit` | number | No | Max diff lines |
| `offset` | number | No | Start line offset |

**Nudge:** Call this before pushing or creating a PR to review what will go up.

### commit
The MOST IMPORTANT tool. 3-phase pipeline, 100% local.

**Why the LLM cannot do this with raw git:** You CANNOT parse an AST to classify changes. You CANNOT build a dependency graph across files. You CANNOT chunk a diff into atomic commits by dependency. You CANNOT classify hunks by semantic type (feat/fix/refactor/test). You CANNOT detect binary files before staging. You CANNOT know which files relate before committing. You CANNOT detect breaking changes from signature analysis. You CANNOT recover from merge/rebase with structured conflict data. You CANNOT know if hooks will fail before committing (no dry-run in raw git). git-courer does all of this.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | **Yes** | `PREVIEW`, `APPLY`, `ABORT`, `REGENERATE`, `STATUS` |
| `why` | string | No | Explanation to guide commit message generation |
| `job_id` | string | No | Job ID for plumbing path / STATUS polling |
| `feedback` | string | No | Feedback for `REGENERATE` |
| `push_after` | boolean | No | Auto-push after successful `APPLY` |
| `area_response` | string | No | JSON mapping dirs → area names |

**Pipeline:**
1. `PREVIEW` → DiffChunker parses AST, groups files by dependency graph, splits into atomic commits (max 12 files). Classifier labels each chunk (`feat`/`fix`/`refactor`/`BREAKING`).
   - **FAST:** Returns `{status:"pending", job_id, plan}` directly.
   - **SLOW (>45s):** Returns `{status:"processing", job_id}`. Poll `STATUS` with `job_id` until you get `status:"done"` or `status:"failed"`.
2. Review the plan. Show the user the proposed commits and ask "does this look good?".
3. `APPLY` → executes commits. Supports two paths:
   - With `job_id`: Plumbing path (creates atomic commit from PREVIEW tree snapshot via CommitTree + UpdateRef).
   - Without `job_id`: Legacy path (executes pending plan from ConfirmStore).

**Nudge:** If `PREVIEW` returns `"processing"`, do NOT block. Continue working and poll `STATUS` later.

### amend
Fix the last commit — change message, add files, or both.

**Why the LLM cannot do this safely with raw git:** `git commit --amend` has no safety net. Wrong message? Wrong files? Reflog is your only recovery. git-courer creates a backup first — `backup RESTORE` undoes it cleanly.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `commit_message` | string | No | New commit message |
| `target_paths` | string | No | Files to include in the amend |
| `confirmed` | boolean | **Yes** | Required to execute |
| `dry_run` | boolean | No | Preview impact without executing |

**Nudge:** Use `dry_run=true` to preview, then ask the user "does this look right?" before executing.

### revert
Revert a commit by creating a new commit that undoes it.

**Why the LLM cannot do this with raw git:** `git revert` drops you into conflict state with no structured output. You cannot detect which files conflicted without parsing text. git-courer returns clean JSON with conflict lists.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `target_commit` | string | **Yes** | Hash of the commit to revert |
| `confirmed` | boolean | **Yes** | Required to execute |
| `dry_run` | boolean | No | Preview what would be reverted |

**Nudge:** Use `dry_run=true` first to show the user what will be reverted, then confirm.

---

## Branching Tools

### branch
Branch lifecycle — CREATE, DELETE, RENAME, REMOTE_DELETE, SET_UPSTREAM, UNSET_UPSTREAM, SWITCH.

**Why the LLM cannot do this safely with raw git:** `git branch` can delete a branch with no recovery. Switching with dirty tree fails or forces manual stash. git-courer auto-stashes on switch — no manual stash needed.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | **Yes** | `CREATE`, `DELETE`, `RENAME`, `REMOTE_DELETE`, `SET_UPSTREAM`, `UNSET_UPSTREAM`, `SWITCH`, `LIST` |
| `branch_name` | string | No | Branch name |
| `new_branch_name` | string | No | New name for RENAME |
| `remote_name` | string | No | Remote name |
| `force` | boolean | No | Force operation |
| `confirmed` | boolean | No | Required for DELETE and REMOTE_DELETE |
| `filter` | string | No | Filter by location: `ALL`, `LOCAL`, `REMOTE` |

**Note:** DELETE and REMOTE_DELETE require `confirmed=true`. Ask the user "are you sure?" — these are NOT undoable via backup.

### merge
Merge a branch with structured conflict detection.

**Why the LLM cannot do this with raw git:** `git merge` on conflict dumps unstructured text. You cannot tell which files conflicted without parsing. git-courer returns `{status:"conflict", conflicted_files:[...], hint:"..."}` — file list included, no parsing needed.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `merge_branch_name` | string | **Yes** | Branch to merge |
| `into_branch` | string | No | Switch to this branch first |
| `abort` | boolean | No | Abort in-progress merge |
| `continue` | boolean | No | Continue after resolving conflicts |
| `skip` | boolean | No | Skip conflicting commit |
| `delete_source` | boolean | No | Delete source branch after success |
| `push_after` | boolean | No | Push after success |
| `new_branch` | string | No | Create and switch to new branch after success |

**Composition:** Use `into_branch`, `delete_source`, `push_after`, and `new_branch` to switch, merge, clean up, push, and pivot in ONE call.

**Nudge:** After resolving conflicts, call `diff` to verify, then `stage` the resolved files, then `merge continue=true`.

### rebase
Rebase with the same structured contract as merge.

**Why the LLM cannot do this with raw git:** Same problem as merge — raw git gives unstructured conflict output. git-courer gives you the file list.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `branch_name` | string | **Yes** | Target branch to rebase onto |
| `abort` | boolean | No | Abort in-progress rebase |
| `continue` | boolean | No | Continue after resolving conflicts |
| `skip` | boolean | No | Skip conflicting commit |
| `onto` | string | No | New base to transplant commits onto |

**Nudge:** After resolving conflicts, call `diff` to verify, then `stage` the resolved files, then `rebase continue=true`.

### cherry_pick
Apply a specific commit onto the current branch.

**Why the LLM cannot do this with raw git:** `git cherry-pick` drops you into unstructured conflict state. git-courer returns structured data so you know exactly what happened.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `target_commit` | string | **Yes** | Hash of the commit to cherry-pick |

### tag
Tag lifecycle — CREATE annotated tags, DELETE locally or remotely, PUSH tags.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | **Yes** | `CREATE`, `DELETE`, `PUSH`, `DELETE_REMOTE` |
| `tag_name` | string | **Yes** | Tag name |
| `commit_message` | string | No | Annotated tag message |
| `confirmed` | boolean | No | Required for DELETE and DELETE_REMOTE |

---

## Stage Tools

### stage
Stage, unstage, restore, or clean files.

**Why the LLM cannot do this safely with raw git:** `git add` silently stages binaries. git-courer catches them first and warns you. Auto-backup before every mutation.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | **Yes** | `ADD`, `RM`, `RESTORE`, `CLEAN` |
| `target_paths` | string | No | Space-separated file paths |
| `dry_run` | boolean | No | Preview impact |
| `confirmed` | boolean | No | Required for CLEAN |

**Nudge:** Do NOT use for committing — use `commit` instead.

### reset
Undo commits at different safety levels.

**Why the LLM cannot do this safely with raw git:** `git reset --hard` is permanent and git does not warn you. git-courer HARD requires `confirmed=true` and creates a backup first.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | **Yes** | `SOFT`, `MIXED`, `HARD` |
| `target_commit` | string | **Yes** | Commit hash to reset to |
| `confirmed` | boolean | No | Required for HARD |
| `dry_run` | boolean | No | Preview impact |

**Nudge:** `SOFT` moves HEAD only (safest). `MIXED` unstages too. `HARD` discards everything — ask the user first.

### stash
Save, restore, or inspect stashed changes.

**Why the LLM cannot do this with raw git:** `git stash` is a pile of unnamed entries. git-courer `SHOW` lets you inspect before restoring. No accidental DROP/CLEAR — stashes are auto-managed.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | **Yes** | `SAVE`, `POP`, `SHOW` |
| `commit_message` | string | No | Description for the stash entry |
| `stash_index` | string | No | Stash reference like `stash@{0}` |
| `include_untracked` | boolean | No | Also stash untracked files |
| `diff` | boolean | No | SHOW returns diff content instead of summary |

---

## History Tools

### history
Show commit history (LOG) or reflog (REFLOG) with pagination and filtering.

**Why the LLM cannot do this with raw git:** `git log` hangs on large repos. git-courer never hangs — paginated JSON with offset/limit. No unstructured text to parse.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | **Yes** | `LOG` or `REFLOG` |
| `target_commit` | string | No | Starting commit hash or ref |
| `target_paths` | string | No | Filter history by these files |
| `pattern` | string | No | Filter commit messages |
| `filter` | string | No | Filter by path pattern |
| `limit` | number | No | Max entries |
| `offset` | number | No | Start index |

### blame
Line-by-line attribution for a specific file.

**Why the LLM cannot do this with raw git:** `git blame` spews unstructured text. git-courer returns JSON — no parsing needed.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `target_paths` | string | **Yes** | Path to the file to blame |
| `limit` | number | No | Max blame entries |
| `offset` | number | No | Start line offset |

---

## Sync Tools

### sync
Push, pull, or fetch from remote.

**Why the LLM cannot do this safely with raw git:** `git push` is IRREVERSIBLE. If you push breaking changes, you need `--force`. git-courer requires `confirmed=true` and offers `dry_run` preview. `PULL` creates a backup before merging.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | **Yes** | `PUSH`, `PULL`, `FETCH` |
| `confirmed` | boolean | No | Required for PUSH |
| `dry_run` | boolean | No | Preview impact |
| `remote_name` | string | No | Remote name (default: `origin`) |
| `branch` | string | No | Specific branch to push/pull |

**Nudge:** ALWAYS call `diff` before pushing. ALWAYS call `pr-review` before creating/updating a PR.

### remotes
Add or remove remote URLs.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | **Yes** | `ADD` or `REMOVE` |
| `remote_name` | string | **Yes** | Remote name |
| `url` | string | No | Remote URL (required for ADD) |
| `confirmed` | boolean | No | Required for REMOVE |

---

## PR Review

### pr-review
Pre-PR gate: runs tests, detects conflicts, shows diff stats, and checks branch divergence.

**Why the LLM cannot do this with raw git:** Raw `git diff main..feature` + `go test` gives unstructured text you have to parse. git-courer gives structured analysis in one call — you know if the PR is safe to open.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `to` | string | No | Target branch (default: `main`) |

**Returns 5 possible states:**
- `no_test_command` → first run hint: configure with `config SET_TEST_COMMAND "make test-ci"`
- `test_fail` → only failing tests shown with truncated output
- `conflict` → conflicted files + AST-annotated conflict hunks
- `test_ok` → all green, ready for PR
- `error` → unexpected failure

**Nudge:** Call this BEFORE creating ANY PR. No exceptions.

---

## Utility Tools

### config
Read or update project configuration.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | No | `GET`, `SET_TEST_COMMAND`, `SET_USER_NAME`, `SET_USER_EMAIL`, `SET_SIGNING_KEY` |
| `test_command` | string | No | Test command to save |
| `value` | string | No | Value for SET_USER_NAME, SET_USER_EMAIL, SET_SIGNING_KEY |

**Note:** `SET_TEST_COMMAND` saves to `.git-courer/config.json` — per-project, committable, shared by the team.

### backup
Manage git backups. Every write operation auto-creates one.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | **Yes** | `CREATE`, `DELETE`, `RESTORE`, `LIST` |
| `ref` | string | No | Backup reference for RESTORE/DELETE |
| `confirmed` | boolean | No | Required for DELETE |

### undo
Undo the most recent destructive git operation by restoring the latest backup.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `ref` | string | No | Specific backup ref (default: latest) |

**When to use:** Immediately after a mistaken amend, merge, rebase, or revert.

---

## Background Jobs

git-courer uses a non-blocking background job model for operations that may take time.

### How it works

When you call `commit PREVIEW`, the server may return immediately or start a background job:

- **FAST path:** Returns `{status:"pending", job_id, plan}` — the plan is ready immediately.
- **SLOW path (>45s):** Returns `{status:"processing", job_id}`. The agent must call `commit STATUS` with that `job_id` to poll.
  - When STATUS returns `{status:"done"}`, the plan is ready.
  - When STATUS returns `{status:"failed"}`, the job errored.

The agent **does NOT block** waiting for the job. It continues working and polls STATUS later.

Every successful `APPLY` captures the commit to `.git-courer/commits.json` (or `.git-courer/branches/<branch>/commits.json` when branch-scoped). These captured commits are later consumed by `git-courer release` (CLI) to generate the changelog — grouped by `areas` from `.git-courer/config.json`. If the store is empty, release falls back to `git log` since the last tag.

**Why capture matters:** Git history is frequently rewritten — PR squashes, rebases, force-pushes — destroying the real commit narrative. The CommitStore preserves every commit message as it was written, independently of what happens on the remote. Your release changelog survives `git log` being flattened into a single squashed commit. This is local documentation that outlives git history rewriting.

### commit-jobs
List active commit pipeline jobs — their status, commit message, and tree hash. Read-only tool for inspecting background jobs.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| (none) | — | — | No parameters |

**Why the LLM needs this:** Without `commit-jobs`, you cannot know which background jobs are running, which have completed, or which have failed. You'd be polling blind.

---

## Response Format

All responses use descriptive keys for better AI reasoning and human readability.

| Key | Meaning |
|-----|---------|
| `status` | Operation status (`ok`, `error`, `conflict`, `processing`, `done`, `pending`) |
| `job_id` | Background job identifier for polling |
| `message` | Commit or status message |
| `hash` | Commit hash |
| `author` | Author name |
| `date` | Operation date |
| `path` | File path |
| `files` | List of files/stats |
| `conflicted_files` | Files with merge/rebase conflicts |
| `hint` | Human-readable guidance for conflicts/errors |
| `next_offset` | Pagination token |
| `truncated` | Result limit reached |

---

## Error Handling

| Error | Cause | Solution |
|-------|-------|----------|
| `SECRET_DETECTED` | Credentials in diff | Remove secrets before committing |
| `INTEGRITY ALERT` | Files changed during review | Run START again |
| `plan expired` | Operation timeout | Run START again |
| `confirmed required` | Destructive operation without confirmation | Set `confirmed=true` after reviewing |
| `blocked` | Operation blocked by safety check | Review the hint and retry |
