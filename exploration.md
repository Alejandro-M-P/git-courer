# Hexagonal Architecture Design for git-agent

## 📚 Learning Objectives

By the end of this exploration, you will understand:
- **What** Hexagonal Architecture is and **why** it matters
- **How** to define ports (interfaces) for testable code
- **How** to implement adapters that connect to external systems
- **Why** this pattern makes your code swappable and maintainable

---

## 🎯 The Problem: Tight Coupling

Imagine you write:

```go
func GenerateCommitMessage() string {
    // Direct dependency on Ollama REST API
    resp, _ := http.Post("http://localhost:11434/api/generate", ...)
    // ...
}
```

**What's wrong?**
- Can't test without a real Ollama server running
- Can't swap to Anthropic or OpenAI later without rewriting
- Business logic is coupled to infrastructure

---

## 🏗️ Hexagonal Architecture: The Solution

Also called **Ports and Adapters**, this pattern says:

> Your core business logic should know NOTHING about the outside world.
> It communicates through abstract interfaces (ports), and external systems (adapters) plug into those interfaces.

### Visual Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           YOUR APPLICATION                              │
│                                                                         │
│    ┌─────────────────────────────────────────────────────────────┐     │
│    │                      CORE DOMAIN                             │     │
│    │                  (Pure Business Logic)                       │     │
│    │                                                              │     │
│    │   - Commit message generation                               │     │
│    │   - Diff validation                                         │     │
│    │   - Git operation orchestration                             │     │
│    └─────────────────────────────────────────────────────────────┘     │
│                              ▲                                           │
│                              │                                          │
│                    ┌─────────┴─────────┐                               │
│                    │    PORTS          │  ← Interfaces (Go interfaces) │
│                    │ (Abstractions)    │                               │
│                    │                   │                               │
│                    │ - GitPort         │                               │
│                    │ - LLMPort         │                               │
│                    │ - UIPort          │                               │
│                    └─────────┬─────────┘                               │
│                              │                                          │
│         ┌────────────────────┼────────────────────┐                   │
│         │                    │                    │                   │
│         ▼                    ▼                    ▼                   │
│    ┌─────────┐         ┌─────────┐         ┌─────────┐               │
│    │ ADAPTER │         │ ADAPTER │         │ ADAPTER │               │
│    │         │         │         │         │         │               │
│    │ GitExec │         │ Ollama  │         │Bubbletea│               │
│    │  Adapter│         │ Adapter │         │ Adapter │               │
│    └─────────┘         └─────────┘         └─────────┘               │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
              │                    │                    │
              ▼                    ▼                    ▼
         os/exec             REST API              TUI
         (git CLI)           (Ollama)            (Bubbletea)
```

**Key insight**: The Core Domain sits at the center. It doesn't know HOW things are done—it only knows WHAT needs to be done, through abstract interfaces.

---

## 📦 Package Structure

```
git-agent/
├── cmd/
│   └── main.go                      # Entry point - wires everything
│
├── internal/
│   ├── domain/                      # 🎯 CORE DOMAIN (no external deps!)
│   │   ├── models/
│   │   │   ├── commit.go            # Commit value object
│   │   │   ├── diff.go              # Diff validation
│   │   │   └── message.go           # Commit message entity
│   │   └── services/
│   │       ├── commit_service.go   # Business logic: generate, validate
│   │       └── git_service.go      # Git operation orchestration
│   │
│   ├── ports/                       # 🔌 INTERFACES (abstractions)
│   │   ├── git_port.go             # What we need from Git
│   │   ├── llm_port.go             # What we need from LLM
│   │   └── ui_port.go              # What we need from UI
│   │
│   └── adapters/                    # 🔌 PLUGINS (implementations)
│       ├── git/
│       │   └── exec_adapter.go     # GitPort implementation (os/exec)
│       ├── llm/
│       │   ├── ollama_adapter.go   # LLMPort for Ollama
│       │   └── mock_adapter.go      # For testing
│       └── ui/
│           ├── bubbletea_adapter.go
│           └── cli_adapter.go
│
└── pkg/
    └── mcp/                         # MCP Server (orchestrator)
        └── server.go
