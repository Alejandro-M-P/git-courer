# git-courer Specification

## Concept

**git-courer** = Local Git Specialist

The cloud AI focuses on what it does best: complex code, architecture, logic.
git-courer focuses on git: mechanical, tedious, repetitive tasks.

**Objective**: Reduce tokens to minimum by delegating ALL git operations to local AI.

### Problem

Cloud AI spends tokens on:
- Reading diffs
- Generating commit messages
- Generating branch names
- Deciding what to stage
- Reading git status

All of this is mechanical work that wastes expensive tokens on complex AI.

### Solution

git-courer handles ALL git operations locally:
- Read diffs
- Detect secrets (passwords, tokens, keys)
- Generate commit messages
- Generate branch names
- Intelligent `git add` (everything EXCEPT secrets, .env, node_modules, etc.)
- Always user validates via TUI

Cloud just says "commit" → git-courer does everything → user confirms.

## Design Principles

1. **Zero tokens for git** - Cloud NEVER executes git, delegates everything
2. **User always validates** - All operations require TUI confirmation
3. **IA local for git** - All git operations done by Ollama, not cloud
4. **Secrets protection** - AI detects secrets, never stages them
5. **Clean architecture** - Hexagonal for testability
6. **Single binary** - No dependencies
7. **Multi-platform** - Works on macOS, Windows, Linux
8. **Configurable** - YAML config with sensible defaults

## Configuration

Create `git-courer.yaml` in the repository root or home directory.

```yaml
# git-courer.yaml
ollama:
  host: http://localhost:11434
  model: llama3.2
  auto_start: false  # try to start Ollama if not running

git:
  workdir: .              # default repository
  auto_add_secrets: true  # detect and skip secrets automatically
  require_clean_repo: false  # allow commit with pending changes

secrets:
  detection_mode: regex+ai  # regex first, AI to confirm
  patterns:
    - "*.key"
    - "*.pem"
    - ".env*"
    - "credentials.json"
    - "secrets.yaml"
    - "*.password"
    - "*.token"

validation:
  require_confirmation: true  # always confirm (security)
  max_commit_length: 72       # subject max chars

ui:
  theme: dark
  show_icons: true

mcp:
  name: git-courer
  version: 1.0.0
```

## MCP Tools

All git operations are handled locally by git-courer. The cloud AI only calls tools, never executes git directly.

### Category: Info
| Tool | Description |
|------|-------------|
| git_status | Current repository status |
| git_diff | Show differences |
| git_log | Commit history |
| git_show | Show commit/file details |
| git_blame | Show file blame |

### Category: Stage
| Tool | Description |
|------|-------------|
| git_add | Stage files (intelligent, skips secrets) |
| git_reset | Unstage files |

### Category: Commit
| Tool | Description |
|------|-------------|
| git_commit | Create commit with AI message |
| git_revert | Revert a commit |

### Category: Branch
| Tool | Description |
|------|-------------|
| git_branch | List/create branches |
| git_checkout | Switch branches |
| git_merge | Merge branch |

### Category: Remote
| Tool | Description |
|------|-------------|
| git_push | Push to remote |
| git_pull | Pull from remote |
| git_fetch | Fetch from remote |

### Category: Advanced
| Tool | Description |
|------|-------------|
| git_rebase | Rebase branches |
| git_stash | Stash changes |
| git_reset | Reset to commit |
| git_clean | Clean untracked files |

Each tool shows TUI confirmation before execution.

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

### git_add

Intelligently stages files, excluding secrets and unwanted files.

**Input**:
- `include_pattern` (optional): Glob pattern of files to include
- `exclude_pattern` (optional): Glob pattern of files to exclude

**Process**:
1. Reads all untracked and modified files
2. AI analyzes each file for secrets (passwords, tokens, keys, .env)
3. Excludes files matching common ignore patterns
4. Shows summary to user for validation
5. Executes `git add` for approved files

**Output**:
```
Analyzed files:
✓ config.go - staged
✓ handlers.go - staged  
✗ .env - SKIPPED (secret detected)
✗ credentials.json - SKIPPED (secret detected)
✗ node_modules/ - SKIPPED (ignored)

[s] confirm  [n] cancel
```

### git_detect_secrets

Scans changed files for potential secrets before staging.

**Input**: None

**Output**:
```
Scanning for secrets...
Files analyzed: 5
Secrets detected: 2

⚠️  config.json:15 - Potential API key detected
⚠️  .env:3 - Potential password detected

[s] show details  [n] ignore
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
