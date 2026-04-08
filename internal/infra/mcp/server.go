package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	branchusecase "github.com/Alejandro-M-P/git-courer/internal/app/branch"
	commitusecase "github.com/Alejandro-M-P/git-courer/internal/app/commit"
	git_read "github.com/Alejandro-M-P/git-courer/internal/app/git_read"
	git_write "github.com/Alejandro-M-P/git-courer/internal/app/git_write"
	git_write_commit "github.com/Alejandro-M-P/git-courer/internal/app/git_write_commit"
	git_write_review "github.com/Alejandro-M-P/git-courer/internal/app/git_write_review"
	operationsusecase "github.com/Alejandro-M-P/git-courer/internal/app/operations"
	queryusecase "github.com/Alejandro-M-P/git-courer/internal/app/query"
	remoteusecase "github.com/Alejandro-M-P/git-courer/internal/app/remote"
	securitysvc "github.com/Alejandro-M-P/git-courer/internal/app/security"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/infra/config"
	"github.com/Alejandro-M-P/git-courer/internal/infra/diff"
	gitadapter "github.com/Alejandro-M-P/git-courer/internal/infra/git"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server holds the MCP server and its dependencies.
// It depends on ports and usecases only — never on concrete adapters.
type Server struct {
	mcpServer          *server.MCPServer
	clientInfo         *domain.ClientInfo
	clientCapabilities *domain.ClientCapabilities
	mu                 sync.Mutex
	validationConfig   config.ValidationConfig

	// Usecases (injected via DI)
	branch     *branchusecase.Service
	commit     *commitusecase.Service
	remote     *remoteusecase.Service
	operations *operationsusecase.Service
	query      *queryusecase.Service

	// LLM port (for direct access when needed)
	llm ports.LLM

	// Git port adapters for deterministic tools
	gitRead        ports.GitReadPort
	gitWrite       ports.GitWritePort
	gitWriteReview ports.GitWriteReviewPort
	gitWriteCommit ports.GitWriteCommitPort
}

// SetClientInfo stores client information from initialize handshake
func (srv *Server) SetClientInfo(info *domain.ClientInfo, caps *domain.ClientCapabilities) {
	srv.clientInfo = info
	srv.clientCapabilities = caps
	log.Printf("Client registered: %s v%s (sampling: %v, elicitation: %v)",
		info.Name, info.Version, caps.Sampling, caps.Elicitation)
}

// GetClientInfo returns the stored client information
func (srv *Server) GetClientInfo() *domain.ClientInfo {
	return srv.clientInfo
}

// OllamaLifecycle abstracts Ollama runtime operations (start, stop, pre-warm).
// This keeps the MCP server decoupled from the concrete adapter.
type OllamaLifecycle interface {
	EnsureOllama() (bool, error)
	PreWarm() error
	Stop()
}

