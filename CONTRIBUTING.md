# Contributing to git-courer

Thanks for your interest. This doc covers everything you need to get started.

## What we're working on

Check [GitHub Issues](https://github.com/Alejandro-M-P/git-courer/issues) for open tasks. We're actively working on:

- Improving prompt quality across different model sizes
- Expanding MCP tool coverage
- Better handling of edge cases (empty repos, large diffs, merge conflicts)

## Setup

**Requirements:** Go 1.24+ · Git · Ollama (optional, for integration tests)

```bash
git clone https://github.com/Alejandro-M-P/git-courer.git
cd git-courer
go build -o git-courer ./cmd/main.go
```

## Tests

```bash
# Unit tests (no Ollama needed — runs in CI)
go test ./...

# Integration tests (requires Ollama running)
go test -tags integration ./internal/integration/... -v

# Installer tests only
go test ./internal/installer/... -v
```

Integration tests use real Ollama with `qwen3.5:latest`. They create isolated git repos in `t.TempDir()` — they never touch the actual project repo.

## Project structure

```
git-courer/
├── cmd/main.go                   # Entry point
├── internal/
│   ├── adapters/                 # Implementations of ports
│   │   ├── confirm/              # Plan/lock lifecycle (file-based + in-memory)
│   │   ├── git/                  # Git adapter (exec-based)
│   │   └── llm/                  # Ollama adapter
│   ├── config/                   # Config loading and defaults
│   ├── core/
│   │   ├── domain/               # Types, semver logic (no dependencies)
│   │   ├── errors/               # Typed errors
│   │   └── ports/                # Interfaces (Git, LLM, Confirm, Security)
│   ├── delivery/mcp/             # MCP server and handlers
│   ├── infra/
│   │   ├── chunkers/             # Diff and log chunkers
│   │   ├── logging/              # Rotating log
│   │   └── secrets/              # Secret detection (regex + magic bytes)
│   ├── installer/                # Install, setup, MCP config per tool
│   ├── integration/              # Integration tests (build tag: integration)
│   ├── security/                 # Multi-layer security service
│   ├── shared/prompts/           # LLM prompt templates (.txt files)
│   └── workflow/                 # Commit and release services
├── docs/                         # Config reference, model guide
└── openspec/                     # Change specifications (SDD)
```

## Architecture

Hexagonal architecture — ports in `core/ports/`, implementations in `adapters/` and `infra/`. The core never imports from adapters.

```
delivery/mcp → workflow → core/ports ← adapters (git, llm, confirm)
                       ↓
                  core/domain
```

## Commit conventions

```
type: short description (max 72 chars)

Optional body — explain WHY, not WHAT.
```

Types: `feat` `fix` `refactor` `chore` `test` `docs` `perf` `ci`

Breaking changes: add `!` after type (`feat!:`) or `BREAKING CHANGE:` in the body.

## Pull request process

1. Branch from `main`
2. Make changes with tests
3. `go test ./...` must pass
4. Open PR with a clear description of what and why

## Prompt changes

Prompts live in `internal/shared/prompts/txt/`. If you change one, run the integration tests to see the actual model output:

```bash
go test -tags integration ./internal/integration/... -v -run TestCommit
```

Check the logged commit messages — they should describe purpose, not file names.

## Reporting issues

Use [GitHub Issues](https://github.com/Alejandro-M-P/git-courer/issues). Include:
- OS and Go version
- Ollama model (if relevant)
- Steps to reproduce
- Expected vs actual behavior
