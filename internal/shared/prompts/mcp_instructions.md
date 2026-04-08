# Git Operations — git-courer MCP Instructions

## Overview

git-courer provides four MCP tools for git operations. Each serves a distinct purpose:

| Tool | Purpose | Confirmation Required |
|------|---------|----------------------|
| `git_read` | Read-only operations | No |
| `git_write` | Direct write operations | No |
| `git_write_review` | Write operations needing review | **Yes** |
| `git_write_commit` | Commit operations with preview | Conditional |

---

## git_read

Read-only operations that execute immediately without Ollama or blocking.

### Available Commands

| Command | Description |
|---------|-------------|
| `READ_STATUS` | Shows working tree status |
| `READ_DIFF` | Shows staged changes |
| `READ_DIFF_UNSTAGED` | Shows unstaged changes |
| `READ_LOG` | Shows recent commit history |
| `READ_BRANCHES` | Lists local and remote branches |

### Usage

```json
{
  "command": "READ_STATUS"
}
```

```json
{
  "command": "READ_LOG"
}
```

### Characteristics

- **No Ollama needed** — direct execution
- **No blocking** — instant response
- **No user confirmation** — executes immediately
- **Safe** — read-only, no side effects

---

## git_write

Direct write operations that execute immediately without preview or confirmation.

### Available Commands

| Command | Description |
|---------|-------------|
| `ADD` | Stage files |
| `CHECKOUT` | Checkout a branch or file |
| `SWITCH` | Switch branches |
| `STASH` | Stash changes |
| `STASH_POP` | Apply stashed changes |
| `PUSH` | Push to remote |
| `PULL` | Pull from remote |
| `FETCH` | Fetch from remote |
| `RM` | Remove files |

### Usage

```json
{
  "command": "ADD",
  "instruction": "add all changes"
}
```

```json
{
  "command": "PUSH",
  "instruction": "push to origin main"
}
```

### Characteristics

- **Direct execution** — no preview needed
- **Ollama interprets args** — natural language instruction is interpreted to decide exact command
- **No user confirmation** — executes immediately
- **Use for**: Safe, reversible operations that don't require review

### When to Use git_write vs git_write_review

| Use git_write | Use git_write_review |
|---------------|---------------------|
| `ADD` | `BRANCH_CREATE` |
| `CHECKOUT` (files) | `BRANCH_DELETE` |
| `SWITCH` (safe) | `TAG_CREATE` |
| `STASH` / `STASH_POP` | `TAG_DELETE` |
| `PUSH` / `PULL` / `FETCH` | `MERGE` |
| `RM` | `REBASE` |
| | `REBASE_CONTINUE` |
| | `REBASE_ABORT` |
| | `RESET_SOFT` |
| | `RESET_HARD` |
| | `CLEAN` |
| | `REMOTE_ADD` |
| | `REMOTE_REMOVE` |
| | `CHERRY_PICK` |
| | `REVERT` |
| | `INIT` |
| | `CLONE` |

---

## git_write_review

Write operations that require user confirmation before execution.

### Available Commands

| Command | Description |
|---------|-------------|
| `BRANCH_CREATE` | Create a new branch |
| `BRANCH_DELETE` | Delete a branch |
| `TAG_CREATE` | Create a tag |
| `TAG_DELETE` | Delete a tag |
| `MERGE` | Merge branches |
| `REBASE` | Rebase current branch |
| `REBASE_CONTINUE` | Continue rebase after resolving |
| `REBASE_ABORT` | Abort ongoing rebase |
| `RESET_SOFT` | Soft reset (keep changes staged) |
| `RESET_HARD` | Hard reset (discard all changes) |
| `CLEAN` | Clean untracked files |
| `REMOTE_ADD` | Add a remote |
| `REMOTE_REMOVE` | Remove a remote |
| `CHERRY_PICK` | Cherry-pick a commit |
| `REVERT` | Revert a commit |
| `INIT` | Initialize a repo |
| `CLONE` | Clone a repository |

### Usage

```json
{
  "command": "BRANCH_CREATE",
  "instruction": "create branch auth-refactor from main"
}
```

```json
{
  "command": "RESET_HARD",
  "instruction": "hard reset to origin/main"
}
```

