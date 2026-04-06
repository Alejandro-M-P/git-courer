package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/git"
	ollama "github.com/Alejandro-M-P/git-courer/internal/adapters/llm"
	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/domain/models"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server holds the MCP server and its dependencies
type Server struct {
	mcpServer     *server.MCPServer
	ollamaAdapter *ollama.Adapter
}

// NewServer creates a new MCP server for git-courer
func NewServer(cfg *config.Config, ollamaAdapter *ollama.Adapter) *Server {
	gitAdapter := git.NewExecAdapter(cfg.Git.WorkDir)

	s := server.NewMCPServer(
		cfg.MCP.Name,
		cfg.MCP.Version,
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)

	registerTools(s, gitAdapter, ollamaAdapter)

	// Pre-warm Ollama model in background so first commit is instant
	// This loads the model into memory while the user is still typing
	go func() {
		// Wait 2 seconds for MCP handshake to complete
		time.Sleep(2 * time.Second)
		if ollamaAdapter.IsAvailable() {
			ollamaAdapter.ResolveModel()
			ollamaAdapter.PreWarm()
		}
	}()

	return &Server{
		mcpServer:     s,
		ollamaAdapter: ollamaAdapter,
	}
}

// Stop stops Ollama if we started it
func (srv *Server) Stop() {
	if srv.ollamaAdapter != nil {
		srv.ollamaAdapter.Stop()
	}
}

// Serve starts the MCP server
func (srv *Server) Serve() {
	log.Printf("Starting git-courer MCP server")
	if err := server.ServeStdio(srv.mcpServer); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func registerTools(s *server.MCPServer, gitAdapter *git.ExecAdapter, ollamaAdapter *ollama.Adapter) {
	// git_local_task - THE ONLY ENTRY POINT (SPEC.md)
	// Cloud NEVER touches git - this is the only door
	s.AddTool(
		mcp.NewTool("git_local_task",
			mcp.WithDescription("Execute git operations from natural language. Zero tokens for git decisions."),
			mcp.WithString("instruction",
				mcp.Description("Natural language (e.g., 'commit the login changes', 'show status', 'push')"),
				mcp.Required(),
			),
		),
		handleGitLocalTask(gitAdapter, ollamaAdapter),
	)
}

// handleGitLocalTask - THE ONLY ENTRY POINT FOR GIT
// Route: read-only operations go direct (zero Ollama, zero tokens)
// Write operations use the 2-call Ollama flow
func handleGitLocalTask(gitAdapter *git.ExecAdapter, ollamaAdapter *ollama.Adapter) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		instruction := request.GetString("instruction", "")
		if instruction == "" {
			return mcp.NewToolResultError("instruction is required"), nil
		}

		// ROUTER: detect read-only operations that don't need Ollama
		if op, ok := classifyReadOnly(instruction); ok {
			return handleReadOnly(gitAdapter, op)
		}

		// WRITE operation: needs Ollama
		started, err := ollamaAdapter.EnsureOllama()
		if err != nil {
			return mcp.NewToolResultError("Ollama is required for this operation but is not available.\n\nTo fix:\n  1. Install Ollama: https://ollama.com\n  2. Run: ollama serve\n  3. Pull a model: ollama pull llama3.2\n\nOriginal instruction: " + instruction), nil
		}
		if started {
			log.Println("Ollama started by git-courer")
		}

		// FIRST CALL: Ask Ollama what context it needs
		ctxReq, err := ollamaAdapter.GetContextNeeded(instruction)
		if err != nil {
			return mcp.NewToolResultError("Failed to get context: " + err.Error()), nil
		}

		// Gather only the context Ollama asked for (locally, instant)
		context := gatherContext(gitAdapter, ctxReq)

		// SECOND CALL: Get full decision from Ollama
		decision, err := ollamaAdapter.GetFullDecision(instruction, context)
		if err != nil {
			return mcp.NewToolResultError("Failed to get decision: " + err.Error()), nil
		}

		// Validate decision before executing
		if err := validateDecision(decision); err != nil {
			return mcp.NewToolResultError("Invalid decision: " + err.Error()), nil
		}

		// Check for secrets - block if any found
		if len(decision.Secrets) > 0 {
			return mcp.NewToolResultError("SECRETS DETECTED - BLOCKED: " + formatSecrets(decision.Secrets)), nil
		}

		// Execute the decision
		result, err := executeDecision(gitAdapter, decision)
		if err != nil {
			return mcp.NewToolResultError("Execution failed: " + err.Error()), nil
		}

		// Build response
		response := map[string]interface{}{
			"result":            result,
			"summary":           decision.Summary.Action,
			"intent":            instruction,
			"executed_commands": flattenCommands(decision.Commits),
			"strategy":          decision.Strategy,
		}

		jsonResp, _ := json.Marshal(response)
		return mcp.NewToolResultText(string(jsonResp)), nil
	}
}

