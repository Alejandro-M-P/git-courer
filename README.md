<!-- markdownlint-disable MD041 -->
<img width="1259" height="619" alt="Gemini_Generated_Image_g9lcw7g9lc" src="https://github.com/user-attachments/assets/2e9c0e64-b0de-4b83-9159-3a5906f9f3f4" />

<p align="center">
  <a href="https://github.com/Alejandro-M-P/git-courer/releases/latest">
    <img src="https://img.shields.io/github/v/release/Alejandro-M-P/git-courer?color=%2300BFFF&label=latest" alt="Release">
  </a>
  <a href="https://github.com/Alejandro-M-P/git-courer/actions">
    <img src="https://img.shields.io/github/actions/workflow/status/Alejandro-M-P/git-courer/test.yml?branch=main" alt="Build">
  </a>
  <a href="https://github.com/Alejandro-M-P/git-courer/blob/main/LICENSE">
    <img src="https://img.shields.io/github/license/Alejandro-M-P/git-courer" alt="MIT License">
  </a>
</p>

> **Issues & Bugs**: [@Alejandro-M-P/git-courer/issues](https://github.com/Alejandro-M-P/git-courer/issues) · **Discussions**: [@Alejandro-M-P/git-courer/discussions](https://github.com/Alejandro-M-P/git-courer/discussions)

---

## Quick Links

| Doc | Description |
|-----|-------------|
| **[Architecture](docs/architecture.md)** | Codebase structure, patterns, and how to add features |
| **[Troubleshooting](docs/troubleshooting.md)** | Fix: Ollama not running, MCP not detected, permission errors |
| **[MCP Clients](docs/mcp-clients.md)** | All 10 supported clients, config formats, manual setup |
| **[Config Options](docs/config.md)** | All `.gcourer/config.yaml` settings and examples |
| **[Models Guide](docs/models.md)** | Tested models, token usage, and which one to pick |
| **[Contributing](CONTRIBUTING.md)** | Setup, running tests, and how to collaborate |

---

## Demo

https://github.com/user-attachments/assets/cb6f11a7-2972-469a-8bf0-f3d0c0ac1f90

---

## Stop Burning Money on Git Operations

Every time your AI assistant reads a diff, writes a commit, or creates a branch, it **burns thousands of tokens** — and it does this *automatically*, without you asking.

| Operation | Tokens wasted | Cost (Claude/OpenAI) |
|-----------|--------------|----------------------|
| Read diff | **~1,000** | **$0.03** |
| **Write commit** | **~3,000** | **$0.09** |
| Create branch + PR | **~5,000** | **$0.15** |
| **Release (read history + changelog)** | **~20,000+** | **$0.60+** |
| **Per hour of coding** | **~25,000+** | **$0.75+** |
| **Per day (8h session)** | **~200,000+** | **$6.00+** |

**That's $120–200/month** just for git operations — the stuff your AI *shouldn't* be doing.

> **Releases are the worst**: Your AI reads ALL commits to generate a changelog. That's **20,000–50,000 tokens** ($0.60–1.50) **per release**. Do a release per week? That's **$6–8/month** just for versioning.

git-courer intercepts those operations and runs them **locally, for free**, using Ollama. Same result, zero tokens.

> **Real numbers**: A heavy session with Cursor/Claude Code burns ~50,000 tokens/hour on git ops. At $3 per million tokens, that's **$1.50/hour** — or **$200+/month** if you code daily. git-courer drops that to $0.

## How it works

```
You: "commit my changes"
        ↓
AI delegates to git-courer (via MCP)
        ↓
git-courer: reads diff → checks for secrets → asks Ollama → commits
        ↓
"✓ feat: add user authentication"
```

Every commit runs through 5 security layers that catch API keys, passwords, and tokens **before** they're staged.

Releases combine two things: **Go calculates the version** from your commit types (`feat:` → minor, `feat!:` → major), and **Ollama writes a human-readable changelog** from those commits.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/Alejandro-M-P/git-courer/main/scripts/install.sh | sh
```

That's it. It installs the binary and auto-configures every AI tool it detects on your machine.

**Requirements:** Git · [Ollama](https://ollama.com) (optional, for AI commit messages)

**Manual install:**
```bash
# macOS / Linux
curl -fsSL https://github.com/Alejandro-M-P/git-courer/releases/latest/download/git-courer_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m).tar.gz | tar -xz -C /usr/local/bin git-courer
chmod +x /usr/local/bin/git-courer
git-courer setup

# Windows (PowerShell)
irm https://github.com/Alejandro-M-P/git-courer/releases/latest/download/git-courer_windows_amd64.tar.gz | tar -xz -o git-courer.exe
.\git-courer.exe setup

# Or with Go
go install github.com/Alejandro-M-P/git-courer@latest
```

## Supported Tools

| Tool | Auto-configured | Config Guide |
|------|----------------|--------------|
| Claude Code | ✓ | [@Alejandro-M-P/git-courer/issues](https://github.com/Alejandro-M-P/git-courer/issues) |
| Cursor | ✓ | [docs/mcp-clients.md](docs/mcp-clients.md) |
| Windsurf | ✓ | [docs/mcp-clients.md](docs/mcp-clients.md) |
| OpenCode | ✓ | [docs/mcp-clients.md](docs/mcp-clients.md) |
| Cline | ✓ | [docs/mcp-clients.md](docs/mcp-clients.md) |
| VS Code | ✓ | [docs/mcp-clients.md](docs/mcp-clients.md) |
| Claude Desktop | ✓ macOS/Win only | [docs/mcp-clients.md](docs/mcp-clients.md) |
| Continue | ✓ | [docs/mcp-clients.md](docs/mcp-clients.md) |
| Zed | ✓ | [docs/mcp-clients.md](docs/mcp-clients.md) |
| Gemini CLI | ✓ | [docs/mcp-clients.md](docs/mcp-clients.md) |

Run `git-courer setup` to configure all detected tools at once, or `git-courer mcp <client>` for a specific one.

## Commands

git-courer runs as an interactive TUI when launched without arguments. It also provides MCP server and management commands:

| Command | Description |
|---------|-------------|
| `git-courer` | Launch interactive TUI (requires terminal) |
| `git-courer mcp` | Run MCP server |
| `git-courer mcp setup` | Configure all detected AI tools |
| `git-courer mcp setup <client>` | Configure a specific tool (e.g. `cursor`) |
| `git-courer remove` | Remove git-courer from the current project |
| `git-courer uninstall` | Uninstall the binary globally |
| `git-courer update` | Update to the latest version |
| `git-courer version` | Show current version |

### MCP Tool Commands

For the 3 MCP tools (`git_read`, `git_write`, `git_write_review`) and their subcommands, see **[docs/commands.md](docs/commands.md)**.

## Configuration

git-courer uses a **global configuration file only**: `~/.config/git-courer/config.yaml`. Run `git-courer setup` to create the initial config with sensible defaults.

> **Note**: git-courer v1.5+ no longer loads per-project `.gcourer/config.yaml`. All settings are in the global config.

All options: **[docs/config.md](docs/config.md)**

## Troubleshooting

Having issues? Check **[docs/troubleshooting.md](docs/troubleshooting.md)** for:
- Ollama not running / model not configured
- MCP not detected by your AI tool
- Permission errors during install
- Secrets detected in commits (false positives)

MCP config file locations: **[docs/mcp-clients.md](docs/mcp-clients.md)**

## Known Limitations

**Breaking change detection in commits** requires a larger model (13b+). Small models (under ~7b) may write `chore: remove X` when `feat!: remove X` is correct. If your change is breaking, say it explicitly:

> *"commit this — it removes the /api/v1 endpoint, it's a breaking change"*

Model comparison: **[docs/models.md](docs/models.md)**

---

## Contributing

Want to collaborate? Here's everything you need:

### Architecture & Codebase

Read **[docs/architecture.md](docs/architecture.md)** for:
- Directory structure and tech stack
- Hexagonal architecture patterns
- Key packages and their responsibilities
- How to add a new feature
- Testing approach

### Reporting Bugs

Found a bug? Open an issue: **[@Alejandro-M-P/git-courer/issues](https://github.com/Alejandro-M-P/git-courer/issues)**

Include:
- Your OS and git-courer version (`git-courer version`)
- AI tool you're using (Claude Code, Cursor, etc.)
- Steps to reproduce
- Relevant logs or error messages

### How to Collaborate

1. **Read the docs**: Start with [docs/architecture.md](docs/architecture.md) and [CONTRIBUTING.md](CONTRIBUTING.md)
2. **Pick an issue**: Check [@Alejandro-M-P/git-courer/issues](https://github.com/Alejandro-M-P/git-courer/issues) for `good first issue` labels
3. **Discuss**: Use [@Alejandro-M-P/git-courer/discussions](https://github.com/Alejandro-M-P/git-courer/discussions) for questions or feature ideas
4. **Submit PR**: Follow conventional commits (`feat:`, `fix:`, `chore:`)

### Adding a New MCP Client

If your AI tool supports MCP but isn't listed, adding it is usually **5 lines of code** in `internal/installer/mcp_config.go`. See [docs/mcp-clients.md](docs/mcp-clients.md) for the format.

---

## FAQ

**Do I need Ollama?**
Yes, git-courer requires a model to be configured. You can use Ollama or any OpenAI-compatible server (LM Studio, vLLM, LocalAI, etc.). Without a configured model, operations will fail with an error.

**Is my code sent anywhere?**
No. Everything runs on your machine — git-courer, Ollama, your data.

**Who decides the version number in a release?**
Go, not Ollama. Version is calculated from commit types (`feat:` → minor, `feat!:` → major). Ollama only writes the human changelog.

**My tool isn't listed.**
Open an issue: [@Alejandro-M-P/git-courer/issues](https://github.com/Alejandro-M-P/git-courer/issues). If it supports MCP, adding it is usually a few lines.

**How do I mark a breaking change?**
Use `!` after the commit type (`feat!:`) or include `BREAKING CHANGE:` in the body. git-courer picks this up automatically for version bumping and changelog generation.
