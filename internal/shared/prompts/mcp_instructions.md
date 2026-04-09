# Git Operations — git-courer MCP Instructions

## Overview

git-courer provides three MCP tools for git operations. Each serves a distinct purpose:

| Tool | Purpose | Ollama | Confirmation |
|------|---------|--------|-------------|
| `git_read` | Read-only operations | No | No |
| `git_write` | Direct writes (no review needed) | Yes (interprets args) | No |
| `git_write_review` | All confirmable operations | Yes | Conditional |

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
| `READ_BRANCHES` | Lists local branches |
| `READ_TAGS` | Lists tags |

### Usage

```json
{"command": "READ_STATUS"}
```

```json
{"command": "READ_LOG"}
```

### Characteristics

- **No Ollama needed** — direct execution
- **No blocking** — instant response
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
{"command": "ADD", "instruction": "add all changes"}
```

```json
{"command": "PUSH", "instruction": "push to origin main"}
```

### Characteristics

- **Ollama interprets args** — natural language is converted to exact git params
- **Direct execution** — no preview, no lock
- **Use for**: reversible operations that don't require review

---

## git_write_review

ALL confirmable operations — including COMMIT — go through this tool.
Ollama interprets natural language, shows a preview, and optionally waits for user confirmation.
Whether confirmation is required depends on the user's `preview.operations` config.

### Three-phase cycle

```
<OP>_START   → Ollama interprets instruction → stores plan → returns preview
<OP>_APPLY   → executes the planned operation
<OP>_ABORT   → cancels the plan
```

### All Operations

| Operation | START command | APPLY command | ABORT command |
|-----------|--------------|--------------|--------------|
| Commit | `COMMIT_START` | `COMMIT_APPLY` | `COMMIT_ABORT` |
| Create branch | `BRANCH_CREATE_START` | `BRANCH_CREATE_APPLY` | `BRANCH_CREATE_ABORT` |
| Delete branch | `BRANCH_DELETE_START` | `BRANCH_DELETE_APPLY` | `BRANCH_DELETE_ABORT` |
| Rename branch | `BRANCH_RENAME_START` | `BRANCH_RENAME_APPLY` | `BRANCH_RENAME_ABORT` |
| Create tag | `TAG_CREATE_START` | `TAG_CREATE_APPLY` | `TAG_CREATE_ABORT` |
| Delete tag | `TAG_DELETE_START` | `TAG_DELETE_APPLY` | `TAG_DELETE_ABORT` |
| Merge | `MERGE_START` | `MERGE_APPLY` | `MERGE_ABORT` |
| Rebase | `REBASE_START` | `REBASE_APPLY` | `REBASE_ABORT` |
| Continue rebase | `REBASE_CONTINUE_START` | `REBASE_CONTINUE_APPLY` | `REBASE_CONTINUE_ABORT` |
| Abort rebase | `REBASE_ABORT_START` | `REBASE_ABORT_APPLY` | — |
| Soft reset | `RESET_SOFT_START` | `RESET_SOFT_APPLY` | `RESET_SOFT_ABORT` |
| Hard reset | `RESET_HARD_START` | `RESET_HARD_APPLY` | `RESET_HARD_ABORT` |
| Clean | `CLEAN_START` | `CLEAN_APPLY` | `CLEAN_ABORT` |
| Add remote | `REMOTE_ADD_START` | `REMOTE_ADD_APPLY` | `REMOTE_ADD_ABORT` |
| Remove remote | `REMOTE_REMOVE_START` | `REMOTE_REMOVE_APPLY` | `REMOTE_REMOVE_ABORT` |
| Cherry-pick | `CHERRY_PICK_START` | `CHERRY_PICK_APPLY` | `CHERRY_PICK_ABORT` |
| Revert | `REVERT_START` | `REVERT_APPLY` | `REVERT_ABORT` |
| Init | `INIT_START` | `INIT_APPLY` | `INIT_ABORT` |
| Clone | `CLONE_START` | `CLONE_APPLY` | `CLONE_ABORT` |

Utility commands (no phase suffix):

| Command | Description |
|---------|-------------|
| `STATUS` | Show current plan status and lock state |
| `SUMMARY` | Get human-readable summary of the pending plan |

### Usage — Branch

```json
{"command": "BRANCH_CREATE_START", "instruction": "create a branch for the login feature"}
```

Response:
```json
{
  "status": "pending_approval",
  "preview": "Create branch: feat/login",
  "args": {"branch": "feat/login"}
}
```

```json
{"command": "BRANCH_CREATE_APPLY"}
```

### Usage — Commit

```json
{"command": "COMMIT_START", "instruction": "commit all staged changes"}
```

Response:
```json
{
  "status": "pending_approval",
  "preview": "feat: add JWT authentication middleware",
  "messages": ["feat: add JWT authentication middleware"],
  "files": ["internal/auth/jwt.go", "internal/auth/middleware.go"]
}
```

```json
{"command": "COMMIT_APPLY"}
```

### Usage — Hard Reset

```json
{"command": "RESET_HARD_START", "instruction": "hard reset to origin/main"}
```

```json
{"command": "RESET_HARD_APPLY"}
```

### Characteristics

- **Ollama interprets ALL args** — natural language → exact git params per operation
- **Plan stored on disk** — survives tool call boundaries
- **TTL 10 minutes** — expired plans require re-running START
- **Commit is just another operation** — same cycle, extra fields (messages, files)

---

## Lock File Mechanism

git-courer uses a lock file to prevent concurrent operations.

**Location:** `.gcourer/gcourer_plan.lock`

git-courer manages this automatically — no action needed from the AI.

---

## Quick Reference

### Decision Tree

```
User wants to: READ git info (status, diff, log, branches, tags)
    → git_read

User wants to: ADD, CHECKOUT, SWITCH, STASH, PUSH, PULL, FETCH, RM
    → git_write (direct, no confirmation)

User wants to: COMMIT or any other confirmable operation
    → git_write_review with <OP>_START → <OP>_APPLY
```

### Key Differences

| Tool | Ollama | User Confirmation | Blocking |
|------|--------|------------------|----------|
| `git_read` | No | No | No |
| `git_write` | Yes (interprets args) | No | No |
| `git_write_review` | Yes (interprets args + generates messages for commits) | Conditional | Yes (when preview.operations[op]=true) |

---

## Important Rules

1. **NEVER** run `git` commands via bash — use the MCP tools exclusively
2. **NEVER** generate commit messages yourself — Ollama via `COMMIT_START` handles it
3. **ALWAYS** use `git_write_review` for commits, destructive ops, and history-modifying ops
4. **RESPECT** the TTL — if 10 minutes pass between START and APPLY, re-run START