```

---

## 🔌 Port 1: GitPort

The GitPort defines what the domain needs from Git—no implementation details.

### Why this matters:
- Core domain doesn't care if you use `git` CLI, libgit2, or a remote API
- You can mock it completely for testing
- Change the implementation without touching business logic

```go
// internal/ports/git_port.go
package ports

import "context"

// GitStatus represents the current state of the repository
type GitStatus struct {
    IsClean    bool
    Staged     []string   // Files with staged changes
    Unstaged   []string   // Files with unstaged changes
    Untracked  []string   // New files not tracked
    Branch     string
    IsDirty    bool
}

// DiffFile represents a file's changes
type DiffFile struct {
    Path     string
    Staged   bool   // true if in staging area
    Additions int
    Deletions int
    Content  string // The actual diff output
}

// GitPort defines the interface for Git operations
// This is what our CORE DOMAIN needs—not how it's implemented
type GitPort interface {
    
    // Status returns the current repository state
    // Returns error if not in a git repository
    Status(ctx context.Context) (GitStatus, error)
    
    // Diff returns the changes for specific files
    // If files is empty, returns all changes
    Diff(ctx context.Context, files ...string) ([]DiffFile, error)
    
    // Stage adds files to the staging area
    Stage(ctx context.Context, files ...string) error
    
    // Commit creates a commit with the given message
    Commit(ctx context.Context, message string) error
    
    // Push sends commits to remote
    Push(ctx context.Context, branch string) error
    
    // Branch returns the current branch name
    Branch(ctx context.Context) (string, error)
    
    // ListBranches returns all local branches
    ListBranches(ctx context.Context) ([]string, error)
    
    // CreateBranch creates a new branch
    CreateBranch(ctx context.Context, name string) error
    
    // Checkout switches to a branch
    Checkout(ctx context.Context, branch string) error
}
```

### Usage in Domain:

```go
// internal/domain/services/commit_service.go
package services

type CommitService struct {
    gitPort   ports.GitPort    // Dependency injected via interface
    llmPort   ports.LLMPort
}

func NewCommitService(git ports.GitPort, llm ports.LLMPort) *CommitService {
    return &CommitService{
        gitPort: git,
        llmPort: llm,
    }
}

func (s *CommitService) GenerateAndCommit(ctx context.Context) (string, error) {
    // Business logic - doesn't know about implementation!
    status, err := s.gitPort.Status(ctx)
    if err != nil {
        return "", err
    }
    
    if !status.IsDirty {
        return "Nothing to commit", nil
    }
    
    diffs, err := s.gitPort.Diff(ctx, status.Staged...)
    if err != nil {
        return "", err
    }
    
    // Generate message using LLM port - could be Ollama, Anthropic, etc.
    message, err := s.llmPort.GenerateCommitMessage(ctx, diffs)
    if err != nil {
        return "", err
    }
    
    // Commit using Git port - could be git CLI, libgit2, etc.
    if err := s.gitPort.Commit(ctx, message); err != nil {
        return "", err
    }
    
    return message, nil
}
```

---

## 🔌 Port 2: LLMPort

The LLMPort abstracts the LLM provider. Today it's Ollama, tomorrow it could be Anthropic or OpenAI.

```go
// internal/ports/llm_port.go
package ports

import "context"

// LLMRequest contains the input for LLM generation
type LLMRequest struct {
    // The diff content to analyze
    Diff string
    
    // Optional: context about the project
    Context string
    
    // Max tokens for the response
    MaxTokens int
}

// LLMResponse contains the LLM's output
type LLMResponse struct {
    // The generated commit message
    Message string
    
    // Confidence score (0.0 to 1.0)
    Confidence float64
    
    // Any additional metadata from the LLM
    Metadata map[string]interface{}
}

