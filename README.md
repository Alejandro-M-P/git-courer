<!-- markdownlint-disable MD041 -->

<h1 align="center">git-courer</h1>

<p align="center">
  <strong>The Local Git Specialist</strong> — Zero tokens for git operations
</p>

<p align="center">
  <a href="https://github.com/Alejandro-M-P/git-courer/releases/latest">
    <img src="https://img.shields.io/github/v/release/Alejandro-M-P/git-courer?color=%2300BFFF&label=latest" alt="Release">
  </a>
  <a href="https://github.com/Alejandro-M-P/git-courer/actions">
    <img src="https://img.shields.io/github/actions/workflow/status/Alejandro-M-P/git-courer/test.yml?branch=main" alt="Build">
  </a>
  <a href="https://goreportcard.com/report/github.com/Alejandro-M-P/git-courer">
    <img src="https://goreportcard.com/badge/github.com/Alejandro-M-P/git-courer" alt="Go Report">
  </a>
  <a href="https://github.com/Alejandro-M-P/git-courer/blob/main/LICENSE">
    <img src="https://img.shields.io/github/license/Alejandro-M-P/git-courer" alt="MIT License">
  </a>
  <img src="https://img.shields.io/badge/status-beta-yellow" alt="Status: Beta">
</p>

---



---

## ⚠️ Status: Beta

git-courer is in **active development** (v0.1.0-beta). Some features may change.

---

## What is it?

git-courer is a local MCP server that handles all git operations for cloud AI agents. Instead of wasting tokens on reading diffs, generating commit messages, and analyzing changes, the AI delegates to git-courer which runs 100% locally.

**Zero tokens spent on git operations.**

### The Problem

Every time a cloud AI agent needs to do git work, it wastes tokens:

| Operation | Tokens Wasted |
|-----------|---------------|
| Reading diffs | ~500-2000 per operation |
| Generating commit messages | ~300-1000 per commit |
| Analyzing changed files | ~200-800 per operation |
| Generating branch names | ~100-300 per branch |

### The Solution

```
Cloud AI: "Make a commit with the changes"
         ↓
git-courer: (reads diff, detects secrets, generates message, commit + push)
         ↓
Result: "✓ Commit created: feat: add auth"
```

### Token Savings

| Scenario | Without git-courer | With git-courer |
|----------|-------------------|-----------------|
| Daily commit | ~2000 tokens | ~50 tokens |
| 20 commits/day | 40,000 tokens | 1,000 tokens |
| Monthly | 1,200,000 tokens | 30,000 tokens |

---

## Installation

### Quick Install

```bash
curl -fsSL https://raw.githubusercontent.com/Alejandro-M-P/git-courer/main/install.sh | sh
```

### Manual Install

**Linux (x86_64):**
```bash
curl -fsSL https://github.com/Alejandro-M-P/git-courer/releases/latest/download/git-courer-linux-amd64 -o git-courer
chmod +x git-courer
sudo mv git-courer /usr/local/bin/
```

**Linux (ARM64):**
```bash
curl -fsSL https://github.com/Alejandro-M-P/git-courer/releases/latest/download/git-courer-linux-arm64 -o git-courer
chmod +x git-courer
sudo mv git-courer /usr/local/bin/
```

**macOS (Intel):**
```bash
curl -fsSL https://github.com/Alejandro-M-P/git-courer/releases/latest/download/git-courer-darwin-amd64 -o git-courer
chmod +x git-courer
sudo mv git-courer /usr/local/bin/
```

**macOS (Apple Silicon):**
```bash
curl -fsSL https://github.com/Alejandro-M-P/git-courer/releases/latest/download/git-courer-darwin-arm64 -o git-courer
chmod +x git-courer
sudo mv git-courer /usr/local/bin/
```

**Windows:**
```powershell
irm https://github.com/Alejandro-M-P/git-courer/releases/latest/download/git-courer-windows-amd64.exe -o git-courer.exe
.\git-courer.exe
```

### Requirements

- **Go** 1.24+ (for development)
- **Ollama** (optional, for AI-powered commit messages)

### Configure your AI tool

#### Opencode

```json
// opencode.json
{
  "mcpServers": {
    "git-courer": {
      "type": "stdio",
      "command": "git-courer"
    }
  }
}
```

