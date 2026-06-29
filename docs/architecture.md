# Architecture

High-level overview of git-courer's codebase for contributors.

## Tech Stack

- **Language**: Go 1.26+
- **MCP Server**: Custom implementation using `mark3labs/mcp-go` under `internal/delivery/mcp`
- **LLM Integration**: Unified OpenAI-compatible backend adapter (supporting Ollama, LM Studio, vLLM, LocalAI, OpenAI, etc.).
- **AI Toggle**: Offline mode when `llm.enabled` is `false` — runs without an LLM (commit pipeline accepts a manual message instead of `why`).
- **Architecture**: Hexagonal / Clean Architecture

## Directory Structure

```
git-courer/
├── cmd/                    # CLI entry point (main.go: dispatches doctor, hook-check, release, MCP server)
├── ffi/                    # Pre-built native shared libraries (libgit2 bindings) per platform
│   ├── darwin_amd64/       # macOS Intel
│   ├── darwin_arm64/       # macOS Apple Silicon
│   ├── linux_amd64/        # Linux x86_64
│   ├── windows_amd64/      # Windows x86_64
│   └── include/            # C headers for libgit2
├── internal/
│   ├── adapters/           # Interface implementations (Ports)
│   │   ├── commitstore/    # Per-branch file-based commit plan database
│   │   ├── confirm/        # In-memory session confirm/safety locks
│   │   ├── git/            # Exec-based Git adapter & Session redirect wrapper
│   │   ├── github/         # API client for PR integration (optional, via `gh` CLI)
│   │   ├── llm/            # OpenAI-standard chat completion client
│   │   │   ├── ollama/     # Ollama-specific client lifecycle manager
│   │   │   └── openai_standard/ # Core OpenAI standard adapter
│   │   └── sessionstore/   # JSON-based session database
│   ├── classifier/         # Command classification (shell-command → MCP tool routing)
│   │   └── gitcmd/         # Classifies git shell commands to redirect to MCP tools (used by hook-check)
│   ├── config/             # Config loaders (global ~/.config/git-courer/config.yaml)
│   ├── core/
│   │   ├── domain/         # Pure domain models (Session, ProjectConfig, Backup, etc.)
│   │   └── ports/          # Central interfaces driven by adapters
│   ├── data/               # Embedded static data (languages.json for chunker language detection)
│   ├── delivery/
│   │   ├── cli/            # CLI subcommands (doctor.go, hook_check.go, release.go)
│   │   └── mcp/            # MCP server & modular command handlers
│   │       ├── branch/     # branch tool handler
│   │       ├── core/       # status, diff, and commit tool handlers
│   │       ├── descriptions/ # centralized tool descriptions
│   │       ├── history/    # history tool handler
│   │       ├── integrate/  # integrate tool handler
│   │       ├── prreview/   # pr-review tool handler
│   │       ├── rewrite/    # rewrite tool handler
│   │       ├── session/    # session tool handler
│   │       ├── shared/     # Shared MCP helpers (param validation, progress, token sanitizer, diff formatters)
│   │       ├── stage/      # stage & stash tool handlers
│   │       ├── sync/       # sync tool handler
│   │       └── utility/    # backup tool handler
│   ├── infra/              # Low-level utilities
│   │   ├── chunkers/       # Diff & log chunking for context windows
│   │   ├── classifier/     # AST-based commit classification via tree-sitter
│   │   ├── filters/        # Diff noise filtering (whitespace/renames/no-op removal)
│   │   └── secrets/        # Secret matching rules
│   │       ├── blacklist.go     # Folder & filename blacklists
│   │       ├── detector.go      # Regex-based secret detection in files and diff content
│   │       └── magic_bytes.go   # Binary file detection via magic bytes
│   ├── installer/          # CLI installer & client configuration setup (hooks, prompt blocks, policies)
│   ├── models/             # Local LLM detection utilities
│   ├── security/           # Multi-layer paranoid security service orchestrator
│   │   └── model.go        # Model-size gating (ParseModelSize) for LLM scan eligibility
│   ├── shared/             # Common helper packages
│   │   ├── prompts/        # Chat completion markdown prompt templates
│   │   │   └── md/         # Core markdown template files
│   │   └── testutil/       # Shared test utilities
│   └── workflow/           # Main business workflows (CommitService, ReleaseService, SessionFinishWorkflow)
├── tui/                    # Interactive configuration TUI (Bubbletea + Lipgloss)
│   ├── screens/            # TUI screens (init, install, mcp_setup)
│   ├── components/         # Reusable TUI components (checkbox, dynamic_form, form)
│   └── styles/             # Lipgloss theme & shared styles
├── test/                   # E2E test suites
│   ├── pipeline/           # Commit pipeline E2E tests
│   └── release/            # Release & changelog E2E tests
├── docs/                   # Documentation resources
└── scripts/                # Automated installer scripts
```

## Layered Architecture