// LLMPort defines the interface for LLM operations
// This allows swapping between Ollama, Anthropic, OpenAI, etc.
type LLMPort interface {
    
    // GenerateCommitMessage creates a commit message from diffs
    // Returns error if LLM is unavailable or returns invalid response
    GenerateCommitMessage(ctx context.Context, req LLMRequest) (LLMResponse, error)
    
    // IsAvailable checks if the LLM service is reachable
    // Used for health checks and graceful degradation
    IsAvailable(ctx context.Context) bool
    
    // Capabilities returns what the LLM adapter supports
    Capabilities() LLMCapabilities
}

// LLMCapabilities describes what the LLM can do
type LLMCapabilities struct {
    SupportsStreaming bool
    MaxContextTokens  int
    ModelName         string
}
```

### Why this design?

```go
// You can create multiple adapters that implement the same interface:

// adapters/llm/ollama_adapter.go
type OllamaAdapter struct {
    endpoint string
    model    string
}

func (o *OllamaAdapter) GenerateCommitMessage(ctx context.Context, req LLMRequest) (LLMResponse, error) {
    // Implementation using Ollama REST API
}

// adapters/llm/anthropic_adapter.go  
type AnthropicAdapter struct {
    apiKey string
}

func (a *AnthropicAdapter) GenerateCommitMessage(ctx context.Context, req LLMRequest) (LLMResponse, error) {
    // Implementation using Anthropic API
}

// adapters/llm/mock_adapter.go (for testing!)
type MockAdapter struct {
    Response LLMResponse
    Err      error
}

func (m *MockAdapter) GenerateCommitMessage(ctx context.Context, req LLMRequest) (LLMResponse, error) {
    return m.Response, m.Err
}
```

---

## 🔌 Port 3: UIPort

The UIPort abstracts how we interact with the user. Today it's Bubbletea, tomorrow it could be a CLI or webhook-based interface.

```go
// internal/ports/ui_port.go
package ports

import "context"

// Message represents user-facing output
type Message struct {
    // The message text
    Text string
    
    // Message type for styling
    Type MessageType
    
    // Optional: associated data (for clickable items, etc.)
    Data interface{}
}

type MessageType int

const (
    MessageTypeInfo    MessageType = iota
    MessageTypeSuccess
    MessageTypeWarning
    MessageTypeError
    MessageTypeDebug
)

// UserChoice represents a selection from the user
type UserChoice struct {
    // Index of selected option
    Selected int
    
    // The selected value
    Value string
    
    // Whether user cancelled
    Cancelled bool
}

// UIPort defines the interface for user interaction
type UIPort interface {
    
    // Show displays a message to the user
    Show(ctx context.Context, msg Message) error
    
    // Ask presents a choice and returns user's selection
    Ask(ctx context.Context, prompt string, options []string) (UserChoice, error)
    
    // Confirm asks for yes/no confirmation
    Confirm(ctx context.Context, question string) (bool, error)
    
    // Progress shows a progress indicator
    // Returns a cancel function
    Progress(ctx context.Context, total int, label string) (update func(current int), done func(), err error)
    
    // Input gets text input from user
    Input(ctx context.Context, prompt string) (string, error)
}
```

---

## 🔌 Adapters Implementation

### GitExecAdapter: Implementing GitPort with os/exec

```go
// internal/adapters/git/exec_adapter.go
package git

import (
    "bytes"
    "context"
    "errors"
    "os/exec"
    "strings"
    
    "git-agent/internal/ports"
)

var (
    ErrNotARepository = errors.New("not a git repository")
)

// GitExecAdapter implements GitPort using the git CLI via os/exec
type GitExecAdapter struct {
    repoPath string
}

func NewGitExecAdapter(repoPath string) *GitExecAdapter {
    return &GitExecAdapter{repoPath: repoPath}
}

