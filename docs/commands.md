# Commands Reference

Complete reference for all git-courer MCP tool commands and arguments.

## Overview

git-courer exposes **3 MCP tools** for AI assistants:

| Tool | Purpose | Workflow |
|------|---------|----------|
| `git_read` | Read-only git operations | Direct execution |
| `git_write` | Direct write operations | Direct execution |
| `git_write_review` | Confirmed write operations (LLM-assisted) | 3-phase protocol |

---

## git_read

Read-only git operations. Returns structured JSON.

### Arguments

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | **Yes** | The read operation to execute |
| `path` | string | No | File or directory path |
| `hash` | string | No | Commit hash |
| `revision` | string | No | Git ref (branch, tag, HEAD~N) |
| `pattern` | string | No | Search regex or filter pattern |
| `limit` | number | No | Max items to return |
| `offset` | number | No | Start index (0-based) |
| `filter` | string | No | Narrow results by pattern (e.g., branch name) |
| `context` | number | No | Context lines for READ_SEARCH (-C) |
| `before` | number | No | Lines before match for READ_SEARCH (-B) |
| `after` | number | No | Lines after match for READ_SEARCH (-A) |
| `compact` | boolean | No | Only show + and - lines in diffs |
| `llm` | boolean | No | Use LLM for semantic summaries (WHAT_CHANGED) |
| `arg` | string | No | **Legacy/Fallback**: Generic argument |


### Commands

#### READ_STATUS
Returns the current repository status.
- Default limit: 100 files.

#### READ_DIFF
Returns unstaged diff. Use `path` to isolate a file or `revision` for range.

#### READ_DIFF_STAGED
Returns staged diff (`--cached`).

#### READ_DIFF_ALL
Returns combined staged + unstaged changes.

#### READ_LOG
Returns commit history.
- `revision`: Branch or range (e.g., `main..feat`)
- `pattern`: Filter messages using `git log --grep`
- `filter`: Client-side search in results

#### READ_BRANCHES
Lists local branches. Use `pattern` as glob.

#### READ_TAGS
Lists tags. Use `pattern` as glob.

#### CURRENT_BRANCH
Returns current branch name.

#### IS_REPO
Returns whether current directory is a git repo.

#### REMOTE_BRANCH_LIST
Lists remote branches.

#### REMOTE_TAG_LIST
Lists remote tags.

#### REMOTE_INFO
Returns configured remote URLs (`git remote -v`).

#### LIST_BACKUPS
Lists available git-courer security backups.

#### WHAT_CHANGED
Summary of changes.
- `filter`: "all", "staged", or "unstaged"
- `llm`: Set `true` for a semantic summary.

#### READ_DIFF_STAT
Stats only (insertion/deletion counts). Use `path` for a specific file.

#### READ_SEARCH
Search for a pattern using `git grep`.
- `pattern`: The string or regex to find.

#### BLAME
Per-line authorship.
- `path`: Target file.
- `limit`/`offset`: Paginate lines.

#### SHOW
Single commit details.
- `hash`: Commit hash.
- **Output includes `fl` (file list)** of affected files.

#### CAT_FILE
Read file content at a specific revision.
- `revision`: Git ref (default: HEAD).
- `path`: File path.

#### LIST_TREE
List files in a revision (`ls-tree`).
- `revision`: Git ref (default: HEAD).
- `path`: Directory path.
- `recursive`: Set `true` to list nested files.

#### REFLOG
Recover lost commits / dangling refs (paginated).

#### STASH_LIST
List stashed changes (paginated).

#### MERGE_BASE
Common ancestor of two branches.
- `revision`: Comma-separated refs (e.g., "main,develop").

---

## git_write

Direct write git operations (no LLM involved). Returns structured JSON.

### Arguments

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | **Yes** | The write operation to execute |
| `paths` | string | No | Comma-separated file paths (ADD, RM) |
| `branch` | string | No | Branch name (SWITCH, MERGE, PULL, PUSH) |
| `remote` | string | No | Remote name (PUSH, PULL) |
| `message` | string | No | Custom message (STASH) |
| `commit` | string | No | Commit ref (RESET_SOFT) |
| `name` | string | No | Branch/Tag name or rename pattern `old:new` |
| `days` | number | No | Retention for PRUNE_BACKUPS (default: 7) |
| `arg` | string | No | **Legacy/Fallback**: Generic argument |

### Commands

#### ADD
Stage files. `paths`: comma-separated paths.

#### SWITCH
Switch branch. `branch`: branch name.

#### STASH
Stash current changes. `message`: optional description.

#### UNDO
Reverts the last direct write operation using the auto-backup system.

#### PRUNE_BACKUPS
Deletes old security backups. `days`: retention period.

#### STASH_POP
Restore stashed changes.

#### PUSH
Push to remote.

#### PULL
Pull from remote. `branch`: optional remote/branch.

#### FETCH
Fetch from remote.

#### RM
Remove files. `paths`: comma-separated paths.

#### RESET_SOFT
Soft reset. `commit`: hash or `HEAD~N`.

#### RENAME_BRANCH
Rename branch. `name`: `old_name:new_name`.

#### BRANCH_CREATE
Create branch. `name`: branch name.

#### BRANCH_DELETE
Delete local branch. `name`: branch name.

#### REMOTE_BRANCH_DELETE
Delete remote branch. `name`: `origin/branch`.

#### REMOTE_TAG_DELETE
Delete remote tag. `name`: tag name.

#### TAG_CREATE
Create tag. `name`: tag name.

#### TAG_DELETE
Delete local tag. `name`: tag name.

#### TAG_PUSH
Push tag to remote. `name`: tag name.

#### TAG_DELETE_REMOTE
Delete tag from remote. `name`: tag name.

#### MERGE
Merge branch. `branch`: branch name.

---

## git_write_review

Confirmed write operations with 3-phase protocol. Uses LLM for commit messages, branch naming, etc.

### Arguments

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | **Yes** | Operation + phase (e.g., `COMMIT_START`) |
| `instruction` | string | For START | Natural language instruction |
| `branch` | string | For branch ops | Branch name |
| `preview` | boolean | No | Show preview before executing (default: true) |
| `feedback` | string | For REGENERATE | Feedback for message regeneration |

### 3-Phase Protocol

1. **START** (`{OP}_START`): Analyzes context and shows a preview.
2. **APPLY** (`{OP}_APPLY`): Executes the operation after user confirmation.
3. **ABORT** (`{OP}_ABORT`): Cancels the pending operation.

### Operations

- **COMMIT**: AI-generated conventional commits.
- **RELEASE**: Automated semver releases and changelogs.
- **BRANCH_CREATE**: Suggested branch names based on intent.
- **BRANCH_DELETE**: Safe branch deletion.
- **MERGE**: Guided merge with conflict detection.

---

## Response Format

All responses use **descriptive keys** for better AI reasoning and human readability.

| Key | Meaning |
|-----|---------|
| `hash` | Commit hash |
| `message` | Commit/Status message |
| `author` | Author name |
| `date` | Operation date |
| `path` | File path |
| `files` | List of files/stats |
| `success` | Boolean status |
| `next_offset` | Pagination token |
| `truncated` | Result limit reached |

---

## Error Handling

| Error | Cause | Solution |
|-------|-------|----------|
| `SECRET_DETECTED` | Credentials in diff | Remove secrets before committing |
| `INTEGRITY ALERT` | Files changed during review | Run START again |
| `plan expired` | Operation timeout | Run START again |
