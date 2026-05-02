// Package mcp provides the MCP server and tool handlers for git-courer.
package mcp

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/confirm"
	ghadapter "github.com/Alejandro-M-P/git-courer/internal/adapters/github"
	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/infra/chunkers"
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

	// pendingState tracks in-flight background operations.
	// Key: operation name (e.g. "commit", "release", op slug for generic ops).
	// Value: "processing" | "error: <msg>" | absent (ready / not started).
	pendingState map[string]string

	cfg *config.Config

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
	commitConfirm := confirm.NewInMemory(cfg.Commit.TTL.Duration)
	reviewConfirm := commitConfirm // shared for all operations

	// Resolve context window from unified LLM config
	resolvedCfg, err := cfg.ResolveLLMConfig()
	if err != nil {
		// If config is invalid (missing model), still start server with safe defaults.
		// The error is already surfaced to the user at startup in main.go.
		resolvedCfg = config.LLMConfig{ContextWindow: 0, NumParallel: 1}
	}
	contextWindow := resolvedCfg.ContextWindow

	// Supporting services.
	chunker := chunkers.NewDiffChunker(
		chunkers.WithMaxFilesPerChunk(12),
		chunkers.WithMinForce(3),
	)
	securitySvc := security.New(cfg, llm)
	logChunker := chunkers.NewLogChunker(contextWindow)

	// Specialized engine configs.
	commitCfg := workflow.DefaultCommitServiceConfig(
		contextWindow,
		cfg.Commit.MaxLogLines,
		cfg.Commit.LogPath,
	)
	commitCfg.NumParallel = resolvedCfg.NumParallel

	releaseCfg := workflow.DefaultReleaseServiceConfigWithPaths(
		contextWindow,
		cfg.Release.MaxCommitsPerChunk,
		cfg.Release.MaxLogLines,
		cfg.Release.LogPath,
	)
	releaseCfg.NumParallel = resolvedCfg.NumParallel

	// Create specialized services.
	commitSvc := workflow.NewCommitService(git, llm, chunker, securitySvc, commitCfg)

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
		pendingState:   make(map[string]string),
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

// setOpState stores the state of a background operation (protected by s.mu).
func (s *Server) setOpState(key, state string) {
	s.mu.Lock()
	s.pendingState[key] = state
	s.mu.Unlock()
}

// getOpState retrieves the state of a background operation (protected by s.mu).
func (s *Server) getOpState(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pendingState[key]
}

// clearOpState removes a background operation state entry (protected by s.mu).
func (s *Server) clearOpState(key string) {
	s.mu.Lock()
	delete(s.pendingState, key)
	s.mu.Unlock()
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
