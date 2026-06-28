package mcp

import (
	"fmt"
	"log"
	"path/filepath"
	"sync/atomic"

	"github.com/blak0p/git-courer/internal/adapters/git"
	"github.com/blak0p/git-courer/internal/adapters/sessionstore"
	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/core/ports"
	"github.com/blak0p/git-courer/internal/delivery/mcp/branch"
	"github.com/blak0p/git-courer/internal/delivery/mcp/core"
	"github.com/blak0p/git-courer/internal/delivery/mcp/history"
	"github.com/blak0p/git-courer/internal/delivery/mcp/integrate"
	"github.com/blak0p/git-courer/internal/delivery/mcp/prreview"
	"github.com/blak0p/git-courer/internal/delivery/mcp/rewrite"
	"github.com/blak0p/git-courer/internal/delivery/mcp/session"
	"github.com/blak0p/git-courer/internal/delivery/mcp/stage"
	mcpsync "github.com/blak0p/git-courer/internal/delivery/mcp/sync"
	"github.com/blak0p/git-courer/internal/delivery/mcp/utility"
	"github.com/blak0p/git-courer/internal/infra/chunkers"
	"github.com/blak0p/git-courer/internal/workflow"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	StagedMarker   = "=== STAGED ==="
	UnstagedMarker = "=== UNSTAGED ==="
)

// sendErrorNotification sends a structured error notification to all clients.
func (s *Server) sendErrorNotification(operation, message string, details map[string]any) {
	if s.mcpServer == nil {
		return
	}
	s.mcpServer.SendNotificationToAllClients("notifications/message", map[string]any{
		"level":     "error",
		"logger":    "git-courer",
		"operation": operation,
		"message":   "❌ Error: " + message,
		"data":      details,
		"hint":      "Verify your git state and configuration. Use ABORT if needed to reset.",
	})
}

// sendSuccessNotification sends a structured success notification to all clients.
func (s *Server) sendSuccessNotification(operation, message string, summary any) {
	data := map[string]any{
		"operation": operation,
		"message":   "✅ Success: " + message,
	}

	if summ, ok := summary.(*workflow.Summary); ok && summ != nil {
		data["operation"] = summ.Operation
		data["files"] = summ.FilesAffected
		data["impact"] = summ.Impact
		data["security"] = summ.SecurityCheck
		if summ.Message != "" {
			data["message"] = "✅ " + summ.Message
		}
		if summ.Reasoning != "" {
			data["reasoning"] = summ.Reasoning
		}
		if len(summ.Messages) > 0 {
			data["messages"] = summ.Messages
		}
	} else if summary != nil {
		data["details"] = summary
	}

	if s.mcpServer == nil {
		return
	}
	s.mcpServer.SendNotificationToAllClients("notifications/message", map[string]any{
		"level":     "info",
		"logger":    "git-courer",
		"operation": operation,
		"message":   data["message"],
		"data":      data,
	})
}

// sendSecurityErrorNotification sends a structured error notification for security blocks.
func (s *Server) sendSecurityErrorNotification(errMsg string) {
	if s.mcpServer == nil {
		return
	}
	s.mcpServer.SendNotificationToAllClients("notifications/message", map[string]any{
		"level":     "error",
		"logger":    "git-courer",
		"operation": "security",
		"error":     "SECRET_DETECTED",
		"message":   "🔒 SECURITY BLOCK: Credentials or secrets detected in the diff.",
		"data": map[string]any{
			"details": errMsg,
			"action":  "Removing sensitive data before committing is a best practice. Use environment variables instead.",
		},
	})
}

