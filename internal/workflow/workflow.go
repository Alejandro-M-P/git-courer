// Package workflow implements the unified git workflow engine.
//
// All operations follow four phases:
//  1. PREPARE  — gather git context
//  2. GENERATE — LLM interprets instruction → concrete args
//  3. CONFIRM  — (optional) save plan + blocker; return pending
//  4. EXECUTE  — run the git command
//
// Operations that require confirmation use a three-step MCP protocol:
//
//	*_START  → Run()   → returns {status: "pending_approval", preview: "..."}
//	*_APPLY  → Apply() → executes the saved plan
//	*_ABORT  → Abort() → discards the plan
package workflow

import (
	"crypto/sha256"
	"fmt"

	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// Status values returned in Result.
const (
	StatusCompleted = "completed"
	StatusPending   = "pending_approval"
	StatusAborted   = "aborted"
)

// Result is the return value of Run / Apply / Abort.
type Result struct {
	Status  string            // "completed" | "pending_approval" | "aborted"
	Output  string            // Human-readable output (for completed)
	Preview string            // What will be done (for pending)
	Args    map[string]string // Resolved args from LLM (for pending)
	Summary *Summary          // Structured metadata for the user
}

// Summary provides a high-level overview of the operation's impact.
type Summary struct {
	Operation     string   `json:"operation"`
	FilesAffected []string `json:"files_affected,omitempty"`
	Impact        string   `json:"impact"` // "Low" | "Medium" | "High"
	SecurityCheck string   `json:"security_check"`
	Message       string   `json:"message"`
	Reasoning     string   `json:"reasoning,omitempty"`
	Messages      []string `json:"messages,omitempty"`
}

// Workflow is the main workflow engine for git review operations.
type Workflow struct {
	git       ports.Git
	llm       ports.LLM
	confirm   ports.Confirm
	cfg       *config.Config
	commitSvc *CommitService
	release   *ReleaseService
	security  ports.SecurityService
}

// New creates a new Workflow with all its specialized services.
func New(git ports.Git, llm ports.LLM, confirm ports.Confirm, cfg *config.Config, commit *CommitService, release *ReleaseService, security ports.SecurityService) *Workflow {
	return &Workflow{
		git:       git,
		llm:       llm,
		confirm:   confirm,
		cfg:       cfg,
		commitSvc: commit,
		release:   release,
		security:  security,
	}
}

// RequiresConfirm returns true if the operation needs user confirmation before executing.
// If providedArgs contains preview="false", confirmation is skipped regardless of config.
func (w *Workflow) RequiresConfirm(op string, providedArgs map[string]string) bool {
	// If preview is explicitly set to false, skip confirmation
	if preview, ok := providedArgs["preview"]; ok && preview == "false" {
		return false
	}
	return w.cfg.Preview.IsRequired(op)
}

// computeDiffHash calculates SHA256 hash of current diff (unstaged + staged).
func (w *Workflow) computeDiffHash() (string, error) {
	diff, err := w.git.Diff()
	if err != nil {
		return "", fmt.Errorf("failed to get diff for hash: %w", err)
	}
	diffStaged, err := w.git.DiffStaged()
	if err != nil {
		return "", fmt.Errorf("failed to get staged diff for hash: %w", err)
	}
	combined := diff + "\n--- staged ---\n" + diffStaged
	hash := sha256.Sum256([]byte(combined))
	return fmt.Sprintf("%x", hash), nil
}

func calculateImpact(op string, fileCount int) string {
	if op == "release" || op == "merge" || op == "branch_delete" {
		return "High"
	}
	if fileCount > 10 {
		return "Medium"
	}
	return "Low"
}

func calculateHash(content string) string {
	if content == "" {
		return ""
	}
	h := sha256.New()
	h.Write([]byte(content))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// HasPendingPlan returns true if there is a pending operation waiting for approval.
func (w *Workflow) HasPendingPlan() bool {
	return w.confirm.HasBlocker()
}

// PlanStatus returns a human-readable summary of the current pending plan (if any).
func (w *Workflow) PlanStatus() (string, error) {
	if !w.confirm.HasBlocker() {
		return "No pending review operation.", nil
	}
	plan, err := w.confirm.ReadPlan()
	if err != nil {
		return "", err
	}
	if plan == nil {
		return "No pending review operation.", nil
	}
	return fmt.Sprintf("Pending: %s\nPreview: %s\nCreated: %s",
		plan.Operation,
		plan.Preview,
		fmt.Sprintf("%d", plan.CreatedAt),
	), nil
}
