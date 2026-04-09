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
	previewConfig      config.PreviewConfig

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

	// Confirmation port — shared by ALL operations configured in preview.operations
	confirm ports.ConfirmPort
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
	confirmAdapter := gitadapter.NewConfirmAdapter(cfg.Git.WorkDir)

	srv := &Server{
		mcpServer:        nil,
		validationConfig: cfg.Validation,
		previewConfig:    cfg.Preview,
		branch:           branchSvc,
		commit:           commitSvc,
		remote:           remoteSvc,
		operations:       opsSvc,
		query:            querySvc,
		llm:              llm,
		gitRead:          gitReadAdapter,
		gitWrite:         gitWriteAdapter,
		gitWriteReview:   gitWriteReviewAdapter,
		confirm:          confirmAdapter,
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

	// git_write_review - All confirmable git operations with a unified three-phase flow.
	// Phase: <OP>_START → <OP>_APPLY or <OP>_ABORT
	// Whether START blocks for confirmation is controlled per-operation by preview.operations config.
	// Utility commands (STATUS, SUMMARY) have no phase suffix.
	s.AddTool(
		mcp.NewTool("git_write_review",
			mcp.WithDescription("All confirmable git operations. Use <OP>_START to initiate, <OP>_APPLY to confirm, <OP>_ABORT to cancel. Operations: COMMIT, BRANCH_CREATE, BRANCH_DELETE, BRANCH_RENAME, TAG_CREATE, TAG_DELETE, MERGE, REBASE, REBASE_CONTINUE, REBASE_ABORT, RESET_SOFT, RESET_HARD, CLEAN, REMOTE_ADD, REMOTE_REMOVE, CHERRY_PICK, REVERT, INIT, CLONE. Utility: STATUS, SUMMARY."),
			mcp.WithString("command",
				mcp.Description("Operation + phase: e.g. COMMIT_START, BRANCH_CREATE_APPLY, RESET_HARD_ABORT, STATUS, SUMMARY"),
				mcp.Required(),
			),
			mcp.WithString("instruction",
				mcp.Description("Natural language instruction for COMMIT_START"),
			),
			mcp.WithString("subcommand",
				mcp.Description("Additional arg: commit count for RESET_SOFT, target for RESET_HARD, url for CLONE, name|url for REMOTE_ADD"),
			),
			mcp.WithString("branch",
				mcp.Description("Branch name for BRANCH_CREATE, BRANCH_DELETE, BRANCH_RENAME, MERGE, REBASE"),
			),
			mcp.WithString("tag",
				mcp.Description("Tag name for TAG_CREATE, TAG_DELETE"),
			),
			mcp.WithString("commit",
				mcp.Description("Commit hash for CHERRY_PICK, REVERT"),
			),
		),
		srv.handleGitWriteReview,
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

// handleGitWriteReview handles write operations with configurable preview/confirmation.
// Each command follows the three-phase flow: <OP>_START → <OP>_APPLY / <OP>_ABORT.
// Whether _START requires confirmation is controlled by preview.operations config.
func (srv *Server) handleGitWriteReview(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	command := request.GetString("command", "")
	if command == "" {
		return mcp.NewToolResultError("command is required"), nil
	}

	// Utility commands (no phase suffix)
	switch command {
	case git_write_review.STATUS:
		plan, err := srv.confirm.ReadPlan()
		if err != nil {
			return mcp.NewToolResultError("failed to read plan: " + err.Error()), nil
		}
		if plan == nil {
			return mcp.NewToolResultText(`{"status":"idle","message":"No active plan."}`), nil
		}
		resp := map[string]interface{}{
			"status":    "pending_approval",
			"operation": plan.Operation,
			"preview":   plan.Preview,
			"created":   time.Unix(plan.CreatedAt, 0).Format(time.RFC3339),
			"expired":   srv.confirm.IsPlanExpired(),
		}
		respBytes, _ := json.Marshal(resp)
		return mcp.NewToolResultText(string(respBytes)), nil

	case git_write_review.SUMMARY:
		status, err := srv.query.Status()
		if err != nil {
			return mcp.NewToolResultError("failed to get status: " + err.Error()), nil
		}
		return mcp.NewToolResultText(status), nil
	}

	op, phase := parseReviewPhase(command)

	switch phase {
	case "START":
		configKey := git_write_review.ConfigKey[op]
		needsConfirm := configKey != "" && srv.previewConfig.IsRequired(configKey)

		// COMMIT is special: needs LLM to prepare messages before storing the plan
		if op == git_write_review.COMMIT {
			return srv.handleCommitStart(request, needsConfirm)
		}

		instruction := request.GetString("instruction", "")

		// Ask Ollama to interpret the natural language instruction into concrete git args
		gitContext, err := srv.buildGitContext(op)
		if err != nil {
			log.Printf("[WARN] failed to build git context for %s: %v", op, err)
		}
		args, err := srv.llm.InterpretGitOp(op, instruction, gitContext)
		if err != nil {
			return mcp.NewToolResultError("failed to interpret instruction: " + err.Error()), nil
		}

		previewMsg := buildReviewPreviewMsg(op, args)

		if !needsConfirm {
			return srv.executeReviewOp(op, args)
		}

		plan := ports.OperationPlan{
			Operation: op,
			Args:      args,
			Preview:   previewMsg,
		}
		if err := srv.confirm.WritePlan(plan); err != nil {
			return mcp.NewToolResultError("failed to save plan: " + err.Error()), nil
		}
		if err := srv.confirm.CreateBlocker(); err != nil {
			return mcp.NewToolResultError("failed to create blocker: " + err.Error()), nil
		}

		resp := map[string]interface{}{
			"status":    "pending_approval",
			"operation": op,
			"preview":   previewMsg,
			"args":      args,
		}
		respBytes, _ := json.Marshal(resp)
		return mcp.NewToolResultText(string(respBytes)), nil

	case "APPLY":
		if !srv.confirm.HasBlocker() {
			return mcp.NewToolResultError("No active plan. Run " + op + "_START first."), nil
		}
		plan, err := srv.confirm.ReadPlan()
		if err != nil || plan == nil {
			srv.confirm.RemoveBlocker()
			return mcp.NewToolResultError("Failed to read plan. Run " + op + "_START again."), nil
		}

		// COMMIT apply re-uses the LLM-generated messages stored in the plan
		if plan.Operation == git_write_review.COMMIT {
			return srv.handleCommitApply(request, plan)
		}

		result, resultErr := srv.executeReviewOp(plan.Operation, plan.Args)
		srv.confirm.DeletePlan()
		return result, resultErr

	case "ABORT":
		srv.confirm.DeletePlan()
		srv.llm.ClearRetryContext()
		resp := map[string]interface{}{
			"status":  "aborted",
			"message": "Operation cancelled",
		}
		respBytes, _ := json.Marshal(resp)
		return mcp.NewToolResultText(string(respBytes)), nil

	default:
		return mcp.NewToolResultError("Unknown command: " + command + ". Use _START, _APPLY or _ABORT suffix. Example: BRANCH_CREATE_START"), nil
	}
}

// buildGitContext queries git state needed for LLM interpretation of the given operation.
// Only fetches what each operation actually needs to keep it fast.
func (srv *Server) buildGitContext(op string) (map[string]string, error) {
	ctx := map[string]string{
		"current_branch": "",
		"branches":       "",
		"recent_commits": "",
		"tags":           "",
	}

	switch op {
	case git_write_review.BRANCH_CREATE, git_write_review.BRANCH_DELETE,
		git_write_review.BRANCH_RENAME, git_write_review.MERGE, git_write_review.REBASE:
		if branch, err := srv.gitRead.CurrentBranch(); err == nil {
			ctx["current_branch"] = branch
		}
		if branches, err := srv.gitRead.ListBranches(); err == nil {
			ctx["branches"] = branches
		}
	case git_write_review.RESET_HARD, git_write_review.RESET_SOFT,
		git_write_review.CHERRY_PICK, git_write_review.REVERT:
		if log, err := srv.gitRead.Log(20); err == nil {
			ctx["recent_commits"] = log
		}
	case git_write_review.TAG_CREATE, git_write_review.TAG_DELETE:
		if tags, err := srv.gitRead.ListTags(); err == nil {
			ctx["tags"] = tags
		}
		if log, err := srv.gitRead.Log(5); err == nil {
			ctx["recent_commits"] = log
		}
	}

	return ctx, nil
}

// handleCommitStart prepares a commit via LLM and either executes it directly
// or stores the plan+blocker for user confirmation, depending on config.
func (srv *Server) handleCommitStart(request mcp.CallToolRequest, needsConfirm bool) (*mcp.CallToolResult, error) {
	instruction := request.GetString("instruction", "")

	if !needsConfirm {
		result, err := srv.commit.Execute(instruction, false)
		if err != nil {
			return mcp.NewToolResultError("commit failed: " + err.Error()), nil
		}
		return mcp.NewToolResultText(result), nil
	}

	// Check if this is a retry (blocker already exists from previous COMMIT_START)
	var rejectedMessage string
	if srv.confirm.HasBlocker() {
		if existing, _ := srv.confirm.ReadPlan(); existing != nil {
			rejectedMessage = existing.RejectedMessage
			if len(existing.Messages) > 0 {
				rejectedMessage = strings.Join(existing.Messages, "\n")
			}
		}
	}

	srv.llm.SetRetryContext(rejectedMessage)

	messages, chunks, warnings, err := srv.commit.PrepareCommit(instruction)
	if err != nil {
		return mcp.NewToolResultError("failed to prepare commit: " + err.Error()), nil
	}

	var files []string
	for _, chunk := range chunks {
		files = append(files, chunk.Files...)
	}

	plan := ports.OperationPlan{
		Operation:       git_write_review.COMMIT,
		Preview:         fmt.Sprintf("Commit %d change(s) across %d file(s)", len(messages), len(files)),
		Messages:        messages,
		Files:           files,
		RejectedMessage: rejectedMessage,
	}
	if err := srv.confirm.WritePlan(plan); err != nil {
		return mcp.NewToolResultError("failed to save plan: " + err.Error()), nil
	}
	if err := srv.confirm.CreateBlocker(); err != nil {
		return mcp.NewToolResultError("failed to create blocker: " + err.Error()), nil
	}

	resp := map[string]interface{}{
		"status":           "pending_approval",
		"operation":        "COMMIT",
		"message":          strings.Join(messages, "\n"),
		"files":            files,
		"rejected_message": rejectedMessage,
		"num_commits":      len(messages),
		"warnings":         warnings,
	}
	respBytes, _ := json.Marshal(resp)
	return mcp.NewToolResultText(string(respBytes)), nil
}

// handleCommitApply executes a commit using the plan stored by handleCommitStart.
func (srv *Server) handleCommitApply(request mcp.CallToolRequest, plan *ports.OperationPlan) (*mcp.CallToolResult, error) {
	instruction := request.GetString("instruction", "")

	defer func() {
		srv.confirm.DeletePlan()
		srv.llm.ClearRetryContext()
	}()

	if instruction != "" {
		result, err := srv.commit.Execute(instruction, false)
		if err != nil {
			srv.confirm.RemoveBlocker()
			return mcp.NewToolResultError("commit failed: " + err.Error()), nil
		}
		return mcp.NewToolResultText(result), nil
	}

	// Re-prepare to get chunks (not serializable to JSON)
	messages, chunks, _, err := srv.commit.PrepareCommit("")
	if err != nil {
		srv.confirm.RemoveBlocker()
		return mcp.NewToolResultError("failed to prepare: " + err.Error()), nil
	}

	result, err := srv.commit.ExecutePrepared(messages, chunks, "")
	if err != nil {
		srv.confirm.RemoveBlocker()
		return mcp.NewToolResultError("commit failed: " + err.Error()), nil
	}
	return mcp.NewToolResultText(result), nil
}

// parseReviewPhase splits "BRANCH_CREATE_START" into op="BRANCH_CREATE", phase="START".
func parseReviewPhase(command string) (op, phase string) {
	for _, suffix := range []string{"_START", "_APPLY", "_ABORT"} {
		if strings.HasSuffix(command, suffix) {
			return strings.TrimSuffix(command, suffix), suffix[1:]
		}
	}
	return command, ""
}

// extractReviewArgs pulls the relevant params from request based on the operation.
func extractReviewArgs(request mcp.CallToolRequest, op string) map[string]string {
	args := map[string]string{}
	sub := request.GetString("subcommand", "")
	branch := request.GetString("branch", "")
	tag := request.GetString("tag", "")
	commit := request.GetString("commit", "")

	switch op {
	case git_write_review.BRANCH_CREATE, git_write_review.BRANCH_DELETE,
		git_write_review.MERGE, git_write_review.REBASE:
		if branch != "" {
			args["branch"] = branch
		} else if sub != "" {
			args["branch"] = sub
		}
	case git_write_review.BRANCH_RENAME:
		if branch != "" {
			args["branch"] = branch
		}
		if sub != "" {
			args["new_branch"] = sub
		}
	case git_write_review.TAG_CREATE, git_write_review.TAG_DELETE:
		if tag != "" {
			args["tag"] = tag
		}
	case git_write_review.CHERRY_PICK, git_write_review.REVERT:
		if commit != "" {
			args["commit"] = commit
		}
	case git_write_review.RESET_HARD, git_write_review.RESET_SOFT,
		git_write_review.REMOTE_REMOVE, git_write_review.CLONE:
		if sub != "" {
			args["subcommand"] = sub
		}
	case git_write_review.REMOTE_ADD:
		if sub != "" {
			args["subcommand"] = sub
		}
	}
	return args
}

// executeReviewOp runs the git operation identified by op using the given args.
func (srv *Server) executeReviewOp(op string, args map[string]string) (*mcp.CallToolResult, error) {
	var result string
	var err error

	switch op {
	case git_write_review.BRANCH_CREATE:
		result, err = srv.gitWriteReview.CreateBranch(args["branch"])
	case git_write_review.BRANCH_DELETE:
		result, err = srv.gitWriteReview.DeleteBranch(args["branch"])
	case git_write_review.BRANCH_RENAME:
		result, err = srv.gitWriteReview.RenameBranch(args["branch"], args["new_branch"])
	case git_write_review.TAG_CREATE:
		result, err = srv.gitWriteReview.CreateTag(args["tag"])
	case git_write_review.TAG_DELETE:
		result, err = srv.gitWriteReview.DeleteTag(args["tag"])
	case git_write_review.MERGE:
		result, err = srv.gitWriteReview.Merge(args["branch"])
	case git_write_review.REBASE:
		result, err = srv.gitWriteReview.Rebase(args["branch"])
	case git_write_review.REBASE_CONTINUE:
		result, err = srv.gitWriteReview.RebaseContinue()
	case git_write_review.REBASE_ABORT:
		result, err = srv.gitWriteReview.RebaseAbort()
	case git_write_review.RESET_SOFT:
		commits := 1
		if sub := args["subcommand"]; sub != "" {
			fmt.Sscanf(sub, "%d", &commits)
		}
		err = srv.gitWriteReview.ResetSoft(commits)
		result = "Soft reset performed"
	case git_write_review.RESET_HARD:
		result, err = srv.gitWriteReview.ResetHard(args["subcommand"])
	case git_write_review.CLEAN:
		result, err = srv.gitWriteReview.Clean()
	case git_write_review.REMOTE_ADD:
		parts := strings.Split(args["subcommand"], "|")
		if len(parts) != 2 {
			return mcp.NewToolResultError("REMOTE_ADD requires subcommand in name|url format"), nil
		}
		result, err = srv.gitWriteReview.AddRemote(parts[0], parts[1])
	case git_write_review.REMOTE_REMOVE:
		result, err = srv.gitWriteReview.RemoveRemote(args["subcommand"])
	case git_write_review.CHERRY_PICK:
		result, err = srv.gitWriteReview.CherryPick(args["commit"])
	case git_write_review.REVERT:
		result, err = srv.gitWriteReview.Revert(args["commit"])
	case git_write_review.INIT:
		result, err = srv.gitWriteReview.Init()
	case git_write_review.CLONE:
		result, err = srv.gitWriteReview.Clone(args["subcommand"])
	default:
		return mcp.NewToolResultError("Unknown operation: " + op), nil
	}

	if err != nil {
		return mcp.NewToolResultError(op + " failed: " + err.Error()), nil
	}
	return mcp.NewToolResultText(result), nil
}

// buildReviewPreviewMsg constructs a human-readable description of what will be executed.
func buildReviewPreviewMsg(op string, args map[string]string) string {
	var description string
	switch op {
	case git_write_review.BRANCH_CREATE:
		description = fmt.Sprintf("Create branch: %s", args["branch"])
	case git_write_review.BRANCH_DELETE:
		description = fmt.Sprintf("Delete branch: %s", args["branch"])
	case git_write_review.BRANCH_RENAME:
		description = fmt.Sprintf("Rename branch %s → %s", args["branch"], args["new_branch"])
	case git_write_review.TAG_CREATE:
		description = fmt.Sprintf("Create tag: %s", args["tag"])
	case git_write_review.TAG_DELETE:
		description = fmt.Sprintf("Delete tag: %s", args["tag"])
	case git_write_review.MERGE:
		description = fmt.Sprintf("Merge branch: %s", args["branch"])
	case git_write_review.REBASE:
		description = fmt.Sprintf("Rebase onto: %s", args["branch"])
	case git_write_review.REBASE_CONTINUE:
		description = "Continue rebase after resolving conflicts"
	case git_write_review.REBASE_ABORT:
		description = "Abort current rebase"
	case git_write_review.RESET_SOFT:
		description = fmt.Sprintf("Soft reset: %s commit(s)", args["subcommand"])
	case git_write_review.RESET_HARD:
		description = fmt.Sprintf("Hard reset to: %s", args["subcommand"])
	case git_write_review.CLEAN:
		description = "Remove all untracked files"
	case git_write_review.REMOTE_ADD:
		parts := strings.Split(args["subcommand"], "|")
		if len(parts) == 2 {
			description = fmt.Sprintf("Add remote: %s → %s", parts[0], parts[1])
		}
	case git_write_review.REMOTE_REMOVE:
		description = fmt.Sprintf("Remove remote: %s", args["subcommand"])
	case git_write_review.CHERRY_PICK:
		description = fmt.Sprintf("Cherry-pick commit: %s", args["commit"])
	case git_write_review.REVERT:
		description = fmt.Sprintf("Revert commit: %s", args["commit"])
	case git_write_review.INIT:
		description = "Initialize new git repository"
	case git_write_review.CLONE:
		description = fmt.Sprintf("Clone repository: %s", args["subcommand"])
	default:
		description = op
	}
	return fmt.Sprintf("Ready to execute:\n  %s", description)
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