### Characteristics

- **Ollama interprets args** — translates natural language to exact git command
- **Shows user operation** — displays what will be done before executing
- **Waits for confirmation** — user must confirm before execution
- **Use for**: Operations that modify history, delete data, or have significant impact

### Flow

1. User requests operation (e.g., "delete the old branch")
2. Ollama interprets args and determines exact command
3. **Show user** the planned operation: "This will delete branch `old-feature`"
4. **Wait for confirmation** from user
5. On confirmation, execute the operation

---

## git_write_commit

Commit operations controlled by `require_confirmation` config.

### Available Commands

| Command | Description |
|---------|-------------|
| `COMMIT_START` | Begin commit planning |
| `COMMIT_STATUS` | Check current plan status |
| `COMMIT_SUMMARY` | Get summary of planned commit |
| `COMMIT_APPLY` | Execute the planned commit |
| `COMMIT_ABORT` | Abort the commit plan |

### Modes

#### Confirmation Required (require_confirmation=true)

```
User: "commit my changes"
→ COMMIT_START
  → Config says require_confirmation=true
  → Stores plan
  → User confirms via git_write_review
  → COMMIT_APPLY
    → Executes the commit
```

1. `COMMIT_START` — analyzes changes, stores plan
2. `COMMIT_SUMMARY` — shows user the planned commit message and files
3. User reviews and confirms via git_write_review APPROVE
4. `COMMIT_APPLY` — executes the commit

### Example Flow

```json
{
  "command": "COMMIT_START",
  "instruction": "commit all changes"
}
```

```json
{
  "command": "COMMIT_SUMMARY"
}
```

```json
{
  "command": "COMMIT_APPLY"
}
```

---

## Lock File Mechanism

git-courer uses a lock file to prevent concurrent operations.

### Lock File Location

```
.gcourer/git-courer.lock
```

### Behavior

1. Before any write operation, check if lock file exists
2. **If locked**: Wait and retry (git-courer handles this automatically)
3. **If not locked**: Create lock file, proceed with operation
4. **After operation**: Release lock (delete lock file)

### Why

- Prevents race conditions when multiple agents operate simultaneously
- Ensures atomic operations
- git-courer manages this internally — no action needed from cloud AI

---

## TTL (Time-To-Live)

### Commit Plan Expiration

- **Plan expires after 10 minutes** (configurable)
- After TTL, the planned commit is automatically aborted
- User must start a new commit plan if TTL expires

### Configuration

TTL is configurable in `.gcourer/config.toml`:

```toml
[commit]
plan_ttl = 600  # seconds, default: 600 (10 minutes)
```

### Implications

- If user starts a commit preview but doesn't confirm within TTL, the plan expires
- Cloud AI should remind user to confirm within TTL window
- Expired plans require restarting with `COMMIT_START`

---

## Quick Reference

### Decision Tree

```
User wants to: READ git info (status, diff, log, branches)
    → git_read with appropriate command

User wants to: ADD, CHECKOUT, SWITCH, STASH, PUSH, PULL, FETCH, RM
    → git_write (direct, no confirmation)

User wants to: BRANCH_CREATE/DELETE, TAG, MERGE, REBASE, RESET, CLEAN, REMOTE, CHERRY_PICK, REVERT
    → git_write_review (show user, wait for confirmation)

User wants to: COMMIT
    → git_write_commit COMMIT_START
```

### Key Differences

| Tool | Ollama Involved | User Confirmation | Blocking |
|------|----------------|------------------|----------|
| `git_read` | No | No | No |
| `git_write` | Yes (interprets args) | No | No |
| `git_write_review` | Yes (interprets args) | **Yes** | Yes |
| `git_write_commit` | Yes (generates message) | Conditional | Yes (preview mode) |

---

## Important Rules

1. **NEVER** run `git` commands via bash — use the MCP tools exclusively
2. **NEVER** generate commit messages yourself — let Ollama via git_write_commit handle it
3. **ALWAYS** use `git_write_review` for destructive or history-modifying operations
4. **ALWAYS** show the user the operation before executing `git_write_review` commands
5. **RESPECT** the lock mechanism — concurrent operations are queued automatically
6. **REMIND** users about TTL for commit plans
