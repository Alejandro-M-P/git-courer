<!-- markdownlint-disable MD041 -->

<p align="center">
  <img src=".github/logo.png" alt="git-courer" width="200" />
</p>

<h1 align="center">git-courer</h1>
<p align="center">
  <strong>The Local Git Specialist</strong> — Zero tokens for git operations 🚀🤖
</p>

---

## The Problem

Every time a cloud AI agent needs to do git work, it wastes tokens on:

1. Reading diffs
2. Generating commit messages
3. Generating branch names
4. Analyzing what files changed

All this mechanical work adds up and costs money.

## The Solution

**git-courer** is a local MCP server that handles ALL git operations. The cloud AI just delegates to git-courer instead of executing git directly.

```
Cloud AI: "Make a commit"
         ↓
git-courer: (reads diff, generates message, commits, pushes)
         ↓
Result: "✓ commit done"
```

**Zero tokens spent on git operations.**

---

## Features

- 🔌 **MCP Server** — Works with Opencode, Claude Code, Cursor, and any MCP client
- 📦 **Single Binary** — No dependencies, just install and run
- ⚡ **All Git Operations** — status, diff, log, add, commit, push, pull, branch, checkout, stash, reset, and more
- 🔒 **User Confirmation** — Every operation requires user confirmation
- 🤖 **AI-Powered** — When Ollama is available: auto-generates commit messages, suggests branch names
- 🛡️ **Secrets Protection** — Detects and avoids staging sensitive files
- 🌍 **Multi-Platform** — Linux, macOS, Windows
- 🏗️ **Clean Architecture** — Hexagonal architecture with ports and adapters

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
4. Ask which AI tool you use
5. Generate the config for your project

### Manual Install

```bash
# Download binary
curl -fsSL https://github.com/Alejandro-M-P/git-courer/releases/latest/download/git-courer-linux-amd64 -o git-courer
chmod +x git-courer

# Move to PATH
sudo mv git-courer /usr/local/bin/

# Or just use locally
./git-courer
```

---

## Configuration

git-courer reads configuration from `git-courer.yaml` in your project directory.

```yaml
# git-courer.yaml
ollama:
  host: http://localhost:11434
  model: llama3.2
  auto_start: false

git:
  workdir: .
  auto_add_secrets: true
  require_clean_repo: false

validation:
  require_confirmation: true
  max_commit_length: 72

ui:
  theme: dark
  show_icons: true
```

---

## Usage

### Start the server

```bash
git-courer
```

The server runs as an MCP server, waiting for tool calls.

### MCP Tools Available

| Tool | Description |
|------|-------------|
| `git_status` | Get current repository status |
| `git_diff` | Show changes in working directory |
| `git_log` | Show commit history |
| `git_add` | Stage files for commit |
| `git_commit` | Create a commit with staged changes |
| `git_push` | Push commits to remote |
| `git_pull` | Pull changes from remote |
| `git_branch` | List or create branches |
| `git_checkout` | Switch branches |
| `git_stash` | Stash changes |
| `git_reset` | Reset changes |

---

## Compatible Tools

### Opencode

Add to `.opencode/config.json`:

```json
{
  "mcp": {
    "git-courer": {
      "type": "local",
      "command": ["git-courer"]
    }
  }
}
```

### Claude Code

Add to `.claude/settings.json`:

```json
{
  "mcpServers": {
    "git-courer": {
      "command": "git-courer"
    }
  }
}
```

### Cursor

Add to `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "git-courer": {
      "command": "git-courer"
    }
  }
}
```

---

## Architecture

git-courer uses **Hexagonal Architecture** for clean separation:

```
┌─────────────────────────────────────────┐
│           MCP Server (Core)            │
└──────────────┬──────────────┬────────────┘
              │              │
   ┌──────────┴──────────┐   │
   │    Git Port        │   │    UI Port
   ├─────────┬──────────┤   ├────────────┐
   │         │          │   │            │
   ▼         ▼          ▼   ▼            ▼
┌─────────┐ ┌────────────────┐ ┌──────────┐
│ Exec    │ │    Ollama      │ │Bubbletea │
│Adapter │ │   Adapter      │ │ Adapter  │
└─────────┘ └────────────────┘ └──────────┘
  (os/exec)     (REST)           (TUI)
```

### Why this architecture?

- **Testable** — Each component can be tested independently
- **Swappable** — Change implementations without touching core logic
- **Flexible** — Easy to add new AI providers or UI frameworks

---

## Development

### Requirements

- Go 1.26+
- Ollama (optional, for AI features)

### Build

```bash
go build -o git-courer ./cmd/main.go
```

### Test

```bash
go test ./...
```

---

## Contributing

Contributions are welcome! Please read our [contributing guidelines](CONTRIBUTING.md) first.

---

## License

MIT License — see [LICENSE](LICENSE) for details.

---

## Links

- [GitHub](https://github.com/Alejandro-M-P/git-courer)
- [Issues](https://github.com/Alejandro-M-P/git-courer/issues)
- [Releases](https://github.com/Alejandro-M-P/git-courer/releases)

---

<p align="center">
  Made with ❤️ by <a href="https://github.com/Alejandro-M-P">Alejandro-M-P</a>
</p>
