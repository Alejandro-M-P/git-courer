<!-- markdownlint-disable MD041 -->

<h1 align="center">git-courer</h1>

<p align="center">
  <strong>The Local Git Specialist</strong><br>
  <em>Zero tokens spent on git operations</em>
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
</p>

---

## What's This?

**git-courer** is your local git assistant that uses AI (Ollama) to handle all git operations. It's an MCP server that connects to your favorite AI coding tool and does the git work for you.

### The Problem

Every time an AI coding assistant (like Claude Code, Cursor, or Windsurf) needs to commit code, create branches, or analyze changes, it spends precious tokens doing git bookkeeping:

| Task | Tokens Wasted |
|------|-------------|
| Reading diffs | 500-2,000 per operation |
| Writing commit messages | 300-1,000 per commit |
| Analyzing changes | 200-800 per operation |

**That's $5-15+ per day** just on git operations you're not even asking for.

### The Solution

Instead of your AI assistant doing git work, it delegates to **git-courer** — which runs locally and costs nothing in tokens.

```
You: "commit my changes"
     ↓
Your AI tool delegates to git-courer
     ↓
git-courer: reads diff → checks for secrets → generates message → commits
     ↓
Result: "✓ Committed: add user authentication"
```

### Why You'll Love It

- **Saves money** — No more wasted tokens on git operations
- **100% local** — Your data never leaves your machine
- **Secure** — 5-layer secret detection blocks accidental credential leaks
- **Works offline** — Git operations work even without Ollama
- **Your tools, your way** — Works with Claude Code, Cursor, Windsurf, OpenCode, and more

### Token Savings

Here's what you're saving every day:

| Task | Without git-courer | With git-courer |
|------|-------------------|----------------|
| Read diff | 500-2,000 tokens | 0 tokens |
| Generate commit message | 300-1,000 tokens | 0 tokens |
| Analyze changes | 200-800 tokens | 0 tokens |
| Create branch | 100-300 tokens | 0 tokens |
| **Daily total** | **1,100-4,100 tokens** | **~0 tokens** |

At $3-5 per 100K tokens with Claude/GPT, that's **$1-5 saved per day**, $30-150 per month — just on git operations you didn't even ask for.

---

## Installation

### One-Command Install

```bash
curl -fsSL https://gitcourer.sh | sh
```

Done! This downloads the binary, installs it, and auto-configures all detected AI coding tools.

### Manual Install

**macOS / Linux:**
```bash
# Download
curl -fsSL https://github.com/Alejandro-M-P/git-courer/releases/latest/download/git-courer-$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m) -o git-courer

# Install
chmod +x git-courer
sudo mv git-courer /usr/local/bin/

# Configure your tools
git-courer setup
```

**Windows (PowerShell):**
```powershell
irm https://github.com/Alejandro-M-P/git-courer/releases/latest/download/git-courer-windows-amd64.exe -o git-courer.exe
.\git-courer.exe setup
```

### Requirements

