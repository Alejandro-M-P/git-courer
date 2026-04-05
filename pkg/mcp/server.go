package mcp

import (
	"context"
	"fmt"
	"log"

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
	// Create git adapter
	gitAdapter := git.NewExecAdapter(cfg.Git.WorkDir)

	// Create MCP server
	s := server.NewMCPServer(
		cfg.MCP.Name,
		cfg.MCP.Version,
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)

	// Register tools
	registerTools(s, gitAdapter, ollamaAdapter)

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
	// git_status tool
	s.AddTool(
		mcp.NewTool("git_status",
			mcp.WithDescription("Get current git repository status"),
		),
		handleGitStatus(gitAdapter),
	)

	// git_diff tool
	s.AddTool(
		mcp.NewTool("git_diff",
			mcp.WithDescription("Show changes in the working directory"),
			mcp.WithBoolean("staged",
				mcp.Description("Show staged changes only"),
				mcp.DefaultBool(false),
			),
		),
		handleGitDiff(gitAdapter),
	)

	// git_log tool
	s.AddTool(
		mcp.NewTool("git_log",
			mcp.WithDescription("Show commit history"),
			mcp.WithNumber("limit",
				mcp.Description("Number of commits to show"),
				mcp.DefaultNumber(10),
			),
		),
		handleGitLog(gitAdapter),
	)

	// git_add tool
	s.AddTool(
		mcp.NewTool("git_add",
			mcp.WithDescription("Stage files for commit"),
			mcp.WithString("paths",
				mcp.Description("Files to stage (space separated)"),
				mcp.Required(),
			),
		),
		handleGitAdd(gitAdapter),
	)

	// git_commit tool
	s.AddTool(
		mcp.NewTool("git_commit",
			mcp.WithDescription("Create a commit with staged changes"),
			mcp.WithString("message",
				mcp.Description("Commit message"),
				mcp.Required(),
			),
		),
		handleGitCommit(gitAdapter),
	)

	// git_push tool
	s.AddTool(
		mcp.NewTool("git_push",
			mcp.WithDescription("Push commits to remote"),
		),
		handleGitPush(gitAdapter),
	)

	// git_pull tool
	s.AddTool(
		mcp.NewTool("git_pull",
			mcp.WithDescription("Pull changes from remote"),
		),
		handleGitPull(gitAdapter),
	)

	// git_branch tool
	s.AddTool(
		mcp.NewTool("git_branch",
			mcp.WithDescription("Create or list branches"),
			mcp.WithString("name",
				mcp.Description("New branch name (optional, omit to list)"),
			),
			mcp.WithString("checkout",
				mcp.Description("Branch to checkout"),
			),
		),
		handleGitBranch(gitAdapter),
	)

	// git_checkout tool
	s.AddTool(
		mcp.NewTool("git_checkout",
			mcp.WithDescription("Switch branches or restore files"),
			mcp.WithString("branch",
				mcp.Description("Branch name"),
				mcp.Required(),
			),
		),
		handleGitCheckout(gitAdapter),
	)

	// git_stash tool
	s.AddTool(
		mcp.NewTool("git_stash",
			mcp.WithDescription("Stash changes"),
			mcp.WithBoolean("pop",
				mcp.Description("Apply stashed changes"),
				mcp.DefaultBool(false),
			),
		),
		handleGitStash(gitAdapter),
	)

	// git_reset tool
	s.AddTool(
		mcp.NewTool("git_reset",
			mcp.WithDescription("Reset changes"),
			mcp.WithString("mode",
				mcp.Description("Reset mode: soft, mixed, hard"),
				mcp.DefaultString("mixed"),
			),
			mcp.WithString("commit",
				mcp.Description("Commit to reset to"),
				mcp.DefaultString("HEAD"),
			),
		),
		handleGitReset(gitAdapter),
	)

	// git_ai_commit - AI generates commit message
	s.AddTool(
		mcp.NewTool("git_ai_commit",
			mcp.WithDescription("Generate AI commit message and commit"),
		),
		handleGitAICommit(gitAdapter, ollamaAdapter),
	)
}

// Tool handlers

func handleGitStatus(adapter *git.ExecAdapter) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		status, err := adapter.Status()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		output := formatStatus(status)
		return mcp.NewToolResultText(output), nil
	}
}

