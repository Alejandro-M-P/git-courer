<!-- markdownlint-disable MD041 -->
<img width="1259" height="619" alt="Gemini_Generated_Image_g9lcw7g9lcw7g9lc" src="https://github.com/user-attachments/assets/00fa1af7-17be-4e5d-bb62-96ad6d038aba" />




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

---

## Demo






https://github.com/user-attachments/assets/cb6f11a7-2972-469a-8bf0-f3d0c0ac1f90







---

git-courer is an **MCP server** that handles git operations locally so your AI coding assistant doesn't waste tokens on them.

Instead of Claude, Cursor, or Windsurf reading diffs and writing commit messages, they delegate to git-courer — which uses a local Ollama model to do it **faster, for free, without sending your code anywhere**.

Every time your AI would read a diff, write a commit, or create a branch, it burns tokens — usually 500–2,000 per operation, without you asking for it. git-courer intercepts those operations and runs them locally. **Near-zero tokens spent on git.**

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

## Supported tools

| Tool | Auto-configured |
|------|----------------|
| Claude Code | ✓ |
| Cursor | ✓ |
| Windsurf | ✓ |
| OpenCode | ✓ |
| Cline | ✓ |
| VS Code | ✓ |
| Claude Desktop | ✓ |
| Continue | ✓ |

Run `git-courer setup` to configure all detected tools at once, or `git-courer mcp cursor` for a specific one.

## Configuration

Run `git-courer setup` in your project — it creates `.gcourer/config.yaml` with sensible defaults.

See **[docs/config.md](docs/config.md)** for all options.

## Known limitations

**Breaking change detection in commits** requires a larger model (13b+). Small models (under ~7b) may write `chore: remove X` when `feat!: remove X` is correct. If your change is breaking, say it explicitly:

> *"commit this — it removes the /api/v1 endpoint, it's a breaking change"*

See **[docs/models.md](docs/models.md)** for a full comparison of tested models and their capabilities.

## Contributing

Read **[CONTRIBUTING.md](CONTRIBUTING.md)** — covers setup, architecture, and how to run the tests (including integration tests with real Ollama).

---

## FAQ

**Do I need Ollama?**
No. git-courer works without it — commit messages will be generic. Install Ollama if you want AI-generated ones.

**Is my code sent anywhere?**
No. Everything runs on your machine — git-courer, Ollama, your data.

**Who decides the version number in a release?**
Go, not Ollama. Version is calculated from commit types (`feat:` → minor, `feat!:` → major). Ollama only writes the human changelog.

**My tool isn't listed.**
Open an issue. If it supports MCP, adding it is usually a few lines.

**How do I mark a breaking change?**
Use `!` after the commit type (`feat!:`) or include `BREAKING CHANGE:` in the body. git-courer picks this up automatically for version bumping and changelog generation.
