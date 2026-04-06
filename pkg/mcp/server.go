package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"

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
	// Mutex to prevent multiple concurrent git operations
	mu sync.Mutex
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

	srv := &Server{
		mcpServer:     s,
		ollamaAdapter: ollamaAdapter,
	}

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

func registerTools(s *server.MCPServer, srv *Server, gitAdapter *git.ExecAdapter, ollamaAdapter *ollama.Adapter) {
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
func handleGitLocalTask(srv *Server, gitAdapter *git.ExecAdapter, ollamaAdapter *ollama.Adapter) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

		// Classify the operation
		opType, action := classifyOperation(instruction)

		// READ operations: no Ollama needed
		if opType == opRead {
			if op, ok := classifyReadOnly(instruction); ok {
				return handleReadOnly(gitAdapter, op)
			}
		}

		// DIRECT operations: no Ollama needed, execute directly
		if opType == opDirect {
			return handleDirectOp(gitAdapter, action, instruction)
		}

		// COMMIT operations: need Ollama for message and grouping
		// Ensure Ollama is running
		started, err := ollamaAdapter.EnsureOllama()
		if err != nil {
			return mcp.NewToolResultError("Ollama is required for this operation but is not available.\n\nTo fix:\n  1. Install Ollama: https://ollama.com\n  2. Run: ollama serve\n  3. Pull a model: ollama pull qwen3.5\n\nOriginal instruction: " + instruction), nil
		}
		if started {
			log.Println("Ollama started by git-courer")
		}

		// Check if model is ready
		if !ollamaAdapter.IsModelReady() {
			return mcp.NewToolResultError("🤖 Model is loading... Please retry in 30 seconds."), nil
		}

		// Get actual file list and diff from git
		files, err := getGitFiles(gitAdapter)
		if err != nil {
			return mcp.NewToolResultError("Failed to get git status: " + err.Error()), nil
		}

		// Get diff for analysis
		diff, _ := gitAdapter.Diff()

		// Decide: single commit vs multiple commits based on file count
		var msgs []string
		if len(files) <= 10 {
			// Single commit with diff analysis
			msg, err := ollamaAdapter.GenerateCommitMessageFromDiff(instruction, files, diff)
			if err != nil {
				return mcp.NewToolResultError("Failed to generate commit message: " + err.Error()), nil
			}
			msgs = []string{msg}
		} else {
			// Multiple commits - ask Ollama to group files
			msgs, err = ollamaAdapter.GenerateMultipleCommitMessages(instruction, files, diff)
			if err != nil {
				return mcp.NewToolResultError("Failed to generate commit messages: " + err.Error()), nil
			}
		}

		// Execute git add -A && git commit -m "msg" && git push
		var results []string

		// Add all files
		if err := gitAdapter.Add([]string{"-A"}); err != nil {
			return mcp.NewToolResultError("git add failed: " + err.Error()), nil
		}
		results = append(results, "git add -A")

		// Commit each message
		for _, msg := range msgs {
			commitResult, err := gitAdapter.Commit(msg)
			if err != nil {
				return mcp.NewToolResultError("git commit failed: " + err.Error()), nil
			}
			results = append(results, "git commit -m \""+msg+"\"")
			_ = commitResult // use if needed
		}

		// Push if instruction contains "push"
		if strings.Contains(strings.ToLower(instruction), "push") {
			_, err := gitAdapter.Push()
			if err != nil {
				// If push fails due to non-fast-forward, try pull --rebase first
				if strings.Contains(err.Error(), "non-fast-forward") || strings.Contains(err.Error(), "rejected") {
					log.Println("Push rejected, trying pull --rebase first...")
					if pullErr := gitAdapter.PullRebase(); pullErr != nil {
						return mcp.NewToolResultError("git push failed (also pull --rebase failed): " + err.Error()), nil
					}
					results = append(results, "git pull --rebase")
					// Retry push
					_, err = gitAdapter.Push()
					if err != nil {
						return mcp.NewToolResultError("git push failed after rebase: " + err.Error()), nil
					}
				} else {
					return mcp.NewToolResultError("git push failed: " + err.Error()), nil
				}
			}
			results = append(results, "git push")
		}

		response := map[string]interface{}{
			"result":            strings.Join(msgs, "; "),
			"summary":           strings.Join(msgs, "; "),
			"intent":            instruction,
			"executed_commands": results,
			"files":             files,
			"commit_count":      len(msgs),
		}

		jsonResp, _ := json.Marshal(response)
		return mcp.NewToolResultText(string(jsonResp)), nil
	}
}

// classifyOperation classifies git operations into types
// read: read-only, no Ollama needed
// direct: direct git commands, no Ollama needed
// commit: needs Ollama for message and grouping
type opType int

const (
	opRead opType = iota
	opDirect
	opCommit
)