func (g *GitExecAdapter) runGit(ctx context.Context, args ...string) (string, error) {
    cmd := exec.CommandContext(ctx, "git", args...)
    cmd.Dir = g.repoPath
    
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr
    
    if err := cmd.Run(); err != nil {
        if strings.Contains(stderr.String(), "not a git repository") {
            return "", ErrNotARepository
        }
        return "", errors.New(stderr.String())
    }
    
    return strings.TrimSpace(stdout.String()), nil
}

func (g *GitExecAdapter) Status(ctx context.Context) (ports.GitStatus, error) {
    output, err := g.runGit(ctx, "status", "--porcelain")
    if err != nil {
        return ports.GitStatus{}, err
    }
    
    status := ports.GitStatus{
        Staged:   []string{},
        Unstaged: []string{},
        Untracked: []string{},
    }
    
    if output == "" {
        status.IsClean = true
        status.IsDirty = false
        return status, nil
    }
    
    status.IsDirty = true
    lines := strings.Split(output, "\n")
    
    for _, line := range lines {
        if len(line) < 3 {
            continue
        }
        
        indexStatus := line[0]
        workTreeStatus := line[1]
        filename := strings.TrimSpace(line[3:])
        
        switch indexStatus {
        case 'M', 'A', 'R', 'C':
            status.Staged = append(status.Staged, filename)
        }
        
        switch workTreeStatus {
        case 'M', 'D':
            status.Unstaged = append(status.Unstaged, filename)
        }
        
        if indexStatus == '?' && workTreeStatus == '?' {
            status.Untracked = append(status.Untracked, filename)
        }
    }
    
    // Get branch name
    branch, err := g.runGit(ctx, "rev-parse", "--abbrev-ref", "HEAD")
    if err == nil {
        status.Branch = branch
    }
    
    return status, nil
}

func (g *GitExecAdapter) Diff(ctx context.Context, files ...string) ([]ports.DiffFile, error) {
    var args []string
    
    if len(files) > 0 {
        args = append(args, "diff", "--no-color")
        args = append(args, files...)
    } else {
        args = []string{"diff", "--no-color"}
    }
    
    output, err := g.runGit(ctx, args...)
    if err != nil {
        return nil, err
    }
    
    // Parse diff output into structured DiffFile
    // (Simplified for illustration)
    diffs := []ports.DiffFile{
        {Path: "example.go", Content: output, Staged: false},
    }
    
    return diffs, nil
}

func (g *GitExecAdapter) Stage(ctx context.Context, files ...string) error {
    args := append([]string{"add"}, files...)
    _, err := g.runGit(ctx, args...)
    return err
}

func (g *GitExecAdapter) Commit(ctx context.Context, message string) error {
    _, err := g.runGit(ctx, "commit", "-m", message)
    return err
}

func (g *GitExecAdapter) Push(ctx context.Context, branch string) error {
    _, err := g.runGit(ctx, "push", "origin", branch)
    return err
}

// ... implement other methods
```

---

### OllamaAdapter: Implementing LLMPort

```go
// internal/adapters/llm/ollama_adapter.go
package llm

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "time"
    
    "git-agent/internal/ports"
)

type OllamaAdapter struct {
    endpoint string
    model    string
    client   *http.Client
}

type ollamaRequest struct {
    Model  string `json:"model"`
    Prompt string `json:"prompt"`
    Stream bool   `json:"stream"`
}

type ollamaResponse struct {
    Response string `json:"response"`
}

