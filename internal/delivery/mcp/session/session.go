package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/core/ports"
	"github.com/blak0p/git-courer/internal/delivery/mcp/shared"
	"github.com/blak0p/git-courer/internal/workflow"
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
const sessionBranchPrefix = ""

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

// slugify converts a string into a URL-friendly slug:
//   - Lowercase
//   - Replace non-alphanumeric chars with '-'
//   - Collapse consecutive '-' into one
//   - Trim leading/trailing '-'
//   - Truncate to 50 chars, trimming any trailing '-' after truncation
func slugify(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	var lastDash bool
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	result := strings.Trim(b.String(), "-")
	if len(result) > 50 {
		result = strings.TrimRight(result[:50], "-")
	}
	return result
}

// computeSessionID derives a human-readable session id from the goal via
// slugify. Deterministic: the same goal always yields the same id.
// Pure function — no side effects, trivially testable.
func computeSessionID(goal string) string {
	return slugify(goal)
}

// HandleSession dispatches a session tool request to its subcommand.
// "start" creates a session; "finish" merges + cleans up; "status" reads
// session state; "discard" removes a session without merging. Unknown
// commands get a suggestion-aware error.
func (h *Handler) HandleSession(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	command := shared.GetStringParam(params, "command", "")
	if command == "" {
		return shared.JSONErrorResult("session", fmt.Errorf("command is required for session"))
	}

	// Validate that all params are known for this tool/command. Each command
	// declares its own allowed parameter set.
	allowed := allowedParams(command)
	if result, err := shared.ValidateKnownParams(params, allowed); result != nil || err != nil {
		return result, err
	}

	switch command {
	case "start":
		return h.handleStart(params)
	case "finish":
		return h.handleFinish(params)
	case "status":
		return h.handleStatus(params)
	case "discard":
		return h.handleDiscard(params)
	default:
		hint := shared.SuggestCommand(command, []string{"start", "finish", "status", "discard"})
		if hint != "" {
			return shared.JSONErrorResult(command, fmt.Errorf("unknown command: %s. Did you mean %s?", command, hint))
		}
		return shared.JSONErrorResult(command, fmt.Errorf("unknown command: %s", command))
	}
}

// allowedParams returns the full set of parameter names allowed for the given
// command. "command" itself is always allowed.
func allowedParams(command string) []string {
	switch command {
	case "start":
		return []string{"command", "agent", "goal", "branch"}
	case "finish":
		return []string{"command", "session_id", "agent"}
	case "status":
		return []string{"command", "session_id"}
	case "discard":
		return []string{"command", "session_id", "confirmed"}
	default:
		return []string{"command", "agent", "goal"}
	}
}

