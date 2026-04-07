package mcp

import (
	"context"
	"log"
	"sync"

	branchusecase "github.com/Alejandro-M-P/git-courer/internal/app/branch"
	commitusecase "github.com/Alejandro-M-P/git-courer/internal/app/commit"
	operationsusecase "github.com/Alejandro-M-P/git-courer/internal/app/operations"
	queryusecase "github.com/Alejandro-M-P/git-courer/internal/app/query"
	remoteusecase "github.com/Alejandro-M-P/git-courer/internal/app/remote"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/infra/config"
	"github.com/Alejandro-M-P/git-courer/internal/infra/diff"
	"github.com/Alejandro-M-P/git-courer/internal/shared/classifier"
	"github.com/Alejandro-M-P/git-courer/internal/shared/parser"
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

	// Usecases (injected via DI)
	branch     *branchusecase.Service
	commit     *commitusecase.Service
	remote     *remoteusecase.Service
	operations *operationsusecase.Service
	query      *queryusecase.Service

	// LLM port (for direct access when needed)
	llm ports.LLM
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
	commitSvc := commitusecase.NewService(git, llm, chunker)
	remoteSvc := remoteusecase.NewService(git)
	opsSvc := operationsusecase.NewService(git)
	querySvc := queryusecase.NewService(git)

	srv := &Server{
		mcpServer:  nil,
		branch:     branchSvc,
		commit:     commitSvc,
		remote:     remoteSvc,
		operations: opsSvc,
		query:      querySvc,
		llm:        llm,
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
		srv.handleGitLocalTask,
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
		srv.handleGitLocalTask,
	)
}

// handleGitLocalTask - THE ONLY ENTRY POINT FOR GIT
// Route: read-only operations go direct (zero Ollama, zero tokens)
// Write operations use the 2-call Ollama flow
// ⚠️ ONE CALL PER REQUEST - uses mutex to prevent concurrent calls
func (srv *Server) handleGitLocalTask(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	// Classify using the local IntentClassifier (bilingual, tested)
	intentClassifier := classifier.NewIntentClassifier()
	intentResult := intentClassifier.Classify(instruction)

	// Route based on intent type
	var callResult *mcp.CallToolResult
	var err error

	switch intentResult.Type {
	case classifier.IntentQuery:
		// Read-only operations (status, log, diff, branches)
		callResult, err = srv.handleReadOnly(intentResult.Action)
	case classifier.IntentCreate, classifier.IntentModify:
		// Simple write operations (branch, checkout, stash, etc.)
		action := intentResult.Action
		if intentResult.Action == "branch" || intentResult.Action == "create-branch" {
			action = "create-branch"
		}
		callResult, err = srv.handleDirectOp(action, instruction)
	case classifier.IntentWrite, classifier.IntentPassthrough:
		// Complex write (commit) or unknown → use Ollama path
		if intentResult.Action == "commit" {
			callResult, err = srv.handleWrite(instruction, preview)
		} else {
			callResult, err = srv.handleDirectOp(intentResult.Action, instruction)
		}
	default:
		// Unknown intent → try direct passthrough
		callResult, err = srv.handleDirectOp(intentResult.Action, instruction)
	}

	return callResult, err
}

// handleDirectOp executes git operations directly without Ollama
// Delegates to appropriate usecase based on action type
func (srv *Server) handleDirectOp(action, instruction string) (*mcp.CallToolResult, error) {
	var result string
	var err error

	switch action {
	case "push":
		result, err = srv.remote.Push()
	case "pull":
		result, err = srv.remote.Pull()
	case "fetch":
		result, err = srv.remote.Fetch()
	case "create-branch":
		branch := parser.ExtractBranchName(instruction)
		if branch == "" {
			return mcp.NewToolResultError("Could not determine branch name"), nil
		}
		result, err = srv.branch.Create(branch)
	case "checkout":
		branch := parser.ExtractBranchName(instruction)
		if branch == "" {
			return mcp.NewToolResultError("Could not determine branch name"), nil
		}
		result, err = srv.branch.Checkout(branch)
	case "stash":
		result, err = srv.operations.Stash()
	case "reset":
		target := parser.ExtractResetTarget(instruction)
		result, err = srv.operations.Reset("--hard", target)
	case "merge":
		branch := parser.ExtractBranchName(instruction)
		if branch == "" {
			return mcp.NewToolResultError("Could not determine branch name to merge"), nil
		}
		result, err = srv.operations.Merge(branch)
	case "rebase":
		branch := parser.ExtractBranchName(instruction)
		if branch == "" {
			return mcp.NewToolResultError("Could not determine branch to rebase onto"), nil
		}
		result, err = srv.operations.Rebase(branch)
	case "cherry-pick":
		commit := parser.ExtractCommitHash(instruction)
		if commit == "" {
			return mcp.NewToolResultError("Could not determine commit to cherry-pick"), nil
		}
		result, err = srv.operations.CherryPick(commit)
	case "clean":
		result, err = srv.operations.Clean()
	case "tag":
		tag := parser.ExtractTagName(instruction)
		if tag == "" {
			return mcp.NewToolResultError("Could not determine tag name"), nil
		}
		result, err = srv.operations.Tag(tag)
	case "delete-branch":
		branch := parser.ExtractBranchName(instruction)
		if branch == "" {
			return mcp.NewToolResultError("Could not determine branch name to delete"), nil
		}
		result, err = srv.branch.Delete(branch)
	case "blame":
		file := parser.ExtractFileName(instruction)
		if file == "" {
			return mcp.NewToolResultError("Could not determine file to blame"), nil
		}
		result, err = srv.operations.Blame(file)
	case "add":
		files := parser.ExtractFilesToAdd(instruction)
		if len(files) == 0 {
			return mcp.NewToolResultError("Could not determine files to add"), nil
		}
		result, err = srv.operations.Add(files)
	case "revert":
		commit := parser.ExtractCommitHash(instruction)
		if commit == "" {
			return mcp.NewToolResultError("Could not determine commit to revert"), nil
		}
		result, err = srv.operations.Revert(commit)
	case "reflog":
		result, err = srv.operations.Reflog()
	default:
		return mcp.NewToolResultError("Unknown operation: " + action), nil
	}

	if err != nil {
		return mcp.NewToolResultError(action + " failed: " + err.Error()), nil
	}

	return mcp.NewToolResultText(result), nil
}

// handleWrite handles commit operations using Ollama to generate commit messages
func (srv *Server) handleWrite(instruction string, preview bool) (*mcp.CallToolResult, error) {
	result, err := srv.commit.Execute(instruction, preview)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(result), nil
}

// handleReadOnly executes read-only git operations without Ollama
func (srv *Server) handleReadOnly(op string) (*mcp.CallToolResult, error) {
	var result string
	var err error

	switch op {
	case "status":
		result, err = srv.query.Status()
	case "log":
		result, err = srv.query.Log(20)
	case "diff":
		result, err = srv.query.Diff()
	case "branches":
		result, err = srv.query.CurrentBranch()
	case "reflog":
		result, err = srv.operations.Reflog()
	case "blame":
		return mcp.NewToolResultError("Blame is not a read operation - use 'git blame <file>' directly"), nil
	default:
		return mcp.NewToolResultError("Unknown read-only operation: " + op), nil
	}

	if err != nil {
		return mcp.NewToolResultError(op + " failed: " + err.Error()), nil
	}

	return mcp.NewToolResultText(result), nil
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
