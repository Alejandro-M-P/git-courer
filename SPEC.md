# git-courer Specification

## Overview

**git-courer** is a local MCP (Model Context Protocol) server that handles all git operations using Ollama for AI-powered commit messages and branch names. Cloud AI agents delegate git operations to this server, saving tokens on mechanical git tasks.

## Problem

When a cloud AI agent needs to make a git commit, it must:
1. Read the diff
2. Generate a commit message
3. Ask for user confirmation
4. Execute git

All of this consumes expensive tokens. With git-courer:
- The cloud only says "git commit" and delegates
- Ollama locally handles all the AI work
- Zero tokens spent on mechanical operations
- User maintains control with confirmation TUI

## Goals

1. **Zero tokens for git operations** - All git work done locally
2. **User control** - Every operation requires user confirmation via TUI
3. **Clean architecture** - Hexagonal architecture for testability and swappable components
4. **Single binary** - `go install` ready, no external dependencies

## Architecture

### Hexagonal Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        MCP Server                            │
│                      (Orchestrator)                          │
└─────────────────────────────────────────────────────────────┘
                              │
         ┌────────────────────┼────────────────────┐
         │                    │                    │
         ▼                    ▼                    ▼
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│   Git Port   │    │   LLM Port   │    │   UI Port    │
│  (interface) │    │  (interface) │    │  (interface) │
└──────────────┘    └──────────────┘    └──────────────┘
         │                    │                    │
         ▼                    ▼                    ▼
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│  ExecAdapter │    │   Ollama     │    │  Bubbletea   │
│   (os/exec)  │    │   Adapter    │    │   Adapter    │
└──────────────┘    └──────────────┘    └──────────────┘
```

### Package Structure

```
cmd/
  main.go              # Entry point

internal/
  domain/
    models/            # Domain models (structs)
    services/          # Business logic

  ports/
    git/               # GitPort interface
    llm/               # LLMPort interface
    ui/                # UIPort interface

  adapters/
    git/               # GitExecAdapter (os/exec implementation)
    llm/               # OllamaAdapter (REST API implementation)
    ui/                # BubbleteaAdapter (TUI implementation)

pkg/
  mcp/                 # MCP server implementation
```

## MCP Tools

The server exposes these tools:

### git_status

Returns human-readable summary of current git status.

**Input**: None

**Output**:
```
Repository: /path/to/repo
Branch: main
Status: clean / 3 files changed (2 staged, 1 modified)

Files:
 M modified.txt
A  added.txt
```

### git_commit

Generates commit message from staged diff, shows confirmation TUI, executes commit.

**Input**: None (reads staged diff automatically)

**Output**:
```
Commit message: feat: add email validation

[s] confirm  [e] edit  [n] cancel
```

On confirm → executes `git commit -m "feat: add email validation"`

### git_push

Shows pending commits and asks for confirmation, then pushes.

**Input**: None

**Output**:
```
Will push 3 commits:
- feat: add email validation
- chore: update dependencies
- fix: resolve nil pointer

[s] push  [n] cancel
```

### git_branch

Suggests branch name based on current task/branch, shows confirmation TUI, creates branch.

**Input**: 
- `task` (optional): Description of the task

**Output**:
```
Suggested branch: feature/add-email-validation

[s] confirm  [e] edit  [n] cancel
```

## TUI Confirmation

When user confirmation is needed, a Bubbletea TUI appears in the terminal.

```
─────────────────────────────────────
🔀 git-courer
─────────────────────────────────────
Summary: you added email validation
and extracted logic to a separate
function in utils/validators.go

Commit: feat: add email validation
> _  ← editable

[s] confirm  [e] edit  [n] cancel
─────────────────────────────────────
```

**Controls**:
- `s` - Confirm and execute
- `e` - Edit the commit message
- `n` - Cancel operation
- `Ctrl+C` - Cancel and exit

## Ollama Integration

### Models

Default model: `llama3.2` (small, fast, sufficient for commit messages)

Alternative models:
- `qwen2.5`
- `mistral`

### Prompts

#### Commit Message Generation

```
You are a git commit message generator. Generate a concise commit message following Conventional Commits format.

Rules:
- Start with type: feat, fix, chore, docs, style, refactor, test, perf, ci, build, revert
- Use imperative mood: "add" not "added" or "adds"
- Keep subject under 72 characters
- No period at end of subject

Diff to analyze:
{DIFF}

Generate only the commit message, nothing else.
```

#### Summary Generation

```
Summarize these git changes in 2-3 sentences for a user:

{DIFF}

Keep it human-readable and concise.
```

#### Branch Name Generation

```
Generate a short branch name (kebab-case) for this task:

{TASK}

Rules:
- Use prefixes: feature/, fix/, chore/, docs/
- Keep under 50 characters
- Use kebab-case
- Be descriptive but concise

Only output the branch name, nothing else.
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OLLAMA_HOST` | `http://localhost:11434` | Ollama API endpoint |
| `OLLAMA_MODEL` | `llama3.2` | Model to use |
| `GIT_COURER_WORKDIR` | `.` | Default git repository path |

### MCP Config

```json
{
  "mcp": {
    "git-courer": {
      "type": "local",
      "command": ["git-courer", "serve"]
    }
  }
}
```

## Technology Stack

- **Language**: Go 1.26+
- **MCP**: [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go)
- **LLM**: Ollama REST API
- **TUI**: [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea)
- **Git**: `os/exec` package

## Dependencies

```go
require (
	github.com/mark3labs/mcp-go v0.0.0
	github.com/charmbracelet/bubbletea v0.25.0
	github.com/charmbracelet/lipgloss v0.9.0
)
```

## Non-Functional Requirements

1. **Single binary output** - `go build` produces one executable
2. **No external runtime** - No Python, Node.js, or other dependencies
3. **Graceful degradation** - If Ollama unavailable, show clear error
4. **Fast startup** - Under 1 second to serve
5. **Memory efficient** - Under 50MB RAM

## Future Considerations

1. **Alternative LLM providers** - Swappable via LLMPort (Anthropic, OpenAI)
2. **Multiple repository support** - Configurable workdir
3. **SSH key management** - Use SSH agent for authentication
4. **Git hooks integration** - Pre-commit AI suggestions

## Testing Strategy

1. **Unit tests** - All domain services testable without external dependencies
2. **Mock adapters** - Ports can be mocked for testing
3. **Integration tests** - Test real git operations in temp directory

## Error Handling

All errors must:
1. Be user-friendly (no technical jargon in TUI)
2. Be logged for debugging
3. Allow recovery (user can retry)

## Acceptance Criteria

- [ ] MCP server starts and responds to tools
- [ ] git_status returns clean status summary
- [ ] git_commit shows TUI with AI-generated message
- [ ] User can confirm, edit, or cancel
- [ ] git_push works with confirmation
- [ ] git_branch suggests and creates branches
- [ ] Ollama integration works (or clear error if unavailable)
- [ ] Single binary builds successfully
- [ ] No tokens spent on git operations
