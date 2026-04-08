<!-- markdownlint-disable MD041 -->

<p align="center">
  <img src=".github/logo.png" alt="git-courer" width="200" />
</p>

<h1 align="center">git-courer</h1>

<p align="center">
  <strong>The Local Git Specialist</strong> — Zero tokens for git operations 🚀🤖✨
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

## ⚠️ Status: Beta

git-courer is in **active development** (v0.1.0-beta). Some features may change.

**Known Issues:**
- MCP client integration is still being tested with some tools
- Secret detection may have false negatives in edge cases
- Preview commit workflow is being validated

---

## The Problem

Every time a cloud AI agent needs to do git work, it wastes tokens on:

| Operation | Tokens Spent | Frequency |
|-----------|--------------|-----------|
| Reading diffs | ~500-2000 | Every time |
| Generating commit messages | ~300-1000 | Every commit |
| Analyzing changed files | ~200-800 | Every operation |
| Generating branch names | ~100-300 | Every branch |

**Result:** Thousands of tokens wasted monthly on mechanical work that could be done locally.

## The Solution

**git-courer** is a local MCP server that handles ALL git operations. The cloud AI just delegates to git-courer.

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                  │
│   Cloud AI: "Make a commit with the changes"                     │
│                         ↓                                        │
│            ┌─────────────────────────┐                          │
│            │      git-courer         │                          │
│            │                         │                          │
│            │  • Reads diff           │                          │
│            │  • Detects secrets      │                          │
│            │  • Generates message    │                          │
│            │  • Groups changes       │                          │
│            │  • Commit + Push        │                          │
│            └─────────────────────────┘                          │
│                         ↓                                        │
│            "✓ Commit created: feat: add auth"                   │
│                                                                  │
│   Tokens spent: ~50 (delegation) vs ~2000 (manual)             │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

**Zero tokens spent on git operations.**

---

## Features

| Feature | Description |
|---------|-------------|
| 🔌 **MCP Server** | Works with Opencode, Claude Code, Cursor, Windsurf, and any MCP client |
| 📦 **Single Binary** | No dependencies. Install and run. |
| ⚡ **Full Operations** | status, diff, log, add, commit, push, pull, branch, checkout, stash, reset, clean, merge, rebase |
| 🤖 **AI-Powered** | When Ollama is available: auto commit messages, smart grouping, intent detection |
| 🛡️ **Secrets Protection** | Detects and avoids staging sensitive files (.env, credentials, keys) |
| 🌍 **Cross-Platform** | Linux, macOS, Windows |
| 🏗️ **Hexagonal Architecture** | Testable and swappable components |
| 💾 **Crash Recovery** | Automatic state recovery if something crashes |

---

## Installation

### Quick Install

```bash
curl -fsSL https://raw.githubusercontent.com/Alejandro-M-P/git-courer/main/install.sh | sh
```

The installer will:

1. Download the latest binary
2. Install to `~/.local/bin`
3. Add to your PATH
4. Detect your AI tool and generate the config

### Manual Install

**Linux/macOS:**
```bash
# Download binary
curl -fsSL https://github.com/Alejandro-M-P/git-courer/releases/latest/download/git-courer-linux-amd64 -o git-courer

# Make executable
chmod +x git-courer

# Move to PATH
sudo mv git-courer /usr/local/bin/

# Or use locally
./git-courer
```

**Windows:**
```powershell
# With PowerShell
irm https://github.com/Alejandro-M-P/git-courer/releases/latest/download/git-courer-windows-amd64.exe -o git-courer.exe
.\git-courer.exe
```

---

## Configuration

git-courer reads configuration from `.gcourer/config.yaml` in your project:

```yaml
# .gcourer/config.yaml

ollama:
  host: http://localhost:11434
  model: llama3.2
  auto_start: false

git:
  workdir: .
  auto_add_secrets: true       # Auto-detect and exclude secrets
  require_clean_repo: false    # Require clean repo before destructive operations

validation:
  require_confirmation: true  # Confirm before dangerous operations
  max_commit_length: 72       # Max commit message length

git_write_commit:
  ttl_minutes: 10              # Lock file TTL for preview mode

ui:
  theme: dark
  show_icons: true
```

### AI Tools Configuration

#### Opencode

```json
// .opencode/config.json
{
  "mcp": {
    "git-courer": {
      "type": "local",
      "command": ["git-courer"]
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

---

## Usage

### Start the Server

```bash
git-courer
```

The server runs as an MCP server, waiting for tool calls.

### Available MCP Tools

#### git_do — Universal Git Operation

Execute any git operation from natural language:

```
git_do(instruction: "show status")
git_do(instruction: "commit the changes")
git_do(instruction: "create branch called feature/new-branch")
git_do(instruction: "push to main")
git_do(instruction: "rebase onto main")
```

#### git_read — Read Operations (read-only)

| Subcommand | Description |
|------------|-------------|
| `READ_STATUS` | Shows current repo status |
| `READ_DIFF` | Shows changes |
| `READ_DIFF_UNSTAGED` | Shows unstaged changes |
| `READ_LOG` | Commit history |
| `READ_BRANCHES` | List branches |

#### git_write — Direct Operations

| Subcommand | Description |
|------------|-------------|
| `ADD` | Stage files |
| `CHECKOUT` | Switch branch |
| `SWITCH` | Switch branch |
| `STASH` | Stash changes |
| `STASH_POP` | Apply stashed changes |
| `PUSH` | Push to remote |
| `PULL` | Pull from remote |
| `FETCH` | Fetch from remote |
| `RM` | Remove files |

#### git_write_review — Operations Requiring Confirmation

| Subcommand | Description |
|------------|-------------|
| `BRANCH_CREATE` | Create branch |
| `BRANCH_DELETE` | Delete branch |
| `MERGE` | Merge branch |
| `REBASE` | Rebase |
| `RESET_SOFT` | Soft reset |
| `RESET_HARD` | Hard reset |
| `CLEAN` | Clean untracked files |
| `REVERT` | Revert commit |

#### git_write_commit — Commits with Preview

Commit workflow with preview support:

```
# Preview Mode (recommended)
git_write_commit("COMMIT_START", preview=true, message: "commitea todo")
  → "pending" (waits for user confirmation)

