# Contributing to git-courer

Thanks for your interest! We're glad you're here.

Please note that this project is governed by a [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you agree to uphold its terms.

## Table of Contents

- [What we're working on](#what-were-working-on)
- [Good first issues](#good-first-issues)
- [Setup](#setup)
- [Development workflow](#development-workflow)
- [Tests](#tests)
- [Project structure](#project-structure)
- [Architecture](#architecture)
- [Commit conventions](#commit-conventions)
- [Pull request process](#pull-request-process)
- [Review process](#review-process)
- [Prompt changes](#prompt-changes)
- [Reporting issues](#reporting-issues)
- [Getting help](#getting-help)

## What we're working on

Check [GitHub Issues](https://github.com/Alejandro-M-P/git-courer/issues) for open tasks. We're actively working on:

- Improving prompt quality across different model sizes
- Expanding MCP tool coverage
- Better handling of edge cases (empty repos, large diffs, merge conflicts)

## Good first issues

If you're new to the project, look for issues labeled [`good first issue`](https://github.com/Alejandro-M-P/git-courer/labels/good%20first%20issue). These are smaller, well-scoped tasks that don't require deep knowledge of the codebase.

## Setup

**Requirements:** Go 1.25+ · Git · Ollama (optional, for integration tests)

```bash
git clone https://github.com/Alejandro-M-P/git-courer.git
cd git-courer
go build -o git-courer ./cmd/main.go
```

Quick sanity check:
```bash
make test-ci
```

## Development workflow

1. **Pick an issue** or create one to discuss your change first
2. **Create a branch** from `main`: `git checkout -b feat/my-change`
3. **Make changes** with tests
4. **Run tests locally** before committing
5. **Open a PR** with a clear description
6. **Address review feedback** if any

Branch naming:
| Pattern | Example |
|---------|---------|
| `feat/<name>` | `feat/add-gitea-support` |
| `fix/<name>` | `fix/empty-repo-crash` |
| `docs/<name>` | `docs/api-reference` |
| `chore/<name>` | `chore/update-deps` |

## Tests

**Tests must pass before merge.** CI enforces this automatically.

```bash
# Quick check (no Ollama needed — runs in CI)
make test-ci

# Unit tests only
make test-unit

# Full suite (requires Ollama for integration/E2E)
make test-full
```

Integration tests use real Ollama with `qwen3.5:latest`. They create isolated git repos in `t.TempDir()` — they never touch the actual project repo.

### Test targets

| Command | Ollama? | What it runs |
|---------|---------|-------------|
| `make test-ci` | No | Build + unit tests + vet (CI) |
| `make test-unit` | No | Unit tests with gotestsum |
| `make test-integration` | Yes | Integration tests |
| `make test-e2e` | Yes | End-to-end workflow tests |
| `make test-torture` | Yes | Stress, injection, edge cases |
| `make test-full` | Both | Everything |

## Project structure

```
git-courer/
├── cmd/main.go                   # Entry point
├── internal/
│   ├── adapters/                 # Implementations of ports
│   │   ├── confirm/              # Plan/lock lifecycle (file-based + in-memory)
│   │   ├── git/                  # Git adapter (exec-based)
│   │   └── llm/                  # LLM adapters (Ollama + OpenAI-compatible)
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
├── tui/                          # Interactive terminal UI (Bubbletea)
└── docs/                         # Config reference, model guide
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
3. Run `make test-ci` — it must pass
4. Open a PR using the [PR template](.github/PULL_REQUEST_TEMPLATE.md)

### PR checklist (required)

- [ ] `go build ./...` compiles without errors
- [ ] `go test ./...` passes
- [ ] `go vet ./...` passes
- [ ] Followed [conventional commits](https://www.conventionalcommits.org/)
- [ ] Updated documentation if applicable

**If tests fail, the PR will not be merged.** No exceptions.

## Review process

- PRs need at least **one approval** from a maintainer
- All conversations must be **resolved** before merge
- **Stale reviews are dismissed** automatically when new commits are pushed
- Reviews focus on correctness, test coverage, and architecture fit

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

For security vulnerabilities, see [SECURITY.md](SECURITY.md) — **do not** open a public issue.

## Getting help

- **Discussions**: [github.com/Alejandro-M-P/git-courer/discussions](https://github.com/Alejandro-M-P/git-courer/discussions)
- **Issues**: For bugs and feature requests