// NewServer creates a new MCP server for git-courer.
// All dependencies are injected — the server knows nothing about concrete adapters.
func NewServer(cfg *config.Config, git ports.Git, llm ports.LLM, ollamaLifecycle OllamaLifecycle) *Server {
	// Create usecases
	branchSvc := branchusecase.NewService(git)
	chunker := diff.NewChunker()
	securitySvc := securitysvc.NewSecurityService(cfg)
	commitSvc := commitusecase.NewService(git, llm, chunker, securitySvc)
	remoteSvc := remoteusecase.NewService(git)
	opsSvc := operationsusecase.NewService(git)
	querySvc := queryusecase.NewService(git)

	// Create git port adapters for deterministic tools
	gitReadAdapter := gitadapter.NewGitReadAdapter(cfg.Git.WorkDir)
	gitWriteAdapter := gitadapter.NewGitWriteAdapter(cfg.Git.WorkDir)
	gitWriteReviewAdapter := gitadapter.NewGitWriteReviewAdapter(cfg.Git.WorkDir)
	gitWriteCommitAdapter := gitadapter.NewGitWriteCommitAdapter(cfg.Git.WorkDir)

	srv := &Server{
		mcpServer:        nil,
		validationConfig: cfg.Validation,
		branch:           branchSvc,
		commit:           commitSvc,
		remote:           remoteSvc,
		operations:       opsSvc,
		query:            querySvc,
		llm:              llm,
		gitRead:          gitReadAdapter,
		gitWrite:         gitWriteAdapter,
		gitWriteReview:   gitWriteReviewAdapter,
		gitWriteCommit:   gitWriteCommitAdapter,
	}

	// Callback to store client info (must be set BEFORE creating hooks)
	clientCallback := func(info *domain.ClientInfo, caps *domain.ClientCapabilities) {
		srv.SetClientInfo(info, caps)
	}

	// Create hooks to capture client info during initialize
	hooks := &server.Hooks{}
	hooks.AddAfterInitialize(func(ctx context.Context, id any, req *mcp.InitializeRequest, res *mcp.InitializeResult) {
		log.Printf("🔍 Initialize received: client=%s v%s", req.Params.ClientInfo.Name, req.Params.ClientInfo.Version)

		clientInfo := &domain.ClientInfo{
			Name:    req.Params.ClientInfo.Name,
			Version: req.Params.ClientInfo.Version,
		}
		caps := &domain.ClientCapabilities{
			Sampling:    req.Params.Capabilities.Sampling != nil,
			Elicitation: req.Params.Capabilities.Elicitation != nil,
		}
		log.Printf("🔍 Capabilities: sampling=%v, elicitation=%v", caps.Sampling, caps.Elicitation)

		clientCallback(clientInfo, caps)
	})

	s := server.NewMCPServer(
		cfg.MCP.Name,
		cfg.MCP.Version,
		server.WithToolCapabilities(true),
		server.WithRecovery(),
		server.WithHooks(hooks),
	)

	srv.mcpServer = s

	// Register tools
	registerTools(s, srv)

	// Start Ollama and pre-warm model synchronously at startup
	log.Println("Starting Ollama...")
	started, err := ollamaLifecycle.EnsureOllama()
	if err != nil {
		log.Printf("⚠ Warning: Ollama not available: %v", err)
		log.Printf("  Write operations will retry starting Ollama automatically")
	} else {
		if started {
			log.Println("✓ Ollama started by git-courer")
		}
		if err := ollamaLifecycle.PreWarm(); err != nil {
			log.Printf("⚠ Failed to pre-warm model: %v", err)
		} else {
			log.Printf("✓ Model ready for instant commits")
		}
	}

	return srv
}

// Stop stops Ollama if we started it
func (srv *Server) Stop(ollamaLifecycle OllamaLifecycle) {
	if ollamaLifecycle != nil {
		ollamaLifecycle.Stop()
	}
}