# Poll until ready
git_write_commit("COMMIT_STATUS")
  → "ready"

# Get the plan
git_write_commit("COMMIT_SUMMARY")
  → Shows planned commits

# Apply
git_write_commit("COMMIT_APPLY")

# Direct Mode (no preview)
git_write_commit("COMMIT_APPLY", message: "commitea todo")
  → Executes directly
```

---

## Practical Examples

### Example 1: Simple Commit

```
> "commitea todo"
↓
git-courer:
  1. Reads diff
  2. Detects secrets (.env not staged)
  3. Generates message: "feat: add JWT authentication"
  4. Commit + push
↓
"✓ Done: 1 commit created"
```

### Example 2: Multiple Commits with Preview

```
> "separate changes into logical commits"
↓
git-courer:
  1. Reads all diffs
  2. Splits into logical chunks
  3. Generates messages for each chunk
  4. Waits for confirmation
↓
[3 commits planned]
  1. fix: correct email validation
  2. feat: add /api/users endpoint
  3. refactor: extract UserService
↓
> "yes, do it"
↓
Executes the 3 commits
```

### Example 3: Branch Management

```
> "create a feature/checkout-flow branch from develop"
↓
git-courer:
  1. Verifies develop exists
  2. Creates and switches to new branch
↓
"✓ Branch feature/checkout-flow created"
```

---

## Architecture

git-courer uses **Hexagonal Architecture** (Ports & Adapters):

```
┌─────────────────────────────────────────────────────────────────┐
│                        MCP Server (Core)                         │
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                      Domain Layer                        │  │
│  │   GitOperation   Commit   Security   LLM   Diff          │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                      Port Interfaces                      │  │
│  │  GitReadPort  GitWritePort  GitWriteCommitPort  Security   │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
          ┌───────────────────┼───────────────────┐
          │                   │                   │
          ▼                   ▼                   ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│   Exec Adapter  │  │  Ollama Adapter │  │  Bubbletea TUI │
│   (os/exec)     │  │    (REST)       │  │    Adapter     │
└─────────────────┘  └─────────────────┘  └─────────────────┘
   Git Commands         AI Generation        Terminal UI
```

### Why Hexagonal Architecture?

- **Testable** — Each component can be tested independently
- **Swappable** — Change implementations without touching core logic
- **Flexible** — Easy to add new AI providers or UIs
- **Maintainable** — Organized and predictable code

---

## Development

### Requirements

- **Go** 1.24+
- **Ollama** (optional, for AI features)

### Build

```bash
git clone https://github.com/Alejandro-M-P/git-courer.git
cd git-courer
go build -o git-courer ./cmd/main.go
```

### Test

```bash
go test ./...
```

### Project Structure

```
git-courer/
├── cmd/
│   └── main.go              # Entry point
├── internal/
│   ├── app/                  # Application services
│   │   ├── commit/          # Commit logic
│   │   ├── git_read/        # Read commands
│   │   ├── git_write/       # Write commands
│   │   ├── git_write_commit/
│   │   ├── git_write_review/
│   │   └── security/        # Secret detection
│   ├── core/
│   │   ├── domain/          # Domain entities
│   │   ├── errors/          # Error types
│   │   └── ports/           # Port interfaces
│   └── infra/
│       ├── config/          # Config loading
│       ├── diff/            # Diff processing
│       ├── git/             # Git adapters
│       ├── llm/             # Ollama adapter
│       ├── logging/         # Logging
│       ├── mcp/             # MCP server
│       └── secrets/         # Secret detection
├── .gcourer/                # Runtime data
├── openspec/                # OpenSpec specs
└── scripts/                 # Build scripts
```

---

## FAQ

### What if Ollama is not available?

Works perfectly without Ollama. Basic git operations always work. Commit messages will be generic ("update files") instead of smart.

### Is it safe?

Yes. git-courer:
- Never uploads sensitive files (.env, credentials)
- Requires confirmation for destructive operations
- Crash recovery ensures no work is lost

### How many tokens does it save?

Depends on your workflow, but a typical case:

| Operation | With git-courer | Without git-courer |
|-----------|-----------------|-------------------|
| Daily commit | ~50 tokens | ~2000 tokens |
| 20 commits/day | 1,000 | 40,000 |
| Monthly | 30,000 | 1,200,000 |

### Can I contribute?

Yes! Read [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

---

## Links

- [GitHub](https://github.com/Alejandro-M-P/git-courer)
- [Issues](https://github.com/Alejandro-M-P/git-courer/issues)
- [Releases](https://github.com/Alejandro-M-P/git-courer/releases)
- [Discussions](https://github.com/Alejandro-M-P/git-courer/discussions)

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/Alejandro-M-P">Alejandro-M-P</a>
</p>
