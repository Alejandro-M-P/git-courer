# Git Operations — git-courer MCP

## MANDATORY — No exceptions

You have access to `git_do`. This tool handles ALL git operations locally via Ollama.

### HARD RULES:

1. NEVER run `git` commands via bash/shell
2. NEVER read diffs, status, or logs before calling git_do
3. NEVER generate commit messages yourself
4. NEVER call git_do more than once per user request
5. NEVER plan or analyze before calling git_do

### WHEN USER ASKS FOR ANY GIT OPERATION:

Call git_do IMMEDIATELY with the user intent. Nothing else.

✅ CORRECT:
- User: "commit my changes" → You: git_do("commit my changes")
- User: "commit and push" → You: git_do("commit and push")

❌ WRONG:
- Multiple calls: git_do("commit") then git_do("push")
- Thinking first: [reads diff] [analyzes] [generates message] git_do(...)
- Bash git: `git status`, `git add`, `git commit -m "feat: ..."`

### ONE CALL. ONE INTENT. TRUST git-courer.

git-courer handles locally:
- Reading diffs
- Generating commit messages (Ollama)
- Detecting secrets
- Pushing to remote

Your job: pass the user intent to git_do. That is all.

## Why

- **Saves cloud tokens** — git operations are handled locally, zero tokens spent on diffs or commit messages
- **Automatic commit messages** — local AI analyzes your diff and generates conventional commits
- **Secret detection** — blocks commits with API keys, tokens, credentials
- **Smart batching** — groups related changes into logical commits

## How to use

| Instruction | What it does |
|---|---|
| `git_do("commit all changes and push")` | Commit + push |
| `git_do("show status")` | Shows working tree status |
| `git_do("show log")` | Shows recent commit history |
| `git_do("show diff")` | Shows unstaged changes |
| `git_do("create branch for auth feature")` | Creates and switches branch |