func NewOllamaAdapter(endpoint, model string) *OllamaAdapter {
    return &OllamaAdapter{
        endpoint: endpoint,
        model:    model,
        client: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}

func (o *OllamaAdapter) GenerateCommitMessage(ctx context.Context, req ports.LLMRequest) (ports.LLMResponse, error) {
    // Build prompt with diff context
    prompt := buildCommitPrompt(req.Diff, req.Context)
    
    oreq := ollamaRequest{
        Model:  o.model,
        Prompt: prompt,
        Stream: false,
    }
    
    body, err := json.Marshal(oreq)
    if err != nil {
        return ports.LLMResponse{}, err
    }
    
    httpReq, err := http.NewRequestWithContext(
        ctx,
        "POST",
        o.endpoint+"/api/generate",
        bytes.NewReader(body),
    )
    if err != nil {
        return ports.LLMResponse{}, err
    }
    
    resp, err := o.client.Do(httpReq)
    if err != nil {
        return ports.LLMResponse{}, err
    }
    defer resp.Body.Close()
    
    var ores ollamaResponse
    if err := json.NewDecoder(resp.Body).Decode(&ores); err != nil {
        return ports.LLMResponse{}, err
    }
    
    return ports.LLMResponse{
        Message:    parseCommitMessage(ores.Response),
        Confidence: 0.8, // Ollama doesn't provide confidence, so we estimate
    }, nil
}

func (o *OllamaAdapter) IsAvailable(ctx context.Context) bool {
    req, _ := http.NewRequestWithContext(ctx, "GET", o.endpoint+"/api/tags", nil)
    resp, err := o.client.Do(req)
    if err != nil {
        return false
    }
    return resp.StatusCode == http.StatusOK
}

func (o *OllamaAdapter) Capabilities() ports.LLMCapabilities {
    return ports.LLMCapabilities{
        SupportsStreaming: false,
        MaxContextTokens:  4096,
        ModelName:         o.model,
    }
}

func buildCommitPrompt(diff, context string) string {
    return `Generate a concise git commit message following conventional commits.
    
Context: ` + context + `

Diff:
` + diff + `

Respond with just the commit message, no explanation.`
}

func parseCommitMessage(response string) string {
    // Clean up the response - take first line, remove quotes
    lines := []byte(response)
    firstLine := bytes.SplitN(lines, []byte("\n"))[0]
    return string(bytes.Trim(firstLine, "\" "))
}
```

---

## 🔌 BubbleteaAdapter: Implementing UIPort

```go
// internal/adapters/ui/bubbletea_adapter.go
package ui

import (
    "context"
    
    "git-agent/internal/ports"
)

// BubbleteaAdapter wraps bubbletea.Model for ports.UIPort
// This allows your core domain to be UI-agnostic
type BubbleteaAdapter struct {
    model tea.Model
}

func NewBubbleteaAdapter(model tea.Model) *BubbleteaAdapter {
    return &BubbleteaAdapter{model: model}
}

func (b *BubbleteaAdapter) Show(ctx context.Context, msg ports.Message) error {
    // Convert ports.Message to bubbletea message and send to model
    // This is a simplified implementation
    return nil
}

func (b *BubbleteaAdapter) Ask(ctx context.Context, prompt string, options []string) (ports.UserChoice, error) {
    // Implement selection UI
    return ports.UserChoice{}, nil
}

func (b *BubbleteaAdapter) Confirm(ctx context.Context, question string) (bool, error) {
    // Implement yes/no UI
    return true, nil
}

func (b *BubbleteaAdapter) Progress(ctx context.Context, total int, label string) (update func(int), done func(), err error) {
    // Implement progress bar UI
    return nil, nil, nil
}

func (b *BubbleteaAdapter) Input(ctx context.Context, prompt string) (string, error) {
    // Implement text input UI
    return "", nil
}
```

---

## 🎯 The Orchestrator: MCP Server

The MCP Server (Model Context Protocol) acts as the application orchestrator. It wires together the ports and adapters, but the business logic remains in the domain.

```go
// cmd/main.go
package main

import (
    "context"
    "log"
    
    "git-agent/internal/adapters/git"
    "git-agent/internal/adapters/llm"
    "git-agent/internal/adapters/ui"
    "git-agent/internal/domain/services"
    "git-agent/internal/ports"
    "git-agent/pkg/mcp"
)

func main() {
    // 1. Create adapters (infrastructure)
    gitAdapter := git.NewGitExecAdapter(".")
    llmAdapter := llm.NewOllamaAdapter("http://localhost:11434", "codellama")
    // uiAdapter := ui.NewBubbleteaAdapter(myModel)  // Or use CLI adapter
    
    // 2. Inject adapters into domain services (constructor injection)
    commitService := services.NewCommitService(gitAdapter, llmAdapter)
    gitService := services.NewGitService(gitAdapter)
    
    // 3. Create MCP server with the services
    server := mcp.NewServer(commitService, gitService)
    
    // 4. Run the server
    if err := server.Run(context.Background()); err != nil {
        log.Fatal(err)
    }
}
```

### MCP Server Structure

```go
// pkg/mcp/server.go
package mcp

import (
    "context"
    
    "git-agent/internal/domain/services"
    "git-agent/internal/ports"
)

// Server is the MCP orchestrator - it connects adapters through ports to domain
type Server struct {
    commitService *services.CommitService
    gitService    *services.GitService
}

func NewServer(commit *services.CommitService, git *services.GitService) *Server {
    return &Server{
        commitService: commit,
        gitService:    git,
    }
}

// Run starts the MCP server
func (s *Server) Run(ctx context.Context) error {
    // This would typically start an MCP server
    // that exposes tools like git_status, git_commit, etc.
    // Each tool call goes through the domain services
    return nil
}

// ToolHandler demonstrates how MCP tools use the domain
func (s *Server) HandleTool(ctx context.Context, tool string, args map[string]interface{}) (interface{}, error) {
    switch tool {
    case "git_status":
        return s.gitService.GetStatus(ctx)
    case "git_commit":
        message := args["message"].(string)
        return s.commitService.CommitWithMessage(ctx, message)
    case "ai_commit":
        return s.commitService.GenerateAndCommit(ctx)
    default:
        return nil, ports.ErrUnknownTool
    }
}
```

---

## 🧪 Why This Is Testable

Because everything goes through interfaces, testing is trivial:

```go
// internal/domain/services/commit_service_test.go
package services

import (
    "context"
    "testing"
    
    "git-agent/internal/ports"
)

// Mock adapters for testing
type mockGitAdapter struct {
    statusResponse ports.GitStatus
    diffResponse   []ports.DiffFile
    commitCalled   bool
    commitMessage  string
}

func (m *mockGitAdapter) Status(ctx context.Context) (ports.GitStatus, error) {
    return m.statusResponse, nil
}

func (m *mockGitAdapter) Diff(ctx context.Context, files ...string) ([]ports.DiffFile, error) {
    return m.diffResponse, nil
}

func (m *mockGitAdapter) Commit(ctx context.Context, message string) error {
    m.commitCalled = true
    m.commitMessage = message
    return nil
}

// ... implement other methods to return expected values

type mockLLMAdapter struct {
    response ports.LLMResponse
}

func (m *mockLLMAdapter) GenerateCommitMessage(ctx context.Context, req ports.LLMRequest) (ports.LLMResponse, error) {
    return m.response, nil
}

func (m *mockLLMAdapter) IsAvailable(ctx context.Context) bool {
    return true
}

func (m *mockLLMAdapter) Capabilities() ports.LLMCapabilities {
    return ports.LLMCapabilities{ModelName: "mock"}
}

// Test without any external dependencies!
func TestCommitService_GenerateAndCommit(t *testing.T) {
    // Setup mock adapters
    gitMock := &mockGitAdapter{
        statusResponse: ports.GitStatus{IsDirty: true, Staged: []string{"main.go"}},
        diffResponse:   []ports.DiffFile{{Path: "main.go", Content: "+fmt.Println()"}},
    }
    
    llmMock := &mockLLMAdapter{
        response: ports.LLMResponse{Message: "feat: add logging"},
    }
    
    // Create service with mocks
    service := NewCommitService(gitMock, llmMock)
    
    // Execute
    msg, err := service.GenerateAndCommit(context.Background())
    
    // Assert
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    
    if msg != "feat: add logging" {
        t.Errorf("expected message 'feat: add logging', got '%s'", msg)
    }
    
    if !gitMock.commitCalled {
        t.Error("expected Commit to be called")
    }
    
    if gitMock.commitMessage != "feat: add logging" {
        t.Errorf("expected commit message 'feat: add logging', got '%s'", gitMock.commitMessage)
    }
}
```

**No external dependencies in tests!** You can run this without:
- A git repository
- An Ollama server
- Any network connection

---

## 🔄 Swappable Adapters: The Real Power

### Adding Anthropic Later

```go
// internal/adapters/llm/anthropic_adapter.go
package llm

import (
    "context"
    
    "github.com/anthropic-ai/sdk-go"
    
    "git-agent/internal/ports"
)

type AnthropicAdapter struct {
    client *anthropic.Client
    model  string
}

func NewAnthropicAdapter(apiKey, model string) *AnthropicAdapter {
    return &AnthropicAdapter{
        client: anthropic.NewClient(anthropic.WithAPIKey(apiKey)),
        model:  model,
    }
}

func (a *AnthropicAdapter) GenerateCommitMessage(ctx context.Context, req ports.LLMRequest) (ports.LLMResponse, error) {
    resp, err := a.client.CreateMessage(ctx, anthropic.MessageCreateParams{
        Model: a.model,
        MaxTokens: 1024,
        Messages: []anthropic.MessageParam{
            {
                Role: anthropic.MessageRoleUser,
                Content: []anthropic.ContentBlock{
                    {Type: anthropic.ContentBlockTypeText, Text: req.Diff},
                },
            },
        },
    })
    
    if err != nil {
        return ports.LLMResponse{}, err
    }
    
    return ports.LLMResponse{
        Message:    resp.Content[0].Text,
        Confidence: 0.95, // Anthropic provides better confidence
    }, nil
}

// Same interface means ZERO changes to domain!
func (a *AnthropicAdapter) IsAvailable(ctx context.Context) bool {
    return a.client != nil
}

func (a *AnthropicAdapter) Capabilities() ports.LLMCapabilities {
    return ports.LLMCapabilities{
        SupportsStreaming: true,
        MaxContextTokens:  200000,
        ModelName:         a.model,
    }
}
```

### Switching Implementation

```go
// In main.go - just change one line:
func main() {
    // Before (Ollama):
    // llmAdapter := llm.NewOllamaAdapter("http://localhost:11434", "codellama")
    
    // After (Anthropic) - same interface!
    llmAdapter := llm.NewAnthropicAdapter(os.Getenv("ANTHROPIC_API_KEY"), "claude-3")
    
    // Domain code NEVER changes!
    commitService := services.NewCommitService(gitAdapter, llmAdapter)
}
```

---

## 📊 Summary: Key Design Principles

| Principle | How It's Applied |
|-----------|------------------|
| **Dependency Inversion** | Domain defines interfaces (ports), adapters implement them |
| **Single Responsibility** | Each adapter does ONE thing (Git, LLM, or UI) |
| **Interface Segregation** | Ports are small, focused interfaces |
| **Open/Closed** | Add new adapters without modifying domain |
| **Testability** | Mock any port for unit tests |

---

## 🎓 Key Takeaways

1. **Ports are the contract** - They define what the domain NEEDS, not how it's done
2. **Adapters are the implementation** - They connect to external systems
3. **Domain is pure** - No imports from external packages (no `net/http`, no `os/exec`)
4. **Constructor injection** - Dependencies flow in through the constructor
5. **Swappable by design** - Change the adapter, domain code stays the same

---

## 🔗 Further Reading

- [Alam Rafiul: Hexagonal Architecture in Go](https://alamrafiul.com/posts/go-hexagonal-architecture/)
- [DEV.to: Ports and Adapters](https://dev.to/elpic/ports-and-adapters-the-pattern-your-architecture-has-been-missing-1j43)
- [felipecrs GitHub: go-hexagonal](https://github.com/felipewom/go-hexagonal)
- [Alistair Cockburn: Hexagonal Architecture](https://alistair.cockburn.us/hexagonal-architecture/)