// readOnlyOps maps keywords to operation types
var readOnlyOps = map[string]string{
	"status":   "status",
	"estado":   "status",
	"log":      "log",
	"history":  "log",
	"historial": "log",
	"commits":  "log",
	"diff":     "diff",
	"cambios":  "diff",
	"branch":   "branches",
	"branches": "branches",
	"ramas":    "branches",
	"remote":   "remotes",
	"remotes":  "remotes",
}

// classifyReadOnly checks if instruction is a read-only operation
// Returns (operation, true) if read-only, ("", false) if needs Ollama
func classifyReadOnly(instruction string) (string, bool) {
	lower := strings.ToLower(instruction)

	// Check for explicit read operations
	if containsAny(lower, "show", "mostrar", "ver", "mostrame", "decime", "dime") {
		for keyword, op := range readOnlyOps {
			if strings.Contains(lower, keyword) {
				return op, true
			}
		}
	}

	// Also match bare keywords: "status", "log", "diff"
	for keyword, op := range readOnlyOps {
		if strings.TrimSpace(lower) == keyword {
			return op, true
		}
	}

	return "", false
}

func containsAny(s string, words ...string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// handleReadOnly executes read-only git operations without Ollama
func handleReadOnly(gitAdapter *git.ExecAdapter, op string) (*mcp.CallToolResult, error) {
	var result string
	var err error

	switch op {
	case "status":
		status, e := gitAdapter.Status()
		if e != nil {
			return mcp.NewToolResultError("Failed to get status: " + e.Error()), nil
		}
		result = formatStatus(status)

	case "log":
		result, err = gitAdapter.Log(20)
		if err != nil {
			return mcp.NewToolResultError("Failed to get log: " + err.Error()), nil
		}

	case "diff":
		result, err = gitAdapter.Diff()
		if err != nil {
			return mcp.NewToolResultError("Failed to get diff: " + err.Error()), nil
		}
		if result == "" {
			result = "No unstaged changes."
		}

	case "branches":
		branch, e := gitAdapter.CurrentBranch()
		if e != nil {
			return mcp.NewToolResultError("Failed to get current branch: " + e.Error()), nil
		}
		result = fmt.Sprintf("Current branch: %s", branch)

	case "remotes":
		// remotes not in git port, skip
		result = "Remotes: (use 'git remote -v' in terminal)"

	default:
		return mcp.NewToolResultError("Unknown read-only operation: " + op), nil
	}

	resp, _ := json.Marshal(map[string]string{
		"operation": op,
		"result":    result,
		"type":      "read",
	})
	return mcp.NewToolResultText(string(resp)), nil
}

// gatherContext collects only what Ollama asked for
func gatherContext(gitAdapter *git.ExecAdapter, ctxReq models.ContextRequest) string {
	var parts []string

	if ctxReq.StatusInfo {
		status, _ := gitAdapter.Status()
		parts = append(parts, fmt.Sprintf("Status: %s", formatStatus(status)))
	}

	if ctxReq.DiffNeeded {
		diff, _ := gitAdapter.DiffStaged()
		parts = append(parts, fmt.Sprintf("Staged Diff:\n%s", diff))
	}

	if ctxReq.BranchInfo {
		branch, _ := gitAdapter.CurrentBranch()
		parts = append(parts, fmt.Sprintf("Current branch: %s", branch))
	}

	if ctxReq.LogInfo {
		log, _ := gitAdapter.Log(10)
		parts = append(parts, fmt.Sprintf("Recent commits:\n%s", log))
	}

	return strings.Join(parts, "\n\n")
}

// validateDecision validates the decision from Ollama
func validateDecision(decision models.GitDecision) error {
	// Must have commits if type is write
	if decision.Type == "write" && len(decision.Commits) == 0 {
		return fmt.Errorf("no commits in write decision")
	}

	// Validate each commit
	for _, c := range decision.Commits {
		if c.Commit == "" {
			return fmt.Errorf("empty commit message")
		}
		for _, cmd := range c.Commands {
			if !strings.HasPrefix(cmd, "git ") {
				return fmt.Errorf("invalid command: %s", cmd)
			}
		}
	}

	return nil
}

// executeDecision executes the git commands in the decision
func executeDecision(gitAdapter *git.ExecAdapter, decision models.GitDecision) (string, error) {
	var results []string

	for _, commit := range decision.Commits {
		for _, cmd := range commit.Commands {
			// Execute command (all are git commands)
			var err error
			var output string

			switch {
			case strings.HasPrefix(cmd, "git add "):
				files := strings.TrimPrefix(cmd, "git add ")
				err = gitAdapter.Add(strings.Fields(files))
			case strings.HasPrefix(cmd, "git commit"):
				msg := extractCommitMsg(cmd)
				res, e := gitAdapter.Commit(msg)
				err, output = e, res
			case strings.HasPrefix(cmd, "git push"):
				res, e := gitAdapter.Push()
				output, err = res, e
			case strings.HasPrefix(cmd, "git pull"):
				res, e := gitAdapter.Pull()
				output, err = res, e
			case strings.HasPrefix(cmd, "git checkout "):
				branch := strings.TrimPrefix(cmd, "git checkout ")
				res, e := gitAdapter.Checkout(branch)
				output, err = res, e
			case strings.HasPrefix(cmd, "git branch "):
				branch := strings.TrimPrefix(cmd, "git branch ")
				res, e := gitAdapter.Branch(branch)
				output, err = res, e
			default:
				output = fmt.Sprintf("SKIPPED: %s (not implemented)", cmd)
			}

			if err != nil {
				return "", fmt.Errorf("%s failed: %w", cmd, err)
			}
			results = append(results, output)
		}
	}

	return strings.Join(results, "\n"), nil
}

func extractCommitMsg(cmd string) string {
	// Extract from: git commit -m "message"
	re := regexp.MustCompile(`-m\s+"([^"]+)"`)
	matches := re.FindStringSubmatch(cmd)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func flattenCommands(commits []models.CommitPlan) []string {
	var cmds []string
	for _, c := range commits {
		cmds = append(cmds, c.Commands...)
	}
	return cmds
}

func formatSecrets(secrets []models.SecretDetection) string {
	var lines []string
	for _, s := range secrets {
		lines = append(lines, fmt.Sprintf("%s:%d (%s)", s.File, s.Line, s.Type))
	}
	return strings.Join(lines, ", ")
}

// formatStatus formats git status output
func formatStatus(status models.Status) string {
	output := fmt.Sprintf("Branch: %s\n", status.Branch)

	if status.IsClean {
		output += "Status: clean\n"
	} else {
		output += fmt.Sprintf("Status: %d files changed\n", len(status.Files))
		output += fmt.Sprintf("  Staged: %d\n", status.Staged)
		output += fmt.Sprintf("  Modified: %d\n", status.Modified)
		output += fmt.Sprintf("  Untracked: %d\n", status.Untracked)
	}

	if len(status.Files) > 0 {
		output += "\nFiles:\n"
		for _, f := range status.Files {
			output += fmt.Sprintf("  %s %s\n", f.Status, f.Path)
		}
	}

	return output
}

// Serve starts the MCP server (legacy)
func Serve(cfg *config.Config) {
	ollamaAdapter := ollama.NewAdapter(cfg.Ollama.Host, cfg.Ollama.Model)
	s := NewServer(cfg, ollamaAdapter)
	log.Printf("Starting git-courer MCP server v%s", cfg.MCP.Version)
	if err := server.ServeStdio(s.mcpServer); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// ServeWithAdapter starts MCP server with pre-configured adapter
func ServeWithAdapter(cfg *config.Config, ollamaAdapter *ollama.Adapter) *Server {
	s := NewServer(cfg, ollamaAdapter)
	log.Printf("Starting git-courer MCP server v%s", cfg.MCP.Version)

	go func() {
		if err := server.ServeStdio(s.mcpServer); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	return s
}