// handleStart creates an isolated session: a branch ref + a linked worktree +
// a metadata file. On worktree failure it rolls back the branch ref so no
// orphan branch remains.
//
// Two modes:
//   - With "branch" param: uses the provided branch name directly. If the
//     branch already exists, returns an error (no retry).
//   - Without "branch" param: derives branch name from the goal slug. On
//     collision retries with -2..-10 suffixes.
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

	id := computeSessionID(goal)
	var branch, ref string

	customBranch := shared.GetStringParam(params, "branch", "")
	if customBranch != "" {
		// Custom branch mode: use the provided name, fail if exists.
		branch = customBranch
		ref = "refs/heads/" + branch
		if cerr := h.git.CreateRef(ref, baseCommit); cerr != nil {
			return shared.JSONErrorResult("start",
				fmt.Errorf("branch already exists: %s", branch))
		}
	} else {
		// Auto branch mode: derive from goal slug with collision retry.
		for attempt := 0; attempt <= maxCollisionRetries; attempt++ {
			suffix := ""
			if attempt > 0 {
				suffix = fmt.Sprintf("-%d", attempt+1) // attempt 1 -> -2, 2 -> -3, ..., 9 -> -10
			}
			id = computeSessionID(goal) + suffix
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

	// Persist the canonical domain.Session via the store when configured so
	// finish/status/discard can read it. Best-effort: a missing store (e.g.
	// in legacy start-only tests) does not block session creation.
	if h.store != nil {
		sess := &domain.Session{
			ID:         id,
			Agent:      agent,
			Goal:       goal,
			Branch:     branch,
			Worktree:   wt,
			BaseCommit: baseCommit,
			BaseBranch: currentBaseBranch(h.git),
			CreatedAt:  meta.CreatedAt,
			Status:     domain.SessionActive,
		}
		if serr := h.store.Save(sess); serr != nil {
			return shared.JSONErrorResult("start",
				fmt.Errorf("session created but store save failed: %w", serr))
		}
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

// handleFinish loads a session, runs preview validation, merges the session
// branch into its base branch, and cleans up the worktree + branch. On
// preview failure or merge conflict the session stays active. On cleanup
// failure the session is marked cleanup_failed (the merge is NOT reverted).
func (h *Handler) handleFinish(params map[string]any) (*mcpgo.CallToolResult, error) {
	sessionID := shared.GetStringParam(params, "session_id", "")
	if sessionID == "" {
		return shared.JSONErrorResult("finish", fmt.Errorf("session_id is required for finish"))
	}

	if h.store == nil {
		return shared.JSONErrorResult("finish", fmt.Errorf("session store not configured"))
	}

	wf := workflow.NewSessionFinishWorkflow(h.git, h.store, h.workDir)
	result, err := wf.Finish(context.Background(), sessionID)
	if err != nil {
		return shared.JSONErrorResult("finish", err)
	}
	payload, _ := json.Marshal(result)
	return mcpgo.NewToolResultText(string(payload)), nil
}

// handleStatus loads a session from the store and returns its current state.
func (h *Handler) handleStatus(params map[string]any) (*mcpgo.CallToolResult, error) {
	sessionID := shared.GetStringParam(params, "session_id", "")
	if sessionID == "" {
		return shared.JSONErrorResult("status", fmt.Errorf("session_id is required for status"))
	}
	if h.store == nil {
		return shared.JSONErrorResult("status", fmt.Errorf("session store not configured"))
	}

	sess, err := h.store.Get(sessionID)
	if err != nil {
		return shared.JSONErrorResult("status", err)
	}
	payload, _ := json.Marshal(sess)
	return mcpgo.NewToolResultText(string(payload)), nil
}

// handleDiscard removes a session without merging: deletes the worktree, the
// session branch, and the session metadata file. Requires confirmed=true to
// guard against accidental data loss.
func (h *Handler) handleDiscard(params map[string]any) (*mcpgo.CallToolResult, error) {
	sessionID := shared.GetStringParam(params, "session_id", "")
	if sessionID == "" {
		return shared.JSONErrorResult("discard", fmt.Errorf("session_id is required for discard"))
	}

	confirmed, _ := params["confirmed"].(bool)
	if !confirmed {
		return shared.JSONErrorResult("discard",
			fmt.Errorf("discard requires confirmed=true to avoid accidental data loss"))
	}

	if h.store == nil {
		return shared.JSONErrorResult("discard", fmt.Errorf("session store not configured"))
	}

	sess, err := h.store.Get(sessionID)
	if err != nil {
		return shared.JSONErrorResult("discard", err)
	}

	var cleanupErrs []string
	if sess.Worktree != "" {
		if werr := h.git.RemoveWorktree(sess.Worktree); werr != nil {
			cleanupErrs = append(cleanupErrs, fmt.Sprintf("remove worktree: %v", werr))
		}
	}
	if _, derr := h.git.DeleteBranch(sess.Branch, true); derr != nil {
		cleanupErrs = append(cleanupErrs, fmt.Sprintf("delete branch: %v", derr))
	}
	_ = h.store.Delete(sess.ID)

	if len(cleanupErrs) > 0 {
		payload, _ := json.Marshal(map[string]any{
			"status":   "partial",
			"message":  "session discarded with errors",
			"errors":   cleanupErrs,
			"session":  sess,
		})
		return mcpgo.NewToolResultText(string(payload)), nil
	}

	payload, _ := json.Marshal(map[string]any{
		"status":  "success",
		"message": fmt.Sprintf("session %q discarded (worktree + branch + metadata removed)", sess.ID),
	})
	return mcpgo.NewToolResultText(string(payload)), nil
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

// currentBaseBranch returns the branch the session was started from, used as
// the merge target for finish. Falls back to "main" when the current branch
// cannot be resolved (e.g. detached HEAD in a worktree).
func currentBaseBranch(git ports.Git) string {
	b, err := git.CurrentBranch()
	if err == nil && b != "" {
		return b
	}
	return "main"
}
