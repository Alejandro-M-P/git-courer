package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	gitadapter "github.com/Alejandro-M-P/git-courer/internal/adapters/git"
	ollama "github.com/Alejandro-M-P/git-courer/internal/adapters/llm"
	"github.com/Alejandro-M-P/git-courer/internal/classifier"
	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/domain/models"
	gitport "github.com/Alejandro-M-P/git-courer/internal/ports/git"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server holds the MCP server and its dependencies
type Server struct {
	mcpServer     *server.MCPServer
	ollamaAdapter *ollama.Adapter
	// Mutex to prevent multiple concurrent git operations
	mu sync.Mutex
}

// NewServer creates a new MCP server for git-courer
func NewServer(cfg *config.Config, ollamaAdapter *ollama.Adapter) *Server {
	gitAdapter := gitadapter.NewExecAdapter(cfg.Git.WorkDir)

	s := server.NewMCPServer(
		cfg.MCP.Name,
		cfg.MCP.Version,
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)

	srv := &Server{
		mcpServer:     s,
		ollamaAdapter: ollamaAdapter,
	}

	// Register tools ONCE with the same srv instance (mutex is now protected)
	registerTools(s, srv, gitAdapter, ollamaAdapter)

	// Start Ollama and pre-warm model synchronously at startup
	// This ensures Ollama is ready when first request comes in
	log.Println("Starting Ollama...")
	started, err := ollamaAdapter.EnsureOllama()
	if err != nil {
		log.Printf("⚠ Warning: Ollama not available: %v", err)
		log.Printf("  Write operations will retry starting Ollama automatically")
	} else {
		if started {
			log.Println("✓ Ollama started by git-courer")
		}
		if err := ollamaAdapter.PreWarm(); err != nil {
			log.Printf("⚠ Failed to pre-warm model: %v", err)
		} else {
			log.Printf("✓ Model ready for instant commits")
		}
	}

	return srv
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

func registerTools(s *server.MCPServer, srv *Server, gitAdapter gitport.Port, ollamaAdapter *ollama.Adapter) {
	// git_do - THE ONLY ENTRY POINT (SPEC.md)
	// Cloud NEVER touches git - this is the only door
	// ⚠️ ONE CALL PER REQUEST: Multiple calls will be rejected to prevent orchestrator from "thinking" first
	s.AddTool(
		mcp.NewTool("git_do",
			mcp.WithDescription("Execute git operations from natural language. Zero tokens. ONE CALL PER REQUEST - if you call this tool more than once, you will get an error. DO NOT analyze, think, or plan - just EXECUTE the instruction exactly as given."),
			mcp.WithString("instruction",
				mcp.Description("Natural language (e.g., 'commit the changes', 'show status', 'push')"),
				mcp.Required(),
			),
		),
		handleGitLocalTask(srv, gitAdapter, ollamaAdapter),
	)

	// ALIAS: Accept both git_do and git_local_task for backwards compatibility
	s.AddTool(
		mcp.NewTool("git_local_task",
			mcp.WithDescription("Execute git operations from natural language. Zero tokens. ONE CALL PER REQUEST. Alias for git_do."),
			mcp.WithString("instruction",
				mcp.Description("Natural language (e.g., 'commit the changes', 'show status', 'push')"),
				mcp.Required(),
			),
		),
		handleGitLocalTask(srv, gitAdapter, ollamaAdapter),
	)
}

// handleGitLocalTask - THE ONLY ENTRY POINT FOR GIT
// Route: read-only operations go direct (zero Ollama, zero tokens)
// Write operations use the 2-call Ollama flow
// ⚠️ ONE CALL PER REQUEST - uses mutex to prevent concurrent calls
func handleGitLocalTask(srv *Server, gitAdapter gitport.Port, ollamaAdapter *ollama.Adapter) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		instruction := request.GetString("instruction", "")
		if instruction == "" {
			return mcp.NewToolResultError("instruction is required"), nil
		}

		// 🚫 BLOCK MULTIPLE CALLS: Only one git operation at a time
		if !srv.mu.TryLock() {
			return mcp.NewToolResultError("⚠️ ERROR: Only ONE git_do call allowed per request.\n\nPass the user's complete intent in a SINGLE call. git-courer handles everything internally."), nil
		}
		defer srv.mu.Unlock()

		// Classify using the real classifier (bilingual, tested)
		intentResult := classifier.Classify(instruction)

		// Map classifier result to IntentType for routing
		intentType := mapClassifierResultToIntent(intentResult)

		var callResult *mcp.CallToolResult
		var err error

		// Route based on intent type
		switch intentType {
		case IntentQuery:
			// Read-only operations (status, log, diff, branches)
			callResult, err = handleReadOnly(gitAdapter, intentResult.Operation)
		case IntentCreate, IntentModify:
			// Simple write operations (branch, checkout, stash, etc.)
			// Map to action strings that handleDirectOp understands
			action := intentResult.Operation
			if intentResult.Operation == "branch" {
				action = "create-branch"
			}
			callResult, err = handleDirectOp(gitAdapter, action, instruction)
		case IntentWrite, IntentPassthrough:
			// Complex write (commit) or unknown → use Ollama path
			// For now, fall back to direct operations based on operation type
			if intentResult.Operation == "commit" {
				// Commit needs Ollama - route to handleWrite when implemented
				// For now, fall back to direct commit
				callResult, err = handleDirectOp(gitAdapter, "commit", instruction)
			} else {
				callResult, err = handleDirectOp(gitAdapter, intentResult.Operation, instruction)
			}
		default:
			// Unknown intent → try direct passthrough
			callResult, err = handleDirectOp(gitAdapter, intentResult.Operation, instruction)
		}

		// Note: savings are tracked internally but displayed via ollamaAdapter.GetStats()
		// when the agent queries token usage

		return callResult, err
	}
}