// classifyOperation returns the type of operation and action
func classifyOperation(instruction string) (opType, string) {
	lower := strings.ToLower(instruction)

	// Read-only operations
	readOnly := map[string]string{
		"status":   "status",
		"log":      "log",
		"diff":     "diff",
		"branch":   "branches",
		"branches": "branches",
		"remote":   "remotes",
	}

	for keyword, op := range readOnly {
		if strings.Contains(lower, keyword) && !strings.Contains(lower, "commit") {
			return opRead, op
		}
	}

	// Direct operations (no Ollama needed)
	if strings.HasPrefix(lower, "pull") || strings.Contains(lower, "pull from") {
		return opDirect, "pull"
	}
	if strings.Contains(lower, "create branch") || strings.Contains(lower, "new branch") {
		return opDirect, "create-branch"
	}
	if strings.Contains(lower, "checkout") || strings.Contains(lower, "switch to") {
		return opDirect, "checkout"
	}
	if strings.Contains(lower, "stash") {
		return opDirect, "stash"
	}
	if strings.Contains(lower, "reset") {
		return opDirect, "reset"
	}
	if strings.HasPrefix(lower, "push") || strings.Contains(lower, "push to") {
		return opDirect, "push"
	}

	// Commit operations need Ollama (check BEFORE direct push to handle "commit and push")
	if strings.Contains(lower, "commit") {
		return opCommit, "commit"
	}

	// Direct push AFTER commit check (so "commit and push" goes to commit flow)
	if strings.HasPrefix(lower, "push") || strings.Contains(lower, "push to") {
		return opDirect, "push"
	}

	// Default to commit for any write operation
	return opCommit, "commit"
}

// getGitFiles returns the actual list of changed files from git status
func getGitFiles(gitAdapter *git.ExecAdapter) ([]string, error) {
	status, err := gitAdapter.Status()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, f := range status.Files {
		files = append(files, f.Path)
	}
	return files, nil
}

// validatePaths checks if all paths exist in the actual file list
// Returns (validPaths, invalidPaths, allValid)
func validatePaths(requested []string, actual []string) ([]string, []string, bool) {
	actualMap := make(map[string]bool)
	for _, f := range actual {
		actualMap[f] = true
	}

	var valid, invalid []string
	allValid := true
	for _, r := range requested {
		if actualMap[r] {
			valid = append(valid, r)
		} else {
			invalid = append(invalid, r)
			allValid = false
		}
	}
	return valid, invalid, allValid
}

// handleDirectOp executes git operations directly without Ollama
func handleDirectOp(gitAdapter *git.ExecAdapter, action, instruction string) (*mcp.CallToolResult, error) {
	var result string
	var err error

	switch action {
	case "push":
		result, err = gitAdapter.Push()
	case "pull":
		result, err = gitAdapter.Pull()
	case "create-branch":
		// Extract branch name from instruction
		branch := extractBranchName(instruction)
		if branch == "" {
			return mcp.NewToolResultError("Could not determine branch name"), nil
		}
		result, err = gitAdapter.Branch(branch)
	case "checkout":
		// Extract branch name from instruction
		branch := extractBranchName(instruction)
		if branch == "" {
			return mcp.NewToolResultError("Could not determine branch name"), nil
		}
		result, err = gitAdapter.Checkout(branch)
	case "stash":
		result, err = gitAdapter.Stash()
	case "reset":
		// Extract commit/branch to reset to
		target := extractResetTarget(instruction)
		result, err = gitAdapter.Reset("--hard", target)
	default:
		return mcp.NewToolResultError("Unknown operation: " + action), nil
	}

	if err != nil {
		return mcp.NewToolResultError(action + " failed: " + err.Error()), nil
	}

	resp, _ := json.Marshal(map[string]string{
		"operation": action,
		"result":    result,
		"type":      "direct",
	})
	return mcp.NewToolResultText(string(resp)), nil
}

// extractBranchName extracts branch name from instruction
func extractBranchName(instruction string) string {
	lower := strings.ToLower(instruction)
	// Remove common prefixes
	lower = strings.ReplaceAll(lower, "create branch ", "")
	lower = strings.ReplaceAll(lower, "new branch ", "")
	lower = strings.ReplaceAll(lower, "checkout to ", "")
	lower = strings.ReplaceAll(lower, "checkout ", "")
	lower = strings.ReplaceAll(lower, "switch to ", "")
	lower = strings.TrimSpace(lower)
	// Remove quotes if present
	lower = strings.Trim(lower, "\"")
	return lower
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

// readOnlyOps maps keywords to operation types
var readOnlyOps = map[string]string{
	"status":    "status",
	"estado":    "status",
	"log":       "log",
	"history":   "log",
	"historial": "log",
	"commits":   "log",
	"diff":      "diff",
	"cambios":   "diff",
	"branch":    "branches",
	"branches":  "branches",
	"ramas":     "branches",
	"remote":    "remotes",
	"remotes":   "remotes",
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

	return truncateContext(strings.Join(parts, "\n\n"))
}

// truncateContext limits context to maxContextSize characters
const maxContextSize = 16 * 1024

func truncateContext(ctx string) string {
	if len(ctx) <= maxContextSize {
		return ctx
	}
	return ctx[:maxContextSize] + "\n... (truncated)"
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
		// Auto-stage files if commit exists but no explicit add command
		hasAddCommand := false
		hasCommitCommand := false
		for _, cmd := range commit.Commands {
			if strings.HasPrefix(cmd, "git add") {
				hasAddCommand = true
			}
			if strings.HasPrefix(cmd, "git commit") {
				hasCommitCommand = true
			}
		}

		// If there's a commit but no add, auto-add all files
		if hasCommitCommand && !hasAddCommand {
			if err := gitAdapter.Add([]string{"-A"}); err != nil {
				return "", fmt.Errorf("auto git add failed: %w", err)
			}
		}

		for _, cmd := range commit.Commands {
			var err error
			var output string

			switch {
			case strings.HasPrefix(cmd, "git add "):
				files := strings.TrimPrefix(cmd, "git add ")
				err = gitAdapter.Add(strings.Fields(files))
				// If add fails (wrong paths), fallback to git add -A
				if err != nil {
					fmt.Fprintf(os.Stderr, "WARN: git add %s failed, falling back to -A\n", files)
					gitAdapter.Add([]string{"-A"})
				}
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
