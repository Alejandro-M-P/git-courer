// Package mcp provides the MCP server and tool handlers for git-courer.
package mcp

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/confirm"
	ghadapter "github.com/Alejandro-M-P/git-courer/internal/adapters/github"
	oai "github.com/Alejandro-M-P/git-courer/internal/adapters/llm/openai_standard"
	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/infra/chunkers"
	"github.com/Alejandro-M-P/git-courer/internal/infra/classifier"
	"github.com/Alejandro-M-P/git-courer/internal/security"
	"github.com/Alejandro-M-P/git-courer/internal/workflow"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server holds the MCP server and all injected dependencies.
type Server struct {
	mcpServer *server.MCPServer
	mu        sync.Mutex

	// Core ports
	git ports.Git
	llm ports.LLM

	// Workflow engines
	reviewWorkflow *workflow.Workflow
	commitSvc      *workflow.CommitService
	commitConfirm  ports.Confirm
	releaseConfirm ports.Confirm
	releaseSvc     *workflow.ReleaseService

	cfg *config.Config

	// lastBackup stores the last backup created before a direct write operation.
	lastBackup domain.Backup

	// Client info (captured during initialize handshake)
	clientInfo *domain.ClientInfo
	clientCaps *domain.ClientCapabilities

	// lifecycle manages provider-specific startup/shutdown.
	// Always non-nil — all providers implement ports.Lifecycle.
	lifecycle ports.Lifecycle
}

// SetClientInfo stores client information captured during the MCP initialize handshake.
func (s *Server) SetClientInfo(info *domain.ClientInfo, caps *domain.ClientCapabilities) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clientInfo = info
	s.clientCaps = caps
	log.Printf("Client registered: %s v%s (sampling: %v, elicitation: %v)",
		info.Name, info.Version, caps.Sampling, caps.Elicitation)
}