```
┌─────────────────────────────────────────────┐
│    AI Assistant Client (Claude, Codex...)   │
└──────────────────┬──────────────────────────┘
                   │ MCP Protocol (JSON-RPC)
┌──────────────────▼──────────────────────────┐
│      internal/delivery/mcp (MCP Server)     │
└──────────────────┬──────────────────────────┘
                   │
┌──────────────────▼──────────────────────────┐
│       internal/workflow (Unified Engine)    │
│  - Commit / Release workflow management      │
│  - Automated backups & recovery actions      │
└──────────────────┬──────────────────────────┘
                   │
        ┌──────────┼──────────┐
        │          │          │
┌───────▼──┐ ┌────▼───┐ ┌──▼──────────┐
│ core/domain│ │adapters│ │ infra/      │
│ (models)  │ │(git,llm)│ │ (secrets,  │
│            │ │multi-  │ │  chunkers)  │
│            │ │backend)│ │            │
└───────────┘ └────────┘ └────────────┘
```

## Hexagonal Architecture

1. **Ports** (`internal/core/ports/`): Domain interfaces defining the required services (e.g. `Git`, `LLM`, `SecurityService`, `Confirm`).
2. **Adapters** (`internal/adapters/`): Implementations targeting external systems (e.g. CLI git execution, HTTP request adapters for LLM, file-based/in-memory session stores).
3. **Domain** (`internal/core/domain/`): Core logic, schemas, and models (e.g. `Backup`, `Session`, `ProjectConfig`) that do not depend on external libraries.

## Interactive Configuration TUI (`tui/`)

The CLI configurator runs as an interactive terminal UI built with [Bubbletea](https://github.com/charmbracelet/bubbletea) and [Lipgloss](https://github.com/charmbracelet/lipgloss). It guides users through:
- Auto-detecting and configuring active MCP clients.
- Setting up LLM backends (Ollama auto-discovery, custom OpenAI providers).
- Saving configuration to the global path.

## 6-Layer Proactive Security (`internal/security/`)

Before staging or committing any changes, the security orchestrator checks the working tree and staged diffs in memory using 6 cumulative defense layers, executed in this order (matches `internal/security/security.go`):

1. **Binary File Block**: Magic bytes check (`secrets.IsBinary`) on files, plus AI-powered content audit (`AuditBinaryContent`) to detect obfuscated payloads disguised as plain text. Go test files (`_test.go`) skip the AI audit.
2. **Folder Blacklist**: Rejects tracking in sensitive directories (e.g. `vendor/`, `.git/`, `.github/`, `test/`). Hard stop.
3. **Name Blacklist**: Blocks known sensitive files (e.g. `.env`, `id_rsa`, `credentials.json`, `auth.key`). Hard stop. Legitimate Go test files (`_test.go`) are exempted here and skip the remaining layers.
4. **Static Analysis**: Runs external detectors (`trufflehog` or `gosec`) on the file list if installed. 30s timeout. Hard stop on findings.
5. **Regex Search**: Matches file content and staged diff lines against a library of regular expressions targeting common secret formats (e.g. Stripe keys, AWS credentials). Test files (`_test.go`, `test/...`) are filtered out before findings are merged.
6. **LLM Verification**: A paranoid AI auditor (`VerifySecrets`) parses a filtered diff (test files and prompts removed via `filterDiff`) to confirm or override regex findings. If the LLM validates that a low-confidence regex match is not a secret, it can clear the block to avoid false positives.

## Testing

To run the test suite, use the targets defined in the [Makefile](file:///git-courer/git-courer/Makefile):

```bash
# Unit tests + go vet (no LLM required)
make test-unit

# E2E Pipeline tests (requires LLM running locally)
make test-e2e

# Run vet and format checks
make lint

# Clean temporary build files
make clean
```

## Common Paths

| Feature | File / Directory |
|---------|------------------|
| Core MCP Server | [internal/delivery/mcp/server.go](file:///git-courer/git-courer/internal/delivery/mcp/server.go) |
| Commit Workflow | [internal/workflow/commit.go](file:///git-courer/git-courer/internal/workflow/commit.go) |
| Release Workflow | [internal/workflow/release.go](file:///git-courer/git-courer/internal/workflow/release.go) |
| Chunker Utility | [internal/infra/chunkers/diff.go](file:///git-courer/git-courer/internal/infra/chunkers/diff.go) |
| Security Service | [internal/security/security.go](file:///git-courer/git-courer/internal/security/security.go) |
| Markdown Prompts | [internal/shared/prompts/md/](file:///git-courer/git-courer/internal/shared/prompts/md/) |
| Git Porcelains | [internal/adapters/git/](file:///git-courer/git-courer/internal/adapters/git/) |
| E2E Commit Tests | [test/pipeline/e2e_test.go](file:///git-courer/git-courer/test/pipeline/e2e_test.go) |
| E2E Release Tests | [test/release/e2e_test.go](file:///git-courer/git-courer/test/release/e2e_test.go) |
