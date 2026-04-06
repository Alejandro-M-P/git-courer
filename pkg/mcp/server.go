package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

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
	gitAdapter := git.NewExecAdapter(cfg.Git.WorkDir)

	s := server.NewMCPServer(
		cfg.MCP.Name,
		cfg.MCP.Version,
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)

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
	// git_local_task - THE ONLY ENTRY POINT (SPEC.md requirement)
	// Cloud NEVER touches git - this is the only door
	s.AddTool(
		mcp.NewTool("git_local_task",
			mcp.WithDescription("Execute git operations from natural language. Zero tokens for git decisions."),
			mcp.WithString("instruction",
				mcp.Description("Natural language (e.g., 'commit the login changes', 'show status', 'push')"),
				mcp.Required(),
			),
		),
		handleGitLocalTask(gitAdapter, ollamaAdapter),
	)
}

// handleGitLocalTask - THE ONLY ENTRY POINT FOR GIT
func handleGitLocalTask(gitAdapter *git.ExecAdapter, ollamaAdapter *ollama.Adapter) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		instruction := request.GetString("instruction", "")
		if instruction == "" {
			return mcp.NewToolResultError("instruction is required"), nil
		}

		inst := strings.ToLower(strings.TrimSpace(instruction))

		intent, err := detectIntent(inst)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var result, summary string
		var executedCommands []string

		switch intent {
		case "status":
			status, err := gitAdapter.Status()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			result = formatStatus(status)
			summary = fmt.Sprintf("Git status: %d files", len(status.Files))
			executedCommands = []string{"git status"}

		case "diff":
			diff, err := gitAdapter.Diff()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			result = diff
			if result == "" {
				result = "No changes"
			}
			summary = "Git diff"
			executedCommands = []string{"git diff"}

		case "diff_staged":
			diff, err := gitAdapter.DiffStaged()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			result = diff
			if result == "" {
				result = "No staged changes"
			}
			summary = "Git diff --cached"
			executedCommands = []string{"git diff --cached"}

		case "commit":
			message := extractMessage(instruction)
			if message == "" {
				// Need AI to generate message
				started, err := ollamaAdapter.EnsureOllama()
				if err != nil {
					return mcp.NewToolResultError("Failed to start Ollama: " + err.Error()), nil
				}
				if started {
					log.Println("Ollama started by git-courer")
				}

				diff, err := gitAdapter.DiffStaged()
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				if diff == "" {
					return mcp.NewToolResultError("No staged changes to commit"), nil
				}

				commitMsg, err := ollamaAdapter.GenerateCommitMessage(diff)
				if err != nil {
					return mcp.NewToolResultError("Failed to generate commit message: " + err.Error()), nil
				}
				message = commitMsg.Full
			}

			res, err := gitAdapter.Commit(message)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			result = fmt.Sprintf("Committed: %s\n%s", message, res)
			summary = fmt.Sprintf("Committed with: %s", message)
			executedCommands = []string{"git commit -m \"" + message + "\""}

		case "push":
			res, err := gitAdapter.Push()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			result = res
			summary = "Pushed to remote"
			executedCommands = []string{"git push"}

		case "pull":
			res, err := gitAdapter.Pull()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			result = res
			summary = "Pulled from remote"
			executedCommands = []string{"git pull"}

		case "branch":
			res, err := gitAdapter.CurrentBranch()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			result = fmt.Sprintf("Current branch: %s", res)
			summary = "Listed branches"
			executedCommands = []string{"git branch"}

		case "checkout":
			branch := extractBranch(instruction)
			if branch == "" {
				return mcp.NewToolResultError("branch name required. Example: 'checkout feature/login'"), nil
			}
			res, err := gitAdapter.Checkout(branch)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			result = res
			summary = fmt.Sprintf("Switched to: %s", branch)
			executedCommands = []string{"git checkout " + branch}

		case "stash":
			res, err := gitAdapter.Stash()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			result = res
			summary = "Stashed changes"
			executedCommands = []string{"git stash"}

		case "stash_pop":
			res, err := gitAdapter.StashPop()
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			result = res
			summary = "Applied stash"
			executedCommands = []string{"git stash pop"}

		case "log":
			res, err := gitAdapter.Log(10)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			result = res
			if result == "" {
				result = "No commits"
			}
			summary = "Git log"
			executedCommands = []string{"git log"}

		case "reset":
			res, err := gitAdapter.Reset("mixed", "HEAD")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			result = res
			summary = "Reset to HEAD"
			executedCommands = []string{"git reset"}

		default:
			return mcp.NewToolResultError(fmt.Sprintf("Unknown intent: %s", intent)), nil
		}

		response := map[string]interface{}{
			"result":            result,
			"summary":           summary,
			"intent":            intent,
			"executed_commands": executedCommands,
		}

		jsonResp, err := json.Marshal(response)
		if err != nil {
			return mcp.NewToolResultError("Failed to build response: " + err.Error()), nil
		}

		return mcp.NewToolResultText(string(jsonResp)), nil
	}
}

// detectIntent maps natural language to git operations
func detectIntent(inst string) (string, error) {
	mappings := map[string][]string{
		"status":      {"status", "state", "what"},
		"diff":        {"diff", "changes", "show changes"},
		"diff_staged": {"staged", "ready"},
		"commit":      {"commit", "save"},
		"push":        {"push", "upload"},
		"pull":        {"pull", "download"},
		"branch":      {"branch", "branches"},
		"checkout":    {"checkout", "switch", "go to"},
		"stash":       {"stash", "save changes"},
		"stash_pop":   {"stash pop", "restore"},
		"log":         {"log", "history"},
		"reset":       {"reset", "revert"},
	}

	for intent, patterns := range mappings {
		for _, p := range patterns {
			if strings.Contains(inst, p) {
				return intent, nil
			}
		}
	}

	return "", fmt.Errorf("Could not understand: %s", inst)
}

// extractMessage extracts commit message from instruction
func extractMessage(instruction string) string {
	re := regexp.MustCompile(`(?i)(?:message|with)[:\s]+(.+)`)
	matches := re.FindStringSubmatch(instruction)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// extractBranch extracts branch name from instruction
func extractBranch(instruction string) string {
	re := regexp.MustCompile(`(?i)(?:checkout|switch|to)[:\s]+(.+)`)
	matches := re.FindStringSubmatch(instruction)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
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
	ollamaAdapter := ollama.NewAdapter(cfg.Ollama.Host, cfg.Ollama.Model)
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
