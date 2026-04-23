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
│       internal/workflow (Use Cases)          │
│  - CommitWorkflow                           │
│  - ReleaseWorkflow                          │
│  - BranchWorkflow                           │
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

### Hexagonal Architecture
- **Ports** (`internal/core/ports/`): Interfaces defining what the core needs
- **Adapters** (`internal/adapters/`): Implementations of those ports
- **Domain** (`internal/core/domain/`): Pure business models, no dependencies

### MCP Server
The MCP server exposes git operations as "tools" that AI assistants can call:
- `git_read`: Read-only operations (status, diff, log, branches)
- `git_write`: Direct write operations (add, push, pull)
- `git_write_review`: Operations requiring LLM (commit, branch create, merge)

### Workflow Layer
Business logic lives in `internal/workflow/`:
- Each workflow orchestrates adapters to fulfill a use case
- Workflows don't know about MCP, HTTP, or CLI — just ports

### Security Layers (`internal/infra/secrets/`)
Five layers catch secrets before they're committed:
1. Pattern matching (API keys, tokens)
2. Entropy-based detection (high-entropy strings)
3. File-type exclusions (images, binaries)
4. Custom patterns from config
5. Ollama-powered semantic analysis (optional)

## Adding a New Feature

1. **Domain**: Add models/types in `internal/core/domain/`
2. **Ports**: Define interfaces in `internal/core/ports/`
3. **Adapters**: Implement in `internal/adapters/` if needed
4. **Workflow**: Add business logic in `internal/workflow/`
5. **MCP Tools**: Expose via `internal/delivery/mcp/` if it's a user-facing operation
6. **Tests**: Add in the same package as the implementation

## Testing

```bash
# Unit tests
go test ./...

# With race detection
go test -race ./...

# Integration tests (requires Ollama)
go test -tags=integration ./test/e2e/...
```

## Common Paths

| What | Where |
|------|-------|
| MCP tool definitions | `internal/delivery/mcp/tools.go` |
| Commit logic | `internal/workflow/commit.go` |
| Release logic | `internal/workflow/release.go` |
| Ollama client | `internal/adapters/llm/ollama.go` |
| Git operations | `internal/adapters/git/git.go` |
| Security checks | `internal/infra/secrets/` |
| Prompt templates | `internal/shared/prompts/` |
| Agent instructions | `prompts/agent-instructions.md` |
| Installer | `internal/installer/` |
| MCP configs | `internal/installer/mcp_config.go` |
