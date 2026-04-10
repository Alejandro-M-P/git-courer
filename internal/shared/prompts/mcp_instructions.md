# Git-courer MCP Instructions

## Overview
git-courer is a local git assistant that uses Ollama (local LLM) for natural language git operations.

## IMPORTANT RULES

1. **NEVER generate commit messages yourself** - let git-courer do it via Ollama
2. **ALWAYS use subcommands** - not custom commands
3. **The review tool depends on config settings** - if preview.enabled=false, operations execute immediately

---

## Tool: git_read (Read-only operations)

**Subcommands:**
- `READ_STATUS` - Shows working tree status
- `READ_DIFF` - Shows unstaged changes  
- `READ_DIFF_STAGED` - Shows staged changes
- `READ_LOG` - Shows recent commit history
- `READ_BRANCHES` - Lists local and remote branches
- `READ_TAGS` - Lists all tags

**Usage:**
```json
{
  "name": "git_read",
  "arguments": {
    "command": "READ_STATUS"
  }
}
```

---

## Tool: git_write (Direct write operations - no LLM)

**Subcommands:**
- `ADD` - Stage files (use arg for specific paths)
- `RM` - Remove files
- `CHECKOUT` - Checkout a file or branch
- `SWITCH` - Switch branches
- `STASH` - Stash changes
- `STASH_POP` - Apply stashed changes
- `PUSH` - Push to remote
- `PULL` - Pull from remote
- `FETCH` - Fetch from remote

**Usage:**
```json
{
  "name": "git_write",
  "arguments": {
    "command": "ADD",
    "arg": "."
  }
}
```

---

## Tool: git_write_review (Workflow operations - LLM + optional confirmation)

**Subcommands:**
```
COMMIT:
  - COMMIT_START      → Prepare commit, returns preview
  - COMMIT_APPLY       → Execute the commit
  - COMMIT_ABORT       → Cancel the commit

BRANCH:
  - BRANCH_CREATE_START
  - BRANCH_CREATE_APPLY
  - BRANCH_CREATE_ABORT
  - BRANCH_DELETE_START
  - BRANCH_DELETE_APPLY
  - BRANCH_DELETE_ABORT
  - BRANCH_RENAME_START
  - BRANCH_RENAME_APPLY
  - BRANCH_RENAME_ABORT

MERGE/REBASE:
  - MERGE_START
  - MERGE_APPLY
  - MERGE_ABORT
  - REBASE_START
  - REBASE_APPLY
  - REBASE_ABORT
  - REBASE_CONTINUE
  - REBASE_ABORT

RESET:
  - RESET_HARD_START
  - RESET_HARD_APPLY
  - RESET_HARD_ABORT
  - RESET_SOFT_START
  - RESET_SOFT_APPLY
  - RESET_SOFT_ABORT

TAGS:
  - TAG_CREATE_START
  - TAG_CREATE_APPLY
  - TAG_CREATE_ABORT
  - TAG_DELETE_START
  - TAG_DELETE_APPLY
  - TAG_DELETE_ABORT

CHERRY_PICK:
  - CHERRY_PICK_START
  - CHERRY_PICK_APPLY
  - CHERRY_PICK_ABORT

REVERT:
  - REVERT_START
  - REVERT_APPLY
  - REVERT_ABORT

CLEAN:
  - CLEAN_START
  - CLEAN_APPLY
  - CLEAN_ABORT

REMOTE:
  - REMOTE_ADD_START
  - REMOTE_ADD_APPLY
  - REMOTE_ADD_ABORT
  - REMOTE_REMOVE_START
  - REMOTE_REMOVE_APPLY
  - REMOTE_REMOVE_ABORT

CLONE/INIT:
  - CLONE_START
  - CLONE_APPLY
  - CLONE_ABORT
  - INIT_START
  - INIT_APPLY
  - INIT_ABORT

UTILITY (no phase):
  - STATUS   → Show current plan status
  - SUMMARY  → Show git summary (status, branches, etc.)
```

**Usage:**
```json
{
  "name": "git_write_review",
  "arguments": {
    "command": "COMMIT_START",
    "instruction": "commit all changes"
  }
}
```

---

## Confirmation Behavior

The confirmation workflow depends on `preview.enabled` in config:

**If preview.enabled=true:**
```
START → returns {status: "pending_approval", preview: "..."}
APPLY → executes the operation
ABORT → cancels
```

**If preview.enabled=false:**
```
START → executes immediately, returns {status: "completed"}
```

---

## Common Mistakes to AVOID

1. ❌ Using COMMIT_SUMMARY - DOES NOT EXIST
2. ❌ Using COMMIT_APPLY without COMMIT_START first
3. ❌ Generating commit messages yourself - let Ollama do it
4. ❌ Using git commands directly via bash
5. ❌ Using wrong subcommand format

---

## Example: Commit Flow

User: "commit my changes"

**If preview.enabled=true:**
```
AI: git_write_review(command="COMMIT_START", instruction="commit all changes")

git-courer:
{
  "status": "pending_approval",
  "preview": "Commit: feat: add prompt templates",
  "files": ["prompts/prompts.go", "prompts/txt/branch_create.txt", ...],
  "messages": ["feat: add prompt templates"]
}

AI to user: "Voy a hacer commit: feat: add prompt templates. ¿Confirmas?"

User: "si"

AI: git_write_review(command="COMMIT_APPLY")

git-courer: "✓ Committed: feat: add prompt templates"
```

**If preview.enabled=false:**
```
AI: git_write_review(command="COMMIT_START", instruction="commit all changes")

git-courer: "✓ Committed: feat: add prompt templates" (executed immediately)
```

---

## Example: Branch Create

```
AI: git_write_review(command="BRANCH_CREATE_START", instruction="create branch for login feature")

git-courer: {status: "pending_approval", preview: "Create branch: feat/login"}

AI: git_write_review(command="BRANCH_CREATE_APPLY")

git-courer: "✓ Branch created: feat/login"
```