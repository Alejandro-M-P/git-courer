# Architecture

High-level overview of git-courer's codebase for contributors.

## Tech Stack

- **Language**: Go 1.21+
- **MCP Server**: Custom implementation in `internal/delivery/mcp`
- **LLM Integration**: Ollama (local) via HTTP API
- **Architecture**: Hexagonal / Clean Architecture

## Directory Structure

```
git-courer/
├── cmd/                    # CLI entry point
├── internal/
│   ├── adapters/           # External adapters
│   │   ├── git/           # Git operations (porcelain commands)
│   │   ├── llm/           # Ollama client
│   │   └── confirm/       # User confirmation prompts
│   ├── core/
│   │   ├── domain/        # Pure domain models (Commit, Release, etc.)
│   │   ├── ports/         # Interfaces (driven by adapters)
│   │   └── errors/        # Domain errors
│   ├── delivery/
│   │   └── mcp/           # MCP protocol server implementation
│   ├── infra/             # Infrastructure
│   │   ├── chunkers/      # Diff chunking for large changes
│   │   ├── logging/       # Structured logging
│   │   └── secrets/       # Secret detection (5 security layers)
│   ├── installer/         # Binary install, MCP config generation
│   ├── shared/
│   │   └── prompts/       # Ollama prompt templates
│   └── workflow/          # Business logic (commit, release, branch)
├── plugin/                 # Editor plugins (OpenCode, etc.)
├── prompts/                # Agent instructions for AI tools
├── docs/                   # Documentation
└── scripts/                # Install/uninstall scripts
```

## Layered Architecture

```
┌─────────────────────────────────────────────┐
│         AI Assistant (Claude, Cursor, etc.) │
└──────────────────┬──────────────────────────┘
                   │ MCP Protocol (JSON-RPC)
┌──────────────────▼──────────────────────────┐
│      internal/delivery/mcp (MCP Server)     │
└──────────────────┬──────────────────────────┘
                   │
┌──────────────────▼──────────────────────────┐
│       internal/workflow (Unified Engine)    │
│  - Orchestrator (Workflow struct)           │
│  - Specialized services (Commit, Release)   │
│  - Proactive Security Interceptor           │
└──────────────────┬──────────────────────────┘
                   │
        ┌──────────┼──────────┐
        │          │          │
┌───────▼──┐ ┌────▼───┐ ┌──▼──────────┐
│ core/domain│ │adapters│ │ infra/      │
│ (models)  │ │(git,llm)│ │ (secrets,  │
│            │ │        │ │  chunkers)  │
└───────────┘ └────────┘ └────────────┘
```

## Key Patterns

### Unified Workflow Engine
As of v1.1.0, all Git operations requiring AI or confirmation pass through a single orchestrator in `internal/workflow/workflow.go`. This ensures:
- **Atomic Operations**: Automatic backup and rollback (Capture state before START, restore on ABORT/Failure).
- **Proactive Security**: Every change is audited for secrets BEFORE the user sees a preview.
- **Consistency**: Unified preview generation and integrity checks (via Diff Hashing).

### Hexagonal Architecture
- **Ports** (`internal/core/ports/`): Interfaces defining what the core needs. Recently added `RenameBranch` and `VerifySecrets`.
- **Adapters** (`internal/adapters/`): Implementations of those ports.
- **Domain** (`internal/core/domain/`): Pure business models. Added `Backup` and `Summary` types.

### Semantic Polyglot Chunking (`internal/infra/chunkers/`)
The `DiffChunker` now understands functional relationships across languages (Go, Python, JS, TS, Rust). It groups files based on:
1. **Semantic Links**: Caller-callee relationships (e.g., a function in A.go called by B.go).
2. **Atomic Pairs**: Keeps Code and Test files together.
3. **Directory Affinity**: Prioritizes grouping files within the same domain/folder.

### Multi-Layer Proactive Security (`internal/infra/secrets/`)
Security is no longer optional or model-dependent:
1. **Magic Bytes**: Direct header scan for binary executables.
2. **Statistical Audit**: Detection of disguised binary payloads.
3. **Path Blacklist**: Filename-based blocking.
4. **Memory-First Regex**: Scans the actual staged Diff in memory.
5. **AI Auditor**: A paranoid LLM agent verifies potential leaks.

## Adding a New Feature

1. **Domain**: Add models/types in `internal/core/domain/`
2. **Ports**: Define interfaces in `internal/core/ports/`
3. **Adapters**: Implement in `internal/adapters/` if needed
4. **Workflow**: Add business logic in `internal/workflow/`
5. **MCP Tools**: Expose via `internal/delivery/mcp/` if it's a user-facing operation
6. **Tests**: Add in the same package as the implementation

## Testing

```bash
# Unit tests (standard)
make test-unit

# CI tests (PR-ready, no Ollama)
make test-ci

# Quality & Prompt Accuracy (requires Ollama)
make test-quality

# Extreme Stress & E2E Torture (Armageddon)
make test-torture

# The Ultimate Maratón (Run everything)
make test-full
```

## Common Paths

| What | Where |
|------|-------|
| Unified Orchestrator | `internal/workflow/workflow.go` |
| Execution Engine | `internal/workflow/execute.go` |
| Preview Logic | `internal/workflow/generate.go` |
| Semantic Chunker | `internal/infra/chunkers/diff.go` |
| Security Service | `internal/security/security.go` |
| AI Prompts (.txt) | `internal/shared/prompts/txt/` |
| Quality Matrix | `internal/adapters/llm/prompt_matrix_test.go` |
| E2E Torture | `test/e2e/armageddon_test.go` |
