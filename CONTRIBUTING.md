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

Check [GitHub Issues](https://github.com/blak0p/git-courer/issues) for open tasks. We're actively working on:

- Improving prompt quality across different model sizes
- Expanding MCP tool coverage
- Better handling of edge cases (empty repos, large diffs, merge conflicts)

## Good first issues

If you're new to the project, look for issues labeled [`good first issue`](https://github.com/blak0p/git-courer/labels/good%20first%20issue). These are smaller, well-scoped tasks that don't require deep knowledge of the codebase.

## Setup

**Requirements:** Go 1.25+ · Git · Ollama (optional, for integration tests)

```bash
git clone https://github.com/blak0p/git-courer.git
cd git-courer
go build -o git-courer ./cmd/main.go
```

Quick sanity check:
```bash
make test-unit
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
# Unit tests + vet (no Ollama needed — runs in CI)
make test-unit

# E2E pipeline tests (requires local or remote LLM)
make test-e2e
```

Integration tests use real Ollama with `qwen3.5:latest`. They create isolated git repos in `t.TempDir()` — they never touch the actual project repo.

### Test targets

| Command | LLM required? | What it runs |
|---------|---------------|--------------|
| `make test-unit` / `make test` | No | Code vetting + standard unit tests (fast check) |
| `make test-e2e` | Yes | Commit pipeline & release E2E tests |
| `make lint` | No | Runs go vet static analysis |

## Project structure

```
git-courer/
├── cmd/main.go                   # Entry point
├── internal/
│   ├── adapters/                 # Implementations of ports
│   │   ├── commitstore/          # Per-branch file-based commit plans
│   │   ├── confirm/              # In-memory safety locks
│   │   ├── git/                  # Git adapter & session redirect wrapper
│   │   ├── github/               # Optional PR client
│   │   ├── llm/                  # Unified OpenAI standard client
│   │   └── sessionstore/         # JSON session database
│   ├── classifier/               # Command-based Git change classification
│   ├── config/                   # Config loading and defaults
│   ├── core/
│   │   ├── domain/               # Core logic (no external dependencies)
│   │   └── ports/                # Interfaces (Git, LLM, Confirm, Security)
│   ├── data/                     # Embedded language definitions
│   ├── delivery/mcp/             # MCP server and modular tool handlers
│   ├── infra/
│   │   ├── chunkers/             # Diff and log chunkers
│   │   ├── classifier/           # AST-based classification via tree-sitter
│   │   ├── filters/              # Path filtering
│   │   └── secrets/              # Secret patterns and magic bytes
│   ├── installer/                # Install, setup, and client configs
│   ├── models/                   # Local LLM capability detector
│   ├── security/                 # Multi-layer security service orchestrator
│   ├── shared/prompts/           # Prompt templates
│   │   └── md/                   # Markdown prompt files
│   └── workflow/                 # Commit and release service workflows
├── tui/                          # Interactive terminal UI (Bubbletea)
├── test/                         # E2E test suites
│   ├── pipeline/                 # Commit E2E pipeline tests
│   └── release/                  # Release E2E tests
└── docs/                         # Config, commands, and architecture guides
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
3. Run `make test-unit` (and `make test-e2e` if you changed LLM logic) — all must pass
4. Open a PR using the [PR template](.github/PULL_REQUEST_TEMPLATE.md)

### PR checklist (required)

- [ ] `go build ./...` compiles without errors
- [ ] `go test ./...` unit tests pass
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

Prompts live in `internal/shared/prompts/md/`. If you change one, run the E2E pipeline tests with your local LLM to see the actual model output:

```bash
go test -tags e2e ./test/pipeline/... -v -run TestCommit
```

Check the logged commit messages — they should describe purpose, not file names.

## Reporting issues

Use [GitHub Issues](https://github.com/blak0p/git-courer/issues). Include:
- OS and Go version
- Ollama model (if relevant)
- Steps to reproduce
- Expected vs actual behavior

For security vulnerabilities, see [SECURITY.md](SECURITY.md) — **do not** open a public issue.

## Getting help

- **Discussions**: [github.com/blak0p/git-courer/discussions](https://github.com/blak0p/git-courer/discussions)
- **Issues**: For bugs and feature requests