// New creates and wires up the MCP server with all its dependencies.
// lifecycle must be non-nil — all providers implement ports.Lifecycle.
func New(cfg *config.Config, git ports.Git, llm ports.LLM, lifecycle ports.Lifecycle) *Server {
	// Confirm adapter — in-memory for all operations (commit, branch, merge, etc.)
	// Default 5-minute TTL for confirmation lock.
	commitConfirm := confirm.NewInMemory(5 * time.Minute)
	reviewConfirm := commitConfirm // shared for all operations

	// Resolve context window from config (resolved at install time by ContextResolver).
	contextWindow := cfg.LLM.ContextWindow
	if contextWindow == 0 {
		contextWindow = 8192 // safe default if install didn't resolve it
	}

	// Inject context window into the LLM adapter so it gets the correct num_ctx parameter.
	if ollama, ok := llm.(*oai.OpenAIStandardAdapter); ok {
		ollama.SetNumCtx(contextWindow)
	}

	// Supporting services.
	chunker := chunkers.NewDiffChunker(
		chunkers.WithMaxFilesPerChunk(12),
		chunkers.WithMinForce(3),
	)
	securitySvc := security.New(cfg, llm)
	logChunker := chunkers.NewLogChunker(contextWindow)

	// Specialized engine configs (CommitConfig/ReleaseConfig were removed in Phase 1).
	// Use sensible defaults for maxLogLines and logPath.
	commitCfg := workflow.DefaultCommitServiceConfig(
		contextWindow,
		50,                    // maxLogLines (default)
		".gcourer/commit.log", // logPath
	)
	commitCfg.NumParallel = cfg.LLM.NumParallel

	releaseCfg := workflow.DefaultReleaseServiceConfigWithPaths(
		contextWindow,
		20,                     // maxCommitsPerChunk (default)
		100,                    // maxLogLines (default)
		".gcourer/release.log", // logPath
	)
	releaseCfg.NumParallel = cfg.LLM.NumParallel

	// Create specialized services.
	commitSvc := workflow.NewCommitService(git, llm, chunker, securitySvc, commitCfg)

	// Wire hexagonal dependencies via ports
	var catalogProvider ports.CatalogProvider
	if cp, ok := interface{}(chunker).(ports.CatalogProvider); ok {
		catalogProvider = cp
	}
	var catalog *domain.LanguageCatalog
	if catalogProvider != nil {
		catalog = catalogProvider.GetLanguageCatalog()
	}
	annotator := chunkers.NewChunkAnnotatorAdapter(catalog)
	msgClassifier := classifier.NewClassifierWithCatalog(git, catalog, classifier.WithBinaryClassifier(llm))

	// Load project config and inject custom PathTypes if configured.
	// When PathTypes is empty/nil, the classifier uses DefaultPathTypes in InferCommitType.
	if projectCfg, err := domain.LoadProjectConfig(cfg.Git.WorkDir); err == nil {
		if projectCfg != nil && len(projectCfg.PathTypes) > 0 {
			msgClassifier = classifier.NewClassifierWithCatalog(git, catalog,
				classifier.WithBinaryClassifier(llm),
				classifier.WithPathTypes(projectCfg.PathTypes),
			)
		}
	}
	typeHelper := classifier.NewCommitTypeHelperAdapter()
	commitSvc.SetDependencies(annotator, msgClassifier, typeHelper, catalogProvider)

	// PR enrichment: opt-in via GITHUB_TOKEN env var.
	var githubAPI ports.GitHubAPI
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		githubAPI = ghadapter.NewClient(token)
	}
	releaseSvc := workflow.NewReleaseService(git, llm, logChunker, releaseCfg, githubAPI)

	// Create the main orchestrator with all its tools.
	reviewWorkflow := workflow.New(git, llm, reviewConfirm, cfg, commitSvc, releaseSvc, securitySvc)

	srv := &Server{
		git:            git,
		llm:            llm,
		reviewWorkflow: reviewWorkflow,
		commitSvc:      commitSvc,
		commitConfirm:  commitConfirm,
		releaseConfirm: commitConfirm,
		releaseSvc:     releaseSvc,
		cfg:            cfg,
	}

	// Capture client info during initialize.
	hooks := &server.Hooks{}
	hooks.AddAfterInitialize(func(_ context.Context, _ any, req *mcpgo.InitializeRequest, _ *mcpgo.InitializeResult) {
		log.Printf("Initialize: client=%s v%s", req.Params.ClientInfo.Name, req.Params.ClientInfo.Version)
		srv.SetClientInfo(
			&domain.ClientInfo{Name: req.Params.ClientInfo.Name, Version: req.Params.ClientInfo.Version},
			&domain.ClientCapabilities{
				Sampling:    req.Params.Capabilities.Sampling != nil,
				Elicitation: req.Params.Capabilities.Elicitation != nil,
			},
		)
	})

	s := server.NewMCPServer(
		config.ServerName,
		config.ServerVersion,
		server.WithToolCapabilities(true),
		server.WithRecovery(),
		server.WithHooks(hooks),
		server.WithInstructions(gitCourerSummary),
	)
	srv.mcpServer = s
	registerTools(s, srv)

	srv.lifecycle = lifecycle

	// Start provider lifecycle management.
	log.Println("Starting LLM provider...")
	started, err := lifecycle.EnsureRunning()
	if err != nil {
		log.Printf("⚠ LLM provider not available: %v", err)
	} else {
		if started {
			log.Println("✓ LLM provider started by git-courer")
		}
		if err := lifecycle.PreWarm(); err != nil {
			log.Printf("⚠ Failed to pre-warm model: %v", err)
		} else {
			log.Printf("✓ Model ready")
		}
	}

	return srv
}

// Serve starts the MCP server on stdin/stdout.
func (s *Server) Serve() {
	log.Printf("Starting git-courer MCP server")
	if err := server.ServeStdio(s.mcpServer); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// Stop shuts down the provider lifecycle.
func (s *Server) Stop() {
	s.lifecycle.Stop()
}

// ServeWithAdapter wires everything together and starts serving asynchronously.
func ServeWithAdapter(cfg *config.Config, git ports.Git, llm ports.LLM, lifecycle ports.Lifecycle) *Server {
	srv := New(cfg, git, llm, lifecycle)
	go func() {
		if err := server.ServeStdio(srv.mcpServer); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()
	return srv
}