#### Claude Code

```json
// .claude/settings.json
{
  "mcpServers": {
    "git-courer": {
      "command": "git-courer"
    }
  }
}
```

#### Cursor

```json
// .cursor/mcp.json
{
  "mcpServers": {
    "git-courer": {
      "command": "git-courer"
    }
  }
}
```

#### Windsurf

```json
// .windsurf/mcp.json
{
  "mcpServers": {
    "git-courer": {
      "command": "git-courer"
    }
  }
}
```

### Run

```bash
git-courer
```

---

## How It Works

### Operation Flow

Every confirmable operation (commit, branch, merge, reset...) uses the same cycle:

```
1. START  — natural language instruction → Ollama interprets → stores plan → returns preview
2. APPLY  — executes the planned operation (skipped if preview disabled in config)
3. ABORT  — cancels the plan at any point

Example: COMMIT_START → COMMIT_APPLY
Example: BRANCH_CREATE_START → BRANCH_CREATE_APPLY
```

#### Commit internals

```
COMMIT_START:
  1. Security Check (5 layers)
     ├─ Magic bytes → binary files blocked
     ├─ Folder blacklist → node_modules, .git blocked
     ├─ Name blacklist → .env, credentials blocked
     ├─ Regex scan → API keys, passwords detected
     └─ LLM verification → confirms findings
  2. Chunk diff → split large diffs into pieces
  3. Generate messages → Ollama writes commit messages
  4. Store plan → returns preview to the AI

COMMIT_APPLY:
  5. Execute with rollback → on failure, reset all commits
```

### MCP Tools

| Tool | Description |
|------|-------------|
| `git_read` | Read-only: status, diff, log, branches, tags |
| `git_write` | Direct write: add, checkout, stash, push, pull |
| `git_write_review` | All confirmable ops via START/APPLY/ABORT cycle: commit, branch, merge, reset, and more |

### Crash Recovery

- Plan stored in `.gcourer/gcourer_plan.json` with 10-minute TTL
- Lock file `.gcourer/gcourer_plan.lock` prevents concurrent operations
- Automatic cleanup of stale locks on next operation

---

## Practical Examples

### Commit (CORRECT FLOW)

```
AI: git_write_review(command="COMMIT_START", instruction="commit all changes")
↓
git-courer:
  1. Detects secrets (.env not staged)
  2. Chunks diff intelligently (by related files)
  3. Generates message via Ollama: "feat: add prompt templates"
  4. If preview.enabled=true → returns pending_approval
  5. If preview.enabled=false → executes immediately
↓
AI shows preview to user → user confirms
↓
AI: git_write_review(command="COMMIT_APPLY")
↓
"✓ Committed: feat: add prompt templates"
```

### Create Branch

```
AI: git_write_review(command="BRANCH_CREATE_START", instruction="branch for login feature")
↓
git-courer (Ollama): "feat/login"
↓
{status: "pending_approval", preview: "Create branch: feat/login"}
↓
AI: git_write_review(command="BRANCH_CREATE_APPLY")
↓
"✓ Branch created: feat/login"
```

### Hard Reset (no confirmation in config)

```
AI: git_write_review(command="RESET_HARD_START", instruction="reset to origin/main")
↓
git-courer (Ollama): {commit: "origin/main"}
↓
{status: "executed", preview: "Hard reset to origin/main"}  ← executed immediately
```

---

## FAQ

### What if Ollama is not available?

Works perfectly without Ollama. Basic git operations always work. Commit messages will be generic ("update files") instead of AI-generated.

### Is it safe?

Yes. git-courer:
- Never stages sensitive files (.env, credentials)
- Uses 5-layer security check before commits
- Requires confirmation for destructive operations
- Crash recovery ensures no work is lost

---

## Contributing

Contributions are welcome! Please read our [CONTRIBUTING](CONTRIBUTING.md) file for guidelines.

---

## License

This project is licensed under the MIT License - see [LICENSE](LICENSE) for details.

---

## Contact

- **Maintainer:** [Alejandro-M-P](https://github.com/Alejandro-M-P)
- **Issues:** [GitHub Issues](https://github.com/Alejandro-M-P/git-courer/issues)
- **Discussions:** [GitHub Discussions](https://github.com/Alejandro-M-P/git-courer/discussions)
