# Contributing to git-courer

Thank you for your interest in contributing!

## What We're Working On

git-courer is in **active development** (v0.2.0). We're focused on:

- **Stabilizing the MCP server** — ensuring all tools work correctly
- **Improving secret detection** — reducing false positives/negatives
- **Fixing known issues** — check the [Audit](Audit.md) for current bugs
- **Adding features** — more git operations, better AI integration

Check [GitHub Issues](https://github.com/Alejandro-M-P/git-courer/issues) for tasks to work on.

## Development Setup

### Requirements

- Go 1.24+
- Ollama (optional, for AI features)
- goreleaser (for releases)

### Clone and Build

```bash
git clone https://github.com/Alejandro-M-P/git-courer.git
cd git-courer
go build -o git-courer ./cmd/main.go
```

### Run Tests

```bash
go test ./...
```

## Project Structure

```
git-courer/
├── cmd/main.go              # Entry point
├── internal/
│   ├── app/                 # Application services
│   │   ├── commit/          # Commit flow logic
│   │   ├── git_read/        # Read operations
│   │   ├── git_write/       # Write operations
│   │   ├── git_write_commit/ # Commit with preview
│   │   ├── git_write_review/ # Operations requiring confirmation
│   │   └── security/        # Secrets detection
│   ├── core/                # Domain entities and ports
│   └── infra/               # Infrastructure adapters
├── .gcourer/               # Runtime data (logs, plans, locks)
├── openspec/               # Change specifications
└── scripts/                # Build scripts
```

## Architecture

git-courer follows **Hexagonal Architecture**:

- `core` — Domain entities and ports (no dependencies)
- `app` — Use cases, knows `core` only
- `infra` — Adapters (git, llm, mcp), knows `core` only

See [architecture-rules.md](architecture-rules.md) for details.

## Workflow

This project uses **Spec-Driven Development (SDD)**:

| Step | Description |
|------|-------------|
| **Explore** | Investigate the problem |
| **Propose** | Create change proposal |
| **Spec** | Write specifications with scenarios |
| **Design** | Create technical design |
| **Tasks** | Break down into tasks |
| **Apply** | Implement |
| **Verify** | Validate implementation |
| **Archive** | Sync specs and close |

## Commit Conventions

```
<type>: <short description>

[optional body]
```

Types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `ci`

Examples:
```
feat: add GitWriteCommitAdapter
fix: correct ResetHard subcommand syntax
docs: update README installation steps
```

## Pull Request Process

1. Fork the repository
2. Create a branch from `main`
3. Make changes
4. Add tests if applicable
5. Ensure `go test ./...` passes
6. Open a pull request with a clear description

For bug fixes, include:
- Description of the issue
- Steps to reproduce
- Expected vs actual behavior

For new features, include:
- Description of the feature
- Use case
- Implementation approach

## Release Process

```bash
# Create and push tag
git tag v0.x.x
git push origin v0.x.x
```

goreleaser handles the rest (builds, upload, Homebrew).

## Code Style Guidelines

- Run `go fmt` before committing
- Run `go vet` to catch issues
- Keep functions small and focused
- Add comments for complex logic
- Error handling should be clear

## Reporting Issues

- Use [GitHub Issues](https://github.com/Alejandro-M-P/git-courer/issues) for bugs and features
- Include steps to reproduce
- Specify OS, Go version, Ollama version if relevant
- Check [Audit.md](Audit.md) for known issues

## Code of Conduct

Please be respectful and follow our [Code of Conduct](CODE_OF_CONDUCT.md).

---

## Quick Reference

```bash
# Build
go build -o git-courer ./cmd/main.go

# Test
go test ./...

# Test with coverage
go test -cover ./...

# Run locally
./git-courer
```
