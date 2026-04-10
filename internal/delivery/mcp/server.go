// Package mcp provides the MCP server and tool handlers for git-courer.
package mcp

import (
	"context"
	"log"
	"sync"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/confirm"
	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/infra/diff"
	"github.com/Alejandro-M-P/git-courer/internal/security"
	"github.com/Alejandro-M-P/git-courer/internal/workflow"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// OllamaLifecycle abstracts Ollama runtime operations (start, stop, pre-warm).
type OllamaLifecycle interface {
	EnsureOllama() (bool, error)
	PreWarm() error
	Stop()
}

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

	cfg *config.Config

	// Client info (captured during initialize handshake)
	clientInfo *domain.ClientInfo
	clientCaps *domain.ClientCapabilities
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
func New(cfg *config.Config, git ports.Git, llm ports.LLM, ollamaLifecycle OllamaLifecycle) *Server {
	// Confirm adapters — separate file paths for commit vs review.
	commitConfirm := confirm.New(cfg.Git.WorkDir, cfg.Commit)
	reviewConfirm := confirm.New(cfg.Git.WorkDir, confirm.ReviewConfig(cfg.Commit))

	// Supporting services.
	chunker := diff.NewChunker()
	securitySvc := security.New(cfg)

	// Workflow engines.
	commitCfg := workflow.DefaultCommitServiceConfig(
		cfg.Ollama.ContextWindow,
		cfg.Commit.BackgroundThreshold,
		cfg.Commit.MaxLogLines,
		cfg.Commit.LogPath,
	)
	commitSvc := workflow.NewCommitService(git, llm, chunker, securitySvc, commitCfg)
	reviewWorkflow := workflow.New(git, llm, reviewConfirm, cfg)

	srv := &Server{
		git:            git,
		llm:            llm,
		reviewWorkflow: reviewWorkflow,
		commitSvc:      commitSvc,
		commitConfirm:  commitConfirm,
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
		cfg.MCP.Name,
		cfg.MCP.Version,
		server.WithToolCapabilities(true),
		server.WithRecovery(),
		server.WithHooks(hooks),
	)
	srv.mcpServer = s
	registerTools(s, srv)

	// Start Ollama synchronously at startup.
	log.Println("Starting Ollama...")
	started, err := ollamaLifecycle.EnsureOllama()
	if err != nil {
		log.Printf("⚠ Ollama not available: %v", err)
	} else {
		if started {
			log.Println("✓ Ollama started by git-courer")
		}
		if err := ollamaLifecycle.PreWarm(); err != nil {
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

// Stop stops Ollama if we started it.
func (s *Server) Stop(ollamaLifecycle OllamaLifecycle) {
	if ollamaLifecycle != nil {
		ollamaLifecycle.Stop()
	}
}

// ServeWithAdapter wires everything together and starts serving asynchronously.
func ServeWithAdapter(cfg *config.Config, git ports.Git, llm ports.LLM, ollamaLifecycle OllamaLifecycle) *Server {
	srv := New(cfg, git, llm, ollamaLifecycle)
	go func() {
		if err := server.ServeStdio(srv.mcpServer); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()
	return srv
}
