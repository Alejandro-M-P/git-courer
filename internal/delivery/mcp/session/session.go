package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/delivery/mcp/shared"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// defaultSessionMetaDir is where session metadata JSON files are persisted.
// It lives under domain.MetadataDir (.git/git-courer) so existing metadata
// tooling (auto-staging, cleanup) extends naturally to sessions.
const defaultSessionMetaDir = domain.MetadataDir + "/sessions"

// maxCollisionRetries is the number of extra attempts (with -2..-10 suffixes)
// made when the base session ref already exists. The first attempt uses the
// bare id; suffixes start at -2, so the total attempt count is 1+9 = 10.
const maxCollisionRetries = 9

// worktreeBasePath is the sibling directory holding session worktrees.
// Kept as a constant to match the spec's ../git-courer-worktrees/{id}/ layout.
const worktreeBasePath = "../git-courer-worktrees"

// sessionBranchPrefix is the ref namespace for session branches.
const sessionBranchPrefix = "courer/session-"

// SessionMeta is the on-disk metadata record for a started session, written
// to {metaDir}/{id}.json. Mirrors the design's SessionMeta struct.
type SessionMeta struct {
	ID         string `json:"id"`
	Agent      string `json:"agent"`
	Goal       string `json:"goal"`
	Branch     string `json:"branch"`
	Worktree   string `json:"worktree"`
	BaseCommit string `json:"base_commit"`
	CreatedAt  string `json:"created_at"` // RFC3339
}

// sessionResult is the exact tool result shape: { id, branch, worktree,
// base_commit }. No wrapper fields — the spec forbids additional keys.
type sessionResult struct {
	ID         string `json:"id"`
	Branch     string `json:"branch"`
	Worktree   string `json:"worktree"`
	BaseCommit string `json:"base_commit"`
}

// computeSessionID derives the 8-hex-char session id from goal and base commit.
// Deterministic: the same (goal, baseCommit) pair always yields the same id.
// Pure function — no side effects, trivially testable.
func computeSessionID(goal, baseCommit string) string {
	sum := sha256.Sum256([]byte(goal + baseCommit))
	return hex.EncodeToString(sum[:])[:8]
}

// HandleSession dispatches a session tool request to its subcommand.
// Only "start" is implemented; "finish", "status", "discard" return a
// "not implemented" error. Unknown commands get a suggestion-aware error.
func (h *Handler) HandleSession(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	// Validate that all params are known for this tool.
	if result, err := shared.ValidateKnownParams(params, []string{"command", "agent", "goal"}); result != nil || err != nil {
		return result, err
	}

	command := shared.GetStringParam(params, "command", "")
	if command == "" {
		return shared.JSONErrorResult("session", fmt.Errorf("command is required for session"))
	}

	switch command {
	case "start":
		return h.handleStart(params)
	case "finish", "status", "discard":
		return handleUnimplemented(command)
	default:
		hint := shared.SuggestCommand(command, []string{"start", "finish", "status", "discard"})
		if hint != "" {
			return shared.JSONErrorResult(command, fmt.Errorf("unknown command: %s. Did you mean %s?", command, hint))
		}
		return shared.JSONErrorResult(command, fmt.Errorf("unknown command: %s", command))
	}
}

// handleStart creates an isolated session: a branch ref + a linked worktree +
// a metadata file. On worktree failure it rolls back the branch ref so no
// orphan branch remains. On ref collision it retries with -2..-10 suffixes.
func (h *Handler) handleStart(params map[string]any) (*mcpgo.CallToolResult, error) {
	agent := shared.GetStringParam(params, "agent", "")
	if agent == "" {
		return shared.JSONErrorResult("start", fmt.Errorf("agent is required for start"))
	}
	goal := shared.GetStringParam(params, "goal", "")
	if goal == "" {
		return shared.JSONErrorResult("start", fmt.Errorf("goal is required for start"))
	}

	baseCommit, err := h.git.Head()
	if err != nil {
		return shared.JSONErrorResult("start", fmt.Errorf("failed to read HEAD: %w", err))
	}

	baseID := computeSessionID(goal, baseCommit)

	// Attempt branch creation with collision retry. Suffixes: "" then -2..-10.
	var id, branch, ref string
	for attempt := 0; attempt <= maxCollisionRetries; attempt++ {
		suffix := ""
		if attempt > 0 {
			suffix = fmt.Sprintf("-%d", attempt+1) // attempt 1 -> -2, 2 -> -3, ..., 9 -> -10
		}
		id = baseID + suffix
		branch = sessionBranchPrefix + id
		ref = "refs/heads/" + branch
		if cerr := h.git.CreateRef(ref, baseCommit); cerr == nil {
			break // branch created
		}
		if attempt == maxCollisionRetries {
			return shared.JSONErrorResult("start",
				fmt.Errorf("session namespace exhausted: all %d ref names already exist", maxCollisionRetries+1))
		}
	}

	// Create the worktree bound to the new branch.
	worktreePath := filepath.Join(worktreeBasePath, id)
	wt, werr := h.git.AddWorktree(worktreePath, branch)
	if werr != nil {
		// Rollback the branch ref we just created so no orphan remains.
		// UpdateRef(ref, "") maps to `git update-ref -d <ref>` in the adapter.
		_, _ = h.git.UpdateRef(ref, "")
		return shared.JSONErrorResult("start",
			fmt.Errorf("failed to create worktree: %w (branch ref rolled back)", werr))
	}

	// Persist session metadata. Failure to write metadata does NOT roll back
	// the worktree/branch — the session is usable; metadata is best-effort
	// bookkeeping. We surface the error so the caller knows state diverged.
	meta := SessionMeta{
		ID:         id,
		Agent:      agent,
		Goal:       goal,
		Branch:     branch,
		Worktree:   wt,
		BaseCommit: baseCommit,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	if merr := h.writeMetadata(id, meta); merr != nil {
		return shared.JSONErrorResult("start",
			fmt.Errorf("session created but metadata write failed: %w", merr))
	}

	// Return exactly { id, branch, worktree, base_commit } — no wrapper.
	out := sessionResult{
		ID:         id,
		Branch:     branch,
		Worktree:   wt,
		BaseCommit: baseCommit,
	}
	payload, _ := json.Marshal(out)
	return mcpgo.NewToolResultText(string(payload)), nil
}

// handleUnimplemented returns a "not implemented" error for the deferred
// session subcommands (finish, status, discard). No side effects.
func handleUnimplemented(command string) (*mcpgo.CallToolResult, error) {
	return shared.JSONErrorResult(command,
		fmt.Errorf("%s is not implemented; only start is available", command))
}

// writeMetadata persists the session metadata JSON file at {metaDir}/{id}.json.
// It creates the directory if needed. Existing files (e.g. from a prior session
// with the same id) are overwritten.
func (h *Handler) writeMetadata(id string, meta SessionMeta) error {
	dir := h.metaDir
	if dir == "" {
		dir = defaultSessionMetaDir
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create metadata dir: %w", err)
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	path := filepath.Join(dir, id+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write metadata file: %w", err)
	}
	return nil
}

// parseTime validates an RFC3339 timestamp. Used by tests.
func parseTime(s string) (any, error) {
	return time.Parse(time.RFC3339, s)
}