// registerTools registers the MCP tools on the server.
func registerTools(s *server.MCPServer, srv *Server) {
	provider := ""
	if srv.cfg != nil {
		provider = srv.cfg.LLM.Provider
	}

	// Wrap the injected git adapter with a session-aware wrapper so that,
	// when an agent calls `session select <id>`, all subsequent git operations
	// target that session's worktree. Worktree-management ops
	// (AddWorktree/RemoveWorktree/CreateRef) always go to a fresh main-repo
	// adapter regardless of the active session.
	var activeSessionPtr *atomic.Value
	var mainGit ports.Git
	if execAdapter, ok := srv.git.(*git.ExecAdapter); ok {
		mainGit = git.New(execAdapter.WorkDir())
		wrapper := git.NewSessionWrapper(execAdapter, mainGit, &srv.activeSession)
		execAdapter.SetWorkDirFn(func() string {
			if v := srv.activeSession.Load(); v != nil {
				if sess, ok := v.(*domain.Session); ok && sess != nil && sess.Worktree != "" {
					return sess.Worktree
				}
			}
			return execAdapter.WorkDir()
		})
		srv.git = wrapper
		activeSessionPtr = &srv.activeSession
	}
	if mainGit == nil {
		mainGit = git.New(".")
	}

	coreHandler := core.NewHandler(srv.git, srv.commitSvc, srv.reviewWorkflow, srv.llm, provider, s, git.NewGitContentProvider("."))
	if srv.cfg != nil {
		coreHandler.SetLLMEnabled(srv.cfg.LLM.Enabled)
	}
	core.Register(s, coreHandler)

	branchHandler := branch.NewHandler(srv.git)
	branch.Register(s, branchHandler)

	// workDir: git-courer operates in the current working directory;
	// .git/git-courer/config.json is loaded from the CWD.
	workDir := "."

	// Resolve the session metadata directory from the git common dir so
	// session records persist in the main repo's .git/git-courer/sessions even
	// when the handler is invoked from a linked worktree. Falls back to the
	// default metadata dir when git is unavailable (e.g. registration tests).
	sessionMetaDir := filepath.Join(domain.MetadataDir, "sessions")
	if srv.git != nil {
		if commonDir, cerr := srv.git.GitCommonDir(); cerr == nil && commonDir != "" {
			sessionMetaDir = filepath.Join(commonDir, "git-courer", "sessions")
		}
	}
	sessionStore := sessionstore.NewFSSessionStore(sessionMetaDir)
	sessionHandler := session.NewHandlerWithStore(srv.git, sessionStore, workDir, activeSessionPtr)
	session.Register(s, sessionHandler)

	rewriteHandler := rewrite.NewHandler(srv.git)
	rewrite.Register(s, rewriteHandler)

	integrateHandler := integrate.NewHandler(srv.git)
	integrate.Register(s, integrateHandler)

	stageHandler := stage.NewHandler(srv.git, nil)
	stage.Register(s, stageHandler)

	historyHandler := history.NewHandler(srv.git)
	history.Register(s, historyHandler)

	syncHandler := mcpsync.NewHandler(srv.git)
	mcpsync.Register(s, syncHandler)

	// Create chunker for pr_review (same config as used in New())
	chunker := chunkers.NewDiffChunker(
		chunkers.WithMaxFilesPerChunk(12),
		chunkers.WithMinForce(3),
	)

	utilityHandler := utility.NewHandler(srv.git, srv.cfg, workDir, srv.mcpServer)
	utility.Register(s, utilityHandler)

	prReviewHandler := prreview.NewHandler(srv.git, workDir, chunker, provider)
	prreview.Register(s, prReviewHandler)
}

// Interface implementation shims for domain sub-packages — removed.
// Core domain handlers (status, diff, commit, amend, revert) are now in core.Handler.

func (s *Server) applyWithBackup(operation string, stashUntracked bool, fn func() (workflow.Result, error)) (workflow.Result, error) {
	mode := domain.StashUnstaged
	if stashUntracked {
		mode = domain.StashAll
	}
	backup, bErr := s.git.CreateBackup(operation, mode)
	if bErr != nil {
		log.Printf("⚠ backup creation failed for %s: %v — proceeding without backup", operation, bErr)
		return fn()
	}

	res, fnErr := fn()
	if fnErr != nil {
		if rErr := s.git.RestoreBackup(backup); rErr != nil {
			s.git.DeleteBackup(backup)
			return workflow.Result{}, fmt.Errorf("operation failed and restore also failed: %v (restore: %v)", fnErr, rErr)
		}
		s.git.DeleteBackup(backup)
		return workflow.Result{}, fmt.Errorf("operation failed - repo restored: %v", fnErr)
	}

	s.git.DeleteBackup(backup)
	return res, nil
}

// JSONErrorResult returns a structured JSON error.
func JSONErrorResult(command string, err error) (*mcpgo.CallToolResult, error) {
	errJSON := fmt.Sprintf(`{"status":"error","command":%q,"error":%q}`, command, err.Error())
	return mcpgo.NewToolResultError(errJSON), nil
}

func ParsePagination(params map[string]any) (limit, offset int) {
	if v, ok := params["limit"].(float64); ok {
		limit = int(v)
	}
	if v, ok := params["offset"].(float64); ok {
		offset = int(v)
	}
	return
}