- **Git** — Already on your machine
- **Ollama** (optional) — For AI-powered commit messages: [ollama.com](https://ollama.com)
- **One of:** Claude Code, Cursor, Windsurf, OpenCode, Cline, Zed, VS Code, or Claude Desktop

### Supported Platforms

| OS | Architecture | Status |
|----|-------------|--------|
| Linux | x86_64 (amd64) | ✓ |
| Linux | ARM64 (apple silicon) | ✓ |
| macOS | Intel (amd64) | ✓ |
| macOS | Apple Silicon (arm64) | ✓ |
| Windows | x86_64 (amd64) | ✓ |

---

## Supported Tools

git-courer works with:

| Tool | Config File | Auto-Detected? |
|------|------------|----------------|
| Claude Code | `~/.claude/settings.json` | ✓ |
| Cursor | `~/.cursor/mcp.json` | ✓ |
| Windsurf | `~/.codeium/windsurf/mcp_config.json` | ✓ |
| OpenCode | `~/.config/opencode/opencode.json` | ✓ |
| Cline | `~/Library/.../cline_mcp_settings.json` | ✓ |
| Zed | `~/.config/zed/settings.json` | ✓ |
| VS Code | `.vscode/mcp.json` | ✓ |
| Claude Desktop | `~/Library/Application Support/Claude/claude_desktop_config.json` | ✓ |

Run `git-courer setup` and it auto-configures whatever tools it detects.

### Commands Reference

| Command | What It Does |
|---------|-------------|
| `git-courer` | Start the MCP server |
| `git-courer install` | Download and install binary |
| `git-courer setup` | Configure current project + all detected AI tools |
| `git-courer setup /path/to/project` | Configure a specific project |
| `git-courer remove` | Remove from current project |
| `git-courer uninstall` | Remove completely from system |
| `git-courer update` | Check for and apply updates |
| `git-courer update --force` | Force update to latest |
| `git-courer version` | Show version |
| `git-courer mcp` | Configure MCP for all detected tools |
| `git-courer mcp cursor` | Configure only Cursor |

---

## Project Configuration

When you run `git-courer setup` in a project, it creates a `.gcourer/config.yaml` file. You can customize how git-courer behaves per-project.

### Config File Location

```
my-project/
├── .gcourer/
│   └── config.yaml    ← project-specific config
└── .git/
```

### Config Options

The settings you'll actually want to change:

```yaml
ollama:
  host: http://localhost:11434    # Ollama API URL
  model: gemma4:26b              # Model to use
  models_dir: ""                 # Custom Ollama models directory

git:
  workdir: .                     # Working directory
  auto_add_secrets: true         # Auto-stage detected secrets
  require_clean_repo: false      # Require clean working tree

secrets:
  detection_mode: regex+ai       # regex, ai, or regex+ai
  patterns:                      # File patterns to check
    - "*.key"
    - "*.pem"
    - ".env*"
    - "credentials.json"
    - "secrets.yaml"
    - "*.password"
    - "*.token"
  use_llm_security_scan: auto     # auto, true, or false

validation:
  require_confirmation: true     # Confirm before commit/branch/release
  max_commit_length: 72          # Max commit message length

preview:
  enabled: true                  # Show preview before executing
  operations:                    # Which ops need confirmation
    commit: true
    branch_create: true
    branch_delete: true
    release: true

commit:
  ttl: 10m                       # Plan time-to-live
  max_plan_retries: 3            # Max retries for failed plans

release:
  max_commits_per_chunk: 20       # Commits per changelog chunk

commands:
  enabled_operations:            # Which operations are allowed
    - commit
    - release
    - push
    - pull
    - branch_create
    - branch_delete
    - merge

backup:
  enabled: true                  # Auto-backup before destructive ops
```

**Advanced settings** (you probably don't need these):
- File paths (lock, plan, logs) — defaults work fine

### Global Config

You can also set a global config at `~/.config/git-courer/config.yaml`. Project config overrides global config.

### Example: Disable Confirmation for Commits

```yaml
# .gcourer/config.yaml
validation:
  require_confirmation: false
preview:
  enabled: false
  operations:
    commit: false
```

Once installed, just talk to your AI coding assistant naturally:

### Commit Changes

```
You: "commit my changes"
AI: (delegates to git-courer)
git-courer: reads diff, detects secrets, generates message
AI: "I'll commit: add user login functionality"
You: "yes"
AI: (applies the commit)
✓ Committed: add user login functionality
```

### Create a Branch

```
You: "create a branch for the new feature"
AI: git_write_review(command="BRANCH_CREATE_START", instruction="new feature branch")
→ "Creating branch: feat/new-feature"
You: "confirm"
AI: git_write_review(command="BRANCH_CREATE_APPLY")
✓ Branch created: feat/new-feature
```

### Create a Release

```
You: "let's release version 1.0"
AI: git_write_review(command="RELEASE_START", instruction="version 1.0")
→ "Creating release with changelog..."
You: "yes"
✓ Release created: v1.0.0
```

---

## Commands Reference

| Command | What It Does |
|---------|-------------|
| `git-courer` | Start the MCP server |
| `git-courer install` | Download and install binary |
| `git-courer setup` | Configure current project + AI tools |
| `git-courer setup [directory]` | Configure a specific project |
| `git-courer remove` | Remove from current project |
| `git-courer uninstall` | Remove completely from system |
| `git-courer update` | Check for updates |
| `git-courer version` | Show version |
| `git-courer mcp` | Configure MCP manually |

---

## How It Works

### The Three-Phase Flow

git-courer uses a confirmable workflow for important operations:

1. **START** — You give an instruction, git-courer creates a plan and shows you a preview
2. **APPLY** — If you confirm, it executes the operation
3. **ABORT** — If something looks wrong, cancel anytime

### Five-Layer Security

Before every commit, git-courer checks for secrets:

1. **Magic bytes** — Blocks binary files
2. **Folder blacklist** — Blocks node_modules, .git, etc.
3. **Name blacklist** — Blocks .env, credentials, secrets
4. **Regex scan** — Detects API keys, passwords, tokens
5. **AI verification** — Ollama confirms findings

### Crash Recovery

If something goes wrong:
- Plans are saved with 10-minute TTL — retry after 10 minutes
- Lock files prevent concurrent operations
- Automatic cleanup of stale locks

---

## Troubleshooting

### "Ollama not found"

That's fine! git-courer works without Ollama. Commit messages will be basic ("update files") instead of AI-generated, but all git operations work perfectly.

### "Not detecting my tool"

Run manually:

```bash
git-courer setup
```

Or for a specific tool:

```bash
git-courer mcp cursor    # configure only Cursor
git-courer mcp claude    # configure only Claude Code
```

### "Permission denied"

```bash
sudo chown $(whoami) /usr/local/bin/git-courer
```

---

## FAQ

### Does it work with [my favorite tool]?

Probably! We've tested with Claude Code, Cursor, Windsurf, OpenCode, Cline, Zed, and VS Code. Open an issue if your tool isn't supported yet.

### Is it safe?

Absolutely:
- Runs 100% locally — nothing leaves your machine
- Never stages sensitive files
- Requires confirmation for destructive operations
- Automatic crash recovery

### Why "zero tokens"?

Your AI assistant (Claude, GPT-4, etc.) charges per token. By delegating git work to git-courer, you save 1,000-5,000 tokens per day on tasks you didn't even ask for.

### Do I need Ollama?

No! git-courer works without Ollama. Get Ollama if you want AI-generated commit messages. Without it, messages will be generic.

### How much does it cost?

Free. Open source, MIT license. Ollama uses your GPU/CPU locally — no API costs.

---

## Contributing

Bug reports, feature requests, and PRs welcome! Read [CONTRIBUTING.md](CONTRIBUTING.md) first.

---

## License

MIT — See [LICENSE](LICENSE) for details.

---

## Links

- **Issues:** https://github.com/Alejandro-M-P/git-courer/issues
- **Discussions:** https://github.com/Alejandro-M-P/git-courer/discussions
- **Maintainer:** [@Alejandro-M-P](https://github.com/Alejandro-M-P)