// Serve starts the MCP server
func (srv *Server) Serve() {
	log.Printf("Starting git-courer MCP server")
	if err := server.ServeStdio(srv.mcpServer); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func registerTools(s *server.MCPServer, srv *Server) {
	// git_read - Direct read-only operations without Ollama
	// Routes directly based on subcommand: READ_STATUS, READ_DIFF, READ_DIFF_UNSTAGED, READ_LOG, READ_BRANCHES
	s.AddTool(
		mcp.NewTool("git_read",
			mcp.WithDescription("Read-only git operations that route DIRECTLY without Ollama. Subcommands: READ_STATUS | READ_DIFF | READ_DIFF_UNSTAGED | READ_LOG | READ_BRANCHES"),
			mcp.WithString("command",
				mcp.Description("Subcommand: READ_STATUS | READ_DIFF | READ_DIFF_UNSTAGED | READ_LOG | READ_BRANCHES"),
				mcp.Required(),
			),
		),
		srv.handleGitRead,
	)

	// git_write - Direct write operations without preview
	// Routes based on subcommand: ADD, CHECKOUT, SWITCH, STASH, STASH_POP, PUSH, PULL, FETCH, RM
	s.AddTool(
		mcp.NewTool("git_write",
			mcp.WithDescription("Direct write git operations without preview. Subcommands: ADD, CHECKOUT, SWITCH, STASH, STASH_POP, PUSH, PULL, FETCH, RM"),
			mcp.WithString("command",
				mcp.Description("Subcommand: ADD | CHECKOUT | SWITCH | STASH | STASH_POP | PUSH | PULL | FETCH | RM"),
				mcp.Required(),
			),
			mcp.WithString("subcommand",
				mcp.Description("Path, branch name, or additional arg depending on command"),
			),
		),
		srv.handleGitWrite,
	)

	// git_write_review - Write operations that require user confirmation
	// Routes based on subcommand: BRANCH_CREATE, BRANCH_DELETE, MERGE, REBASE, etc.
	s.AddTool(
		mcp.NewTool("git_write_review",
			mcp.WithDescription("Write git operations requiring confirmation. Subcommands: BRANCH_CREATE, BRANCH_DELETE, BRANCH_RENAME, TAG_CREATE, TAG_DELETE, MERGE, REBASE, REBASE_CONTINUE, REBASE_ABORT, RESET_SOFT, RESET_HARD, CLEAN, REMOTE_ADD, REMOTE_REMOVE, CHERRY_PICK, REVERT, INIT, CLONE"),
			mcp.WithString("command",
				mcp.Description("Subcommand"),
				mcp.Required(),
			),
			mcp.WithString("subcommand",
				mcp.Description("Additional argument (e.g., branch name, commit hash)"),
			),
			mcp.WithString("branch",
				mcp.Description("Branch name for branch operations"),
			),
			mcp.WithString("tag",
				mcp.Description("Tag name for tag operations"),
			),
			mcp.WithString("commit",
				mcp.Description("Commit hash for cherry-pick, revert"),
			),
		),
		srv.handleGitWriteReview,
	)

	// git_write_commit - Commit operations with preview mode
	// Routes based on subcommand: COMMIT_START, COMMIT_STATUS, COMMIT_SUMMARY, COMMIT_APPLY, COMMIT_ABORT
	// Flow controlled by config.Validation.RequireConfirmation (not by preview parameter)
	s.AddTool(
		mcp.NewTool("git_write_commit",
			mcp.WithDescription(fmt.Sprintf("Commit operations. Confirmation required: %v. Subcommands: COMMIT_START, COMMIT_STATUS, COMMIT_SUMMARY, COMMIT_APPLY, COMMIT_ABORT", srv.validationConfig.RequireConfirmation)),
			mcp.WithString("command",
				mcp.Description("Subcommand: COMMIT_START | COMMIT_STATUS | COMMIT_SUMMARY | COMMIT_APPLY | COMMIT_ABORT"),
				mcp.Required(),
			),
			mcp.WithString("instruction",
				mcp.Description("Commit instruction for COMMIT_APPLY"),
			),
			mcp.WithBoolean("preview",
				mcp.Description(fmt.Sprintf("If true, returns preview without executing (default: %v)", srv.validationConfig.RequireConfirmation)),
			),
		),
		srv.handleGitWriteCommit,
	)
}

// handleGitRead handles direct read-only git operations without Ollama
// Routes based solely on subcommand - no intent classification, no blocking
func (srv *Server) handleGitRead(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	command := request.GetString("command", "")
	if command == "" {
		return mcp.NewToolResultError("command is required (READ_STATUS | READ_DIFF | READ_DIFF_UNSTAGED | READ_LOG | READ_BRANCHES)"), nil
	}

	var result string
	var err error

	switch command {
	case git_read.READ_STATUS:
		result, err = srv.query.Status()
	case git_read.READ_DIFF, git_read.READ_DIFF_UNSTAGED:
		// Both map to Diff() since Diff() returns unstaged changes
		result, err = srv.query.Diff()
	case git_read.READ_LOG:
		result, err = srv.query.Log(20)
	case git_read.READ_BRANCHES:
		result, err = srv.query.CurrentBranch()
	default:
		return mcp.NewToolResultError("Unknown command: " + command + ". Valid: READ_STATUS | READ_DIFF | READ_DIFF_UNSTAGED | READ_LOG | READ_BRANCHES"), nil
	}

	if err != nil {
		return mcp.NewToolResultError(command + " failed: " + err.Error()), nil
	}

	return mcp.NewToolResultText(result), nil
}

// handleGitWrite handles direct write git operations without preview
// Routes based solely on subcommand
func (srv *Server) handleGitWrite(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	command := request.GetString("command", "")
	if command == "" {
		return mcp.NewToolResultError("command is required"), nil
	}

	subCommand := request.GetString("subcommand", "")
	var result string
	var err error

	switch command {
	case git_write.ADD:
		err = srv.gitWrite.Add([]string{subCommand})
		result = "Files added"
	case git_write.CHECKOUT:
		err = srv.gitWrite.Checkout(subCommand)
		result = "Checked out branch"
	case git_write.SWITCH:
		err = srv.gitWrite.Switch(subCommand)
		result = "Switched to branch"
	case git_write.STASH:
		err = srv.gitWrite.Stash()
		result = "Changes stashed"
	case git_write.STASH_POP:
		err = srv.gitWrite.StashPop()
		result = "Stashed changes restored"
	case git_write.PUSH:
		result, err = srv.gitWrite.Push()
	case git_write.PULL:
		result, err = srv.gitWrite.Pull()
	case git_write.FETCH:
		result, err = srv.gitWrite.Fetch()
	case git_write.RM:
		err = srv.gitWrite.Remove([]string{subCommand})
		result = "Files removed"
	default:
		return mcp.NewToolResultError("Unknown command: " + command), nil
	}

	if err != nil {
		return mcp.NewToolResultError(command + " failed: " + err.Error()), nil
	}

	return mcp.NewToolResultText(result), nil
}

// handleGitWriteReview handles write operations that require user confirmation
func (srv *Server) handleGitWriteReview(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	command := request.GetString("command", "")
	if command == "" {
		return mcp.NewToolResultError("command is required"), nil
	}

	subCommand := request.GetString("subcommand", "")
	branchName := request.GetString("branch", "")
	tagName := request.GetString("tag", "")
	commit := request.GetString("commit", "")

	// Build preview message to show user what will be done
	previewMsg := srv.buildReviewPreview(command, branchName, tagName, subCommand, commit)

	// Acquire lock and wait for user confirmation
	if err := srv.gitWriteCommit.AcquireLock(); err != nil {
		return mcp.NewToolResultError("Another operation is in progress. Please try again later."), nil
	}
	defer srv.gitWriteCommit.ReleaseLock()

	// Signal that we have a pending operation
	srv.gitWriteCommit.Approve()

	// Wait for user confirmation
	if !srv.gitWriteCommit.WaitForConfirmation() {
		return mcp.NewToolResultText("Operation cancelled by user"), nil
	}

	var result string
	var err error

	switch command {
	case git_write_review.BRANCH_CREATE:
		result, err = srv.gitWriteReview.CreateBranch(branchName)
	case git_write_review.BRANCH_DELETE:
		result, err = srv.gitWriteReview.DeleteBranch(branchName)
	case git_write_review.TAG_CREATE:
		result, err = srv.gitWriteReview.CreateTag(tagName)
	case git_write_review.TAG_DELETE:
		result, err = srv.gitWriteReview.DeleteTag(tagName)
	case git_write_review.MERGE:
		result, err = srv.gitWriteReview.Merge(branchName)
	case git_write_review.REBASE:
		result, err = srv.gitWriteReview.Rebase(branchName)
	case git_write_review.REBASE_CONTINUE:
		result, err = srv.gitWriteReview.RebaseContinue()
	case git_write_review.REBASE_ABORT:
		result, err = srv.gitWriteReview.RebaseAbort()
	case git_write_review.RESET_SOFT:
		commits := 1
		if subCommand != "" {
			fmt.Sscanf(subCommand, "%d", &commits)
		}
		err = srv.gitWriteReview.ResetSoft(commits)
		result = "Soft reset performed"
	case git_write_review.RESET_HARD:
		result, err = srv.gitWriteReview.ResetHard(subCommand)
	case git_write_review.CLEAN:
		result, err = srv.gitWriteReview.Clean()
	case git_write_review.REMOTE_ADD:
		parts := strings.Split(subCommand, "|")
		if len(parts) != 2 {
			return mcp.NewToolResultError("remote add requires name|url format"), nil
		}
		result, err = srv.gitWriteReview.AddRemote(parts[0], parts[1])
	case git_write_review.REMOTE_REMOVE:
		result, err = srv.gitWriteReview.RemoveRemote(subCommand)
	case git_write_review.CHERRY_PICK:
		result, err = srv.gitWriteReview.CherryPick(commit)
	case git_write_review.REVERT:
		result, err = srv.gitWriteReview.Revert(commit)
	case git_write_review.INIT:
		result, err = srv.gitWriteReview.Init()
	case git_write_review.CLONE:
		result, err = srv.gitWriteReview.Clone(subCommand)
	default:
		return mcp.NewToolResultError("Unknown command: " + command), nil
	}

	if err != nil {
		return mcp.NewToolResultError(command + " failed: " + err.Error()), nil
	}

	return mcp.NewToolResultText(previewMsg + "\n\n" + result), nil
}

// buildReviewPreview constructs a human-readable description of what will be executed
func (srv *Server) buildReviewPreview(command, branchName, tagName, subCommand, commit string) string {
	var operation string
	switch command {
	case git_write_review.BRANCH_CREATE:
		operation = fmt.Sprintf("Create branch: %s", branchName)
	case git_write_review.BRANCH_DELETE:
		operation = fmt.Sprintf("Delete branch: %s", branchName)
	case git_write_review.TAG_CREATE:
		operation = fmt.Sprintf("Create tag: %s", tagName)
	case git_write_review.TAG_DELETE:
		operation = fmt.Sprintf("Delete tag: %s", tagName)
	case git_write_review.MERGE:
		operation = fmt.Sprintf("Merge branch: %s", branchName)
	case git_write_review.REBASE:
		operation = fmt.Sprintf("Rebase onto branch: %s", branchName)
	case git_write_review.REBASE_CONTINUE:
		operation = "Continue rebase after resolving conflicts"
	case git_write_review.REBASE_ABORT:
		operation = "Abort current rebase"
	case git_write_review.RESET_SOFT:
		operation = fmt.Sprintf("Soft reset: %s commit(s)", subCommand)
	case git_write_review.RESET_HARD:
		operation = fmt.Sprintf("Hard reset to: %s", subCommand)
	case git_write_review.CLEAN:
		operation = "Remove untracked files"
	case git_write_review.REMOTE_ADD:
		parts := strings.Split(subCommand, "|")
		if len(parts) == 2 {
			operation = fmt.Sprintf("Add remote: %s -> %s", parts[0], parts[1])
		}
	case git_write_review.REMOTE_REMOVE:
		operation = fmt.Sprintf("Remove remote: %s", subCommand)
	case git_write_review.CHERRY_PICK:
		operation = fmt.Sprintf("Cherry-pick commit: %s", commit)
	case git_write_review.REVERT:
		operation = fmt.Sprintf("Revert commit: %s", commit)
	case git_write_review.INIT:
		operation = "Initialize new git repository"
	case git_write_review.CLONE:
		operation = fmt.Sprintf("Clone repository: %s", subCommand)
	default:
		operation = command
	}
	return fmt.Sprintf("Ready to execute:\n  %s\n\nWaiting for confirmation...", operation)
}

// handleGitWriteCommit handles commit operations with preview mode
func (srv *Server) handleGitWriteCommit(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	command := request.GetString("command", "")
	if command == "" {
		return mcp.NewToolResultError("command is required"), nil
	}

	// Use config require_confirmation as the source of truth
	// If client explicitly passes preview, use it; otherwise fall back to config
	requireConfirmation := srv.validationConfig.RequireConfirmation
	preview := request.GetBool("preview", requireConfirmation)

	switch command {
	case git_write_commit.COMMIT_START:
		// If preview mode, prepare commit without executing
		if preview {
			// Check if this is a retry (blocker exists from previous attempt)
			isRetry := srv.gitWriteCommit.HasBlocker()

			// Prepare commit: analyze diff, stage files, generate messages
			messages, chunks, warnings, err := srv.commit.PrepareCommit("")
			if err != nil {
				return mcp.NewToolResultError("failed to prepare commit: " + err.Error()), nil
			}

			// Get files from chunks
			var files []string
			for _, chunk := range chunks {
				files = append(files, chunk.Files...)
			}

			// Read existing plan to get rejected_message if retrying
			var rejectedMessage string
			if isRetry {
				existingPlan, _ := srv.gitWriteCommit.ReadPlan()
				if existingPlan != nil && existingPlan.Message != "" {
					rejectedMessage = existingPlan.Message
				}
			}

			// Set retry context on LLM so it generates better messages on retry
			srv.llm.SetRetryContext(rejectedMessage)

			// Create new plan with generated messages
			plan := ports.CommitPlan{
				Preview:         preview,
				CreatedAt:       time.Now().Unix(),
				Messages:        messages,
				Files:           files,
				Message:         strings.Join(messages, "\n"),
				RejectedMessage: rejectedMessage,
			}

			if err := srv.gitWriteCommit.WritePlan(plan); err != nil {
				return mcp.NewToolResultError("failed to save plan: " + err.Error()), nil
			}

			// Create blocker to prevent execution until approval
			if err := srv.gitWriteCommit.CreateBlocker(); err != nil {
				return mcp.NewToolResultError("failed to create blocker: " + err.Error()), nil
			}

			// Return JSON response for AI to display to user
			resp := map[string]interface{}{
				"status":           "pending_approval",
				"message":          plan.Message,
				"files":            files,
				"rejected_message": rejectedMessage,
				"num_commits":      len(messages),
				"warnings":         warnings,
			}
			respBytes, _ := json.Marshal(resp)
			return mcp.NewToolResultText(string(respBytes)), nil
		}

		// No preview - execute directly
		result, err := srv.commit.Execute("", preview)
		if err != nil {
			return mcp.NewToolResultError("commit failed: " + err.Error()), nil
		}
		srv.gitWriteCommit.DeletePlan()
		return mcp.NewToolResultText(result), nil

	case git_write_commit.COMMIT_STATUS:
		plan, err := srv.gitWriteCommit.ReadPlan()
		if err != nil {
			return mcp.NewToolResultError("failed to read plan: " + err.Error()), nil
		}
		if plan == nil {
			return mcp.NewToolResultText("No active commit plan."), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Plan: preview=%v, created=%s", plan.Preview, time.Unix(plan.CreatedAt, 0).Format(time.RFC3339))), nil

	case git_write_commit.COMMIT_SUMMARY:
		status, err := srv.query.Status()
		if err != nil {
			return mcp.NewToolResultError("failed to get status: " + err.Error()), nil
		}
		return mcp.NewToolResultText(status), nil

	case git_write_commit.COMMIT_APPLY:
		// Check if blocker exists (user must have approved)
		if !srv.gitWriteCommit.HasBlocker() {
			return mcp.NewToolResultError("No active commit plan. Run COMMIT_START first."), nil
		}

		// Read the plan to get prepared messages and chunks
		plan, err := srv.gitWriteCommit.ReadPlan()
		if err != nil || plan == nil {
			srv.gitWriteCommit.RemoveBlocker()
			return mcp.NewToolResultError("Failed to read plan. Run COMMIT_START again."), nil
		}

		instruction := request.GetString("instruction", "")

		// If user provided a custom message, use it for all chunks
		if instruction != "" {
			// User provided custom message - replace all messages
			for i := range plan.Messages {
				plan.Messages[i] = instruction
			}
		}

		// We need the chunks to execute. Since we can't store chunks in plan (not serializable),
		// we need to re-prepare. This is a limitation - in a real implementation we'd store chunks.
		// For now, if user passed instruction, just execute with that instruction
		if instruction != "" {
			// User gave custom message - execute direct with that message
			result, err := srv.commit.Execute(instruction, false)
			if err != nil {
				srv.gitWriteCommit.RemoveBlocker()
				return mcp.NewToolResultError("commit failed: " + err.Error()), nil
			}
			srv.gitWriteCommit.DeletePlan()
			srv.llm.ClearRetryContext()
			return mcp.NewToolResultText(result), nil
		}

		// No custom instruction - need to re-prepare to get chunks
		// This is because chunks can't be serialized to JSON
		messages, chunks, _, err := srv.commit.PrepareCommit("")
		if err != nil {
			srv.gitWriteCommit.RemoveBlocker()
			return mcp.NewToolResultError("failed to prepare: " + err.Error()), nil
		}

		// Execute with prepared messages
		result, err := srv.commit.ExecutePrepared(messages, chunks, "")
		if err != nil {
			srv.gitWriteCommit.RemoveBlocker()
			return mcp.NewToolResultError("commit failed: " + err.Error()), nil
		}

		// Clean up blocker and plan
		srv.gitWriteCommit.DeletePlan()
		srv.llm.ClearRetryContext()
		return mcp.NewToolResultText(result), nil

	case git_write_commit.COMMIT_ABORT:
		srv.gitWriteCommit.Abort()
		err := srv.gitWriteCommit.DeletePlan()
		if err != nil {
			return mcp.NewToolResultError("failed to delete plan: " + err.Error()), nil
		}
		srv.llm.ClearRetryContext()
		return mcp.NewToolResultText("Commit plan aborted."), nil

	default:
		return mcp.NewToolResultError("Unknown command: " + command), nil
	}
}

// ServeWithAdapter is a convenience factory that wires everything together.
// For full DI control, use NewServer with pre-created ports and usecases.
func ServeWithAdapter(cfg *config.Config, git ports.Git, llm ports.LLM, ollamaLifecycle OllamaLifecycle) *Server {
	srv := NewServer(cfg, git, llm, ollamaLifecycle)

	go func() {
		if err := server.ServeStdio(srv.mcpServer); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	return srv
}
