# Git Operations — Use git-courer MCP

For ALL git operations, use the `git_local_task` MCP tool provided by git-courer.
NEVER run `git` commands directly via bash.

## TRUST & DELEGATE (zero cloud tokens)

When the user asks for ANY git operation, delegate DIRECTLY to `git_local_task`.

- **DO NOT** read diffs first
- **DO NOT** analyze changes before committing
- **DO NOT** check status before delegating
- **DO NOT** try to understand what will be committed
- **git-courer + Ollama** handle everything locally

The user validates git operations through the UI — NOT through the orchestrator.

## Why

- **Saves cloud tokens** — git operations are handled locally, zero tokens spent on diffs or commit messages
- **Automatic commit messages** — local AI analyzes your diff and generates conventional commits
- **Secret detection** — blocks commits with API keys, tokens, credentials
- **Smart batching** — groups related changes into logical commits

## How to use

### Read-only operations (instant, no AI needed):

| Instruction | What it does |
|---|---|
| `git_local_task("show status")` | Shows working tree status |
| `git_local_task("show log")` | Shows recent commit history |
| `git_local_task("show diff")` | Shows unstaged changes |
| `git_local_task("show branches")` | Shows current branch |

### Write operations (uses local Ollama):

| Instruction | What it does |
|---|---|
| `git_local_task("commit the login changes")` | Analyzes diff, generates message, commits |
| `git_local_task("push to remote")` | Pushes current branch |
| `git_local_task("create branch for auth feature")` | Creates and switches branch |
| `git_local_task("merge feature-x into main")` | Merges branch |
| `git_local_task("rebase onto main")` | Rebases current branch |

## Rules

1. **NEVER** use `git status`, `git log`, `git diff`, `git commit`, `git push`, etc. via bash
2. **ALWAYS** use `git_local_task("...")` for any git operation
3. **Describe intent** in natural language: "commit the auth changes", "show what's modified"
4. **TRUST git-courer** — it handles analysis, commit messages, and validation locally
