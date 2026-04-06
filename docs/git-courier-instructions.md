# Git Operations — Use git-courer MCP

For ALL git operations, use the `git_do` MCP tool provided by git-courer.
NEVER run `git` commands directly via bash.

## 🚨 EXECUTE ONLY — NEVER THINK

When the user asks for ANY git operation, execute DIRECTLY with `git_do`.

- **DO NOT** analyze or plan before executing
- **DO NOT** read diffs first
- **DO NOT** check status before committing
- **git-courer + Ollama** handle everything locally

The user validates git operations through the UI — NOT through the orchestrator.

## Why

- **Saves cloud tokens** — git operations are handled locally, zero tokens spent on diffs or commit messages
- **Automatic commit messages** — local AI analyzes your diff and generates conventional commits
- **Secret detection** — blocks commits with API keys, tokens, credentials
- **Smart batching** — groups related changes into logical commits

## How to use

### Examples

| Instruction | What it does |
|---|---|
| `git_do("commit all changes and push")` | Commit + push |
| `git_do("show status")` | Shows working tree status |
| `git_do("show log")` | Shows recent commit history |
| `git_do("show diff")` | Shows unstaged changes |
| `git_do("create branch for auth feature")` | Creates and switches branch |

## Rules

1. **NEVER** use `git status`, `git log`, `git diff`, `git commit`, `git push`, etc. via bash
2. **ALWAYS** use `git_do("...")` — ONE call, ONE intent
3. **Pass user intent exactly** — do not translate or plan