// mapClassifierResultToIntent maps the real classifier result to IntentType
func mapClassifierResultToIntent(result classifier.Result) IntentType {
	switch result.Category {
	case classifier.ReadOnly:
		return IntentQuery
	case classifier.SimpleWrite:
		// Map simple write operations to Create/Modify/Write
		switch result.Operation {
		case "commit":
			return IntentWrite
		case "push", "pull":
			return IntentWrite
		case "branch", "tag", "stash":
			return IntentCreate
		case "checkout", "reset", "revert", "restore", "clean", "fetch":
			return IntentModify
		default:
			return IntentModify
		}
	case classifier.ComplexWrite:
		// Commit, merge, rebase need Ollama
		return IntentWrite
	case classifier.Unknown:
		// Fallback to write path (will try Ollama)
		return IntentWrite
	default:
		return IntentPassthrough
	}
}

// handleDirectOp executes git operations directly without Ollama
func handleDirectOp(gitAdapter gitport.Port, action, instruction string) (*mcp.CallToolResult, error) {
	var result string
	var created string // What was created/done (for user feedback)
	var err error

	switch action {
	case "push":
		result, err = gitAdapter.Push()
		created = "pushed to remote"
	case "pull":
		result, err = gitAdapter.Pull()
		created = "pulled from remote"
	case "create-branch":
		// Extract branch name from instruction
		branch := extractBranchName(instruction)
		if branch == "" {
			return mcp.NewToolResultError("Could not determine branch name"), nil
		}
		result, err = gitAdapter.Branch(branch)
		created = fmt.Sprintf("branch '%s'", branch)
	case "checkout":
		// Extract branch name from instruction
		branch := extractBranchName(instruction)
		if branch == "" {
			return mcp.NewToolResultError("Could not determine branch name"), nil
		}
		result, err = gitAdapter.Checkout(branch)
		created = fmt.Sprintf("switched to '%s'", branch)
	case "stash":
		result, err = gitAdapter.Stash()
		created = "changes stashed"
	case "reset":
		// Extract commit/branch to reset to
		target := extractResetTarget(instruction)
		result, err = gitAdapter.Reset("--hard", target)
		created = fmt.Sprintf("reset to '%s'", target)
	default:
		return mcp.NewToolResultError("Unknown operation: " + action), nil
	}

	if err != nil {
		return mcp.NewToolResultError(action + " failed: " + err.Error()), nil
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"operation": action,
		"created":   created,
		"result":    result,
		"type":      "direct",
		"tokens":    0, // Direct git operations don't use Ollama tokens
	})
	return mcp.NewToolResultText(string(resp)), nil
}

// extractBranchName extracts branch name from instruction
// Uses ordered prefix matching + validation to avoid extracting garbage
func extractBranchName(instruction string) string {
	lower := strings.ToLower(instruction)

	// Ordered prefix list (longest first to avoid partial matches)
	prefixes := []string{
		"create branch ",
		"new branch ",
		"checkout to ",
		"checkout ",
		"switch to ",
		"switch ",
		"make branch ",
		"crea branch ",
		"crear branch ",
		"rama ",
		"branch ",
	}

	var afterPrefix string
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			afterPrefix = strings.TrimSpace(lower[len(prefix):])
			break
		}
	}

	if afterPrefix == "" {
		// No prefix matched, try to find "called" or similar pattern
		// e.g. "create a branch called feature/auth"
		if idx := strings.Index(lower, "called "); idx >= 0 {
			afterPrefix = strings.TrimSpace(lower[idx+7:])
		} else if idx := strings.Index(lower, "llamado "); idx >= 0 {
			afterPrefix = strings.TrimSpace(lower[idx+8:])
		} else {
			// Fallback: trim common filler words from the start
			afterPrefix = lower
		}
	}

	// Remove surrounding quotes
	afterPrefix = strings.Trim(afterPrefix, "\"'")

	// Validate: branch names should be reasonable (alphanumeric, slashes, hyphens, underscores)
	// If result looks like garbage (too long, contains weird chars), return empty
	if len(afterPrefix) > 100 || strings.ContainsAny(afterPrefix, "!@#$%^&*()=+[]{}|\\:;<>?") {
		return ""
	}

	return afterPrefix
}

// extractResetTarget extracts reset target from instruction
func extractResetTarget(instruction string) string {
	lower := strings.ToLower(instruction)
	// Look for commit hash or branch name after "reset"
	parts := strings.Split(lower, "reset")
	if len(parts) < 2 {
		return "HEAD~1"
	}
	target := strings.TrimSpace(parts[1])
	target = strings.Trim(target, "\"")
	if target == "" {
		return "HEAD~1"
	}
	return target
}

// handleReadOnly executes read-only git operations without Ollama
func handleReadOnly(gitAdapter gitport.Port, op string) (*mcp.CallToolResult, error) {
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

	resp, _ := json.Marshal(map[string]interface{}{
		"operation": op,
		"result":    result,
		"type":      "read",
		"tokens":    0, // Read operations don't use tokens
	})
	return mcp.NewToolResultText(string(resp)), nil
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
	ollamaAdapter := ollama.NewAdapter(cfg.Ollama.Host, cfg.Ollama.Model, cfg.Ollama.ModelsDir)
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
