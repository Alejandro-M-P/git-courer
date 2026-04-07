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
			mcp.WithBoolean("preview",
				mcp.Description("If true, returns preview without executing (requires confirmation)"),
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

		// Get preview flag (defaults to false if not provided)
		preview := request.GetBool("preview", false)

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
			if intentResult.Operation == "commit" {
				// Commit with Ollama-generated message
				callResult, err = handleWrite(gitAdapter, ollamaAdapter, instruction, preview)
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
		// If push rejected because no upstream, set it up and retry
		if err != nil && (strings.Contains(err.Error(), "no upstream") || strings.Contains(err.Error(), "no tracking information") || strings.Contains(err.Error(), "no hay informaci")) {
			// Get current branch name
			branch, branchErr := gitAdapter.CurrentBranch()
			if branchErr != nil {
				return mcp.NewToolResultError("Push failed: could not determine branch"), nil
			}
			// Try to set upstream using PushWithUpstream method
			result, err = gitAdapter.PushWithUpstream(branch)
		}
		// If push rejected (remote has new commits), pull and try again
		if err != nil && strings.Contains(err.Error(), "PUSH_REJECTED") {
			// Try pull with rebase first
			pullResult, pullErr := gitAdapter.PullRebase()
			if pullErr != nil {
				// If rebase fails, try regular pull
				pullResult, pullErr = gitAdapter.Pull()
				if pullErr != nil {
					return mcp.NewToolResultError("Push rejected and pull failed: " + pullErr.Error()), nil
				}
			}
			// Retry push
			result, err = gitAdapter.Push()
			if err != nil {
				return mcp.NewToolResultError("Push failed after pull: " + err.Error()), nil
			}
			result = pullResult + "\n" + result
		}
		created = "pushed to remote"
	case "pull":
		result, err = gitAdapter.Pull()
		created = "pulled from remote"
	case "fetch":
		result, err = gitAdapter.Fetch()
		created = "fetched from remote"
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
	case "merge":
		branch := extractBranchName(instruction)
		if branch == "" {
			return mcp.NewToolResultError("Could not determine branch name to merge"), nil
		}
		result, err = gitAdapter.Merge(branch)
		created = fmt.Sprintf("merged '%s'", branch)
	case "rebase":
		branch := extractBranchName(instruction)
		if branch == "" {
			return mcp.NewToolResultError("Could not determine branch to rebase onto"), nil
		}
		result, err = gitAdapter.Rebase(branch)
		created = fmt.Sprintf("rebased onto '%s'", branch)
	case "cherry-pick":
		commit := extractCommitHash(instruction)
		if commit == "" {
			return mcp.NewToolResultError("Could not determine commit to cherry-pick"), nil
		}
		result, err = gitAdapter.CherryPick(commit)
		created = fmt.Sprintf("cherry-picked '%s'", commit)
	case "clean":
		result, err = gitAdapter.Clean(true)
		created = "cleaned untracked files"
	case "tag":
		tag := extractTagName(instruction)
		if tag == "" {
			return mcp.NewToolResultError("Could not determine tag name"), nil
		}
		result, err = gitAdapter.Tag(tag)
		created = fmt.Sprintf("created tag '%s'", tag)
	case "delete-branch":
		branch := extractBranchName(instruction)
		if branch == "" {
			return mcp.NewToolResultError("Could not determine branch name to delete"), nil
		}
		result, err = gitAdapter.DeleteBranch(branch)
		created = fmt.Sprintf("deleted branch '%s'", branch)
	case "blame":
		file := extractFileName(instruction)
		if file == "" {
			return mcp.NewToolResultError("Could not determine file to blame"), nil
		}
		result, err = gitAdapter.Blame(file)
		created = fmt.Sprintf("blame for '%s'", file)
	case "add":
		files := extractFilesToAdd(instruction)
		if len(files) == 0 {
			return mcp.NewToolResultError("Could not determine files to add"), nil
		}
		err = gitAdapter.Add(files)
		if err != nil {
			return mcp.NewToolResultError("Add failed: " + err.Error()), nil
		}
		result = fmt.Sprintf("staged %d file(s)", len(files))
		created = "staged files"
	case "revert":
		commit := extractCommitHash(instruction)
		if commit == "" {
			return mcp.NewToolResultError("Could not determine commit to revert"), nil
		}
		result, err = gitAdapter.Revert(commit)
		created = fmt.Sprintf("reverted '%s'", commit)
	case "reflog":
		result, err = gitAdapter.Reflog(20)
		created = "showed reflog"
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

// handleWrite handles commit operations using Ollama to generate commit messages
// If preview is true, returns preview JSON without executing commits
func handleWrite(gitAdapter gitport.Port, ollamaAdapter *ollama.Adapter, instruction string, preview bool) (*mcp.CallToolResult, error) {
	// Get changed files
	status, err := gitAdapter.Status()
	if err != nil {
		return mcp.NewToolResultError("Failed to get status: " + err.Error()), nil
	}

	if len(status.Files) == 0 {
		resp, _ := json.Marshal(map[string]interface{}{
			"operation": "commit",
			"result":    "Nothing to commit",
			"type":      "write",
			"tokens":    0,
		})
		return mcp.NewToolResultText(string(resp)), nil
	}

	// Get list of changed files
	var files []string
	for _, f := range status.Files {
		files = append(files, f.Path)
	}

	diff, _ := gitAdapter.Diff()

	// Use AI to analyze and plan the commit
	analysis, err := ollamaAdapter.AnalyzeAndPlanCommit(files, diff)
	if err != nil {
		return mcp.NewToolResultError("Analysis failed: " + err.Error()), nil
	}

	// Nothing safe to commit
	if len(analysis.Commits) == 0 {
		var excluded []string
		for _, e := range analysis.Excluded {
			excluded = append(excluded, fmt.Sprintf("  %s → %s", e.File, e.Reason))
		}
		return mcp.NewToolResultError("⚠️ Nothing safe to commit. Excluded:\n" + strings.Join(excluded, "\n")), nil
	}

	// 🚨 PREVIEW MODE: Return analysis without executing
	if preview {
		// Build excluded files list
		var excludedFiles []string
		for _, e := range analysis.Excluded {
			excludedFiles = append(excludedFiles, e.File)
		}

		// Build commits preview
		var commitsPreview []map[string]interface{}
		for _, commit := range analysis.Commits {
			commitsPreview = append(commitsPreview, map[string]interface{}{
				"message": commit.Message,
				"files":   commit.Files,
			})
		}

		// Build warnings (already []string)
		warnings := analysis.Warnings

		// Build preview response
		previewResp := map[string]interface{}{
			"preview":   true,
			"operation": "commit",
			"commits":   commitsPreview,
			"excluded":  excludedFiles,
			"warnings":  warnings,
			"options": []map[string]interface{}{
				{"label": "Confirmar", "action": "execute"},
				{"label": "Regenerar mensaje", "action": "feedback"},
				{"label": "Editar mensaje", "action": "edit"},
				{"label": "Mi propio mensaje", "action": "custom"},
			},
		}

		respBytes, _ := json.Marshal(previewResp)
		return mcp.NewToolResultText(string(respBytes)), nil
	}

	// Execute each commit with rollback on failure
	var committed []string
	for i, commit := range analysis.Commits {
		if err := gitAdapter.Add(commit.Files); err != nil {
			// Rollback previous commits
			for range committed {
				gitAdapter.Reset("--soft", "HEAD~1")
			}
			return mcp.NewToolResultError(fmt.Sprintf("Failed to stage commit %d: %s", i+1, err.Error())), nil
		}

		if _, err := gitAdapter.Commit(commit.Message); err != nil {
			for range committed {
				gitAdapter.Reset("--soft", "HEAD~1")
			}
			return mcp.NewToolResultError(fmt.Sprintf("Failed commit %d: %s", i+1, err.Error())), nil
		}

		committed = append(committed, commit.Message)
	}

	// Push if instruction contains push
	resp := map[string]interface{}{
		"operation": "commit",
		"commits":   committed,
		"excluded":  analysis.Excluded,
		"warnings":  analysis.Warnings,
		"type":      "write",
	}

	if strings.Contains(strings.ToLower(instruction), "push") {
		pushResult, pushErr := gitAdapter.Push()
		if pushErr != nil {
			return mcp.NewToolResultError(pushErr.Error()), nil
		}
		resp["push"] = pushResult
	}

	respBytes, _ := json.Marshal(resp)
	return mcp.NewToolResultText(string(respBytes)), nil
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

// extractCommitHash extracts commit hash from instruction
func extractCommitHash(instruction string) string {
	lower := strings.ToLower(instruction)
	// Look for hash after "cherry-pick" or "revert"
	patterns := []string{"cherry-pick ", "revert "}
	for _, pattern := range patterns {
		if idx := strings.Index(lower, pattern); idx >= 0 {
			hash := strings.TrimSpace(lower[idx+len(pattern):])
			hash = strings.Trim(hash, "\"")
			// Hash should be short (7+ chars) or full
			if len(hash) >= 4 && len(hash) <= 40 {
				return hash
			}
		}
	}
	// Try to find any hex string that looks like a commit hash
	parts := strings.Fields(instruction)
	for _, part := range parts {
		part = strings.Trim(part, "\"")
		if len(part) >= 4 && len(part) <= 40 {
			// Check if it's hex
			isHex := true
			for _, c := range part {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
					isHex = false
					break
				}
			}
			if isHex {
				return part
			}
		}
	}
	return ""
}

// extractTagName extracts tag name from instruction
func extractTagName(instruction string) string {
	lower := strings.ToLower(instruction)
	// Look for tag name after "tag" or "create tag"
	patterns := []string{"tag ", "create tag ", "new tag ", "make tag "}
	for _, pattern := range patterns {
		if idx := strings.Index(lower, pattern); idx >= 0 {
			tag := strings.TrimSpace(lower[idx+len(pattern):])
			tag = strings.Trim(tag, "\"")
			if tag != "" && len(tag) <= 100 {
				return tag
			}
		}
	}
	return ""
}

// extractFileName extracts file name from instruction
func extractFileName(instruction string) string {
	lower := strings.ToLower(instruction)
	// Look for file path after "blame"
	if idx := strings.Index(lower, "blame"); idx >= 0 {
		file := strings.TrimSpace(lower[idx+5:])
		file = strings.Trim(file, "\"")
		if file != "" {
			return file
		}
	}
	// Look for common file patterns
	words := strings.Fields(instruction)
	for _, word := range words {
		word = strings.Trim(word, "\"")
		if strings.Contains(word, ".") && !strings.Contains(word, " ") {
			return word
		}
	}
	return ""
}

// extractFilesToAdd extracts files to add from instruction
func extractFilesToAdd(instruction string) []string {
	lower := strings.ToLower(instruction)
	var files []string
	// Look for "add" or "stage" followed by file names
	patterns := []string{"add ", "stage "}
	for _, pattern := range patterns {
		if idx := strings.Index(lower, pattern); idx >= 0 {
			filesStr := strings.TrimSpace(lower[idx+len(pattern):])
			filesStr = strings.Trim(filesStr, "\"")
			// Split by spaces
			parts := strings.Fields(filesStr)
			for _, f := range parts {
				f = strings.Trim(f, "\"")
				if f != "" && f != "." {
					files = append(files, f)
				}
			}
		}
	}
	if len(files) == 0 {
		// Default to all files
		files = append(files, ".")
	}
	return files
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

	case "reflog":
		result, err = gitAdapter.Reflog(20)
		if err != nil {
			return mcp.NewToolResultError("Failed to get reflog: " + err.Error()), nil
		}

	case "blame":
		return mcp.NewToolResultError("Blame is not a read operation - use 'git blame <file>' directly"), nil

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