func handleGitDiff(adapter *git.ExecAdapter) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		staged := request.GetBool("staged", false)

		var diff string
		var err error

		if staged {
			diff, err = adapter.DiffStaged()
		} else {
			diff, err = adapter.Diff()
		}

		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if diff == "" {
			return mcp.NewToolResultText("No changes"), nil
		}

		return mcp.NewToolResultText(diff), nil
	}
}

func handleGitLog(adapter *git.ExecAdapter) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limitStr := request.GetString("limit", "10")
		limit := 10
		fmt.Sscanf(limitStr, "%d", &limit)

		log, err := adapter.Log(limit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if log == "" {
			return mcp.NewToolResultText("No commits"), nil
		}

		return mcp.NewToolResultText(log), nil
	}
}

func handleGitAdd(adapter *git.ExecAdapter) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pathsStr := request.GetString("paths", "")
		paths := []string{pathsStr}

		err := adapter.Add(paths)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Staged: %s", pathsStr)), nil
	}
}

func handleGitCommit(adapter *git.ExecAdapter) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		message := request.GetString("message", "")

		result, err := adapter.Commit(message)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(result), nil
	}
}

func handleGitPush(adapter *git.ExecAdapter) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := adapter.Push()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(result), nil
	}
}

func handleGitPull(adapter *git.ExecAdapter) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := adapter.Pull()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(result), nil
	}
}

func handleGitBranch(adapter *git.ExecAdapter) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := request.GetString("name", "")
		checkout := request.GetString("checkout", "")

		if checkout != "" {
			result, err := adapter.Checkout(checkout)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(result), nil
		}

		if name != "" {
			result, err := adapter.Branch(name)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return mcp.NewToolResultText(result), nil
		}

		current, err := adapter.CurrentBranch()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Current branch: %s", current)), nil
	}
}

func handleGitCheckout(adapter *git.ExecAdapter) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		branch := request.GetString("branch", "")

		result, err := adapter.Checkout(branch)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(result), nil
	}
}

func handleGitStash(adapter *git.ExecAdapter) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pop := request.GetBool("pop", false)

		var result string
		var err error

		if pop {
			result, err = adapter.StashPop()
		} else {
			result, err = adapter.Stash()
		}

		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(result), nil
	}
}

func handleGitReset(adapter *git.ExecAdapter) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mode := request.GetString("mode", "mixed")
		commit := request.GetString("commit", "HEAD")

		result, err := adapter.Reset(mode, commit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(result), nil
	}
}

// Helper functions

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

// Serve starts the MCP server (legacy - for backward compatibility)
func Serve(cfg *config.Config) {
	ollamaAdapter := ollama.NewAdapter(cfg.Ollama.Host, cfg.Ollama.Model)
	s := NewServer(cfg, ollamaAdapter)
	log.Printf("Starting git-courer MCP server v%s", cfg.MCP.Version)
	if err := server.ServeStdio(s.mcpServer); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// ServeWithAdapter starts the MCP server with a pre-configured Ollama adapter
func ServeWithAdapter(cfg *config.Config, ollamaAdapter *ollama.Adapter) *Server {
	s := NewServer(cfg, ollamaAdapter)
	log.Printf("Starting git-courer MCP server v%s", cfg.MCP.Version)

	// Start server in a goroutine to allow graceful shutdown
	go func() {
		if err := server.ServeStdio(s.mcpServer); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	return s
}

// handleGitAICommit generates an AI commit message and creates the commit
func handleGitAICommit(gitAdapter *git.ExecAdapter, ollamaAdapter *ollama.Adapter) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Ensure Ollama is running (lazy start if needed)
		started, err := ollamaAdapter.EnsureOllama()
		if err != nil {
			return mcp.NewToolResultError("Failed to start Ollama: " + err.Error()), nil
		}
		if started {
			log.Println("🔄 Ollama started by git-courer")
		}

		// Get staged diff
		diff, err := gitAdapter.DiffStaged()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if diff == "" {
			return mcp.NewToolResultText("No staged changes to commit. Use git_add first."), nil
		}

		// Generate commit message with AI
		commitMsg, err := ollamaAdapter.GenerateCommitMessage(diff)
		if err != nil {
			return mcp.NewToolResultError("Failed to generate commit message: " + err.Error()), nil
		}

		// Create the commit
		result, err := gitAdapter.Commit(commitMsg.Full)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("✓ Committed: %s\n\n%s", commitMsg.Full, result)), nil
	}
}
