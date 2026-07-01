package prreview

import (
	"context"
	"fmt"
	"strings"

	"github.com/blak0p/git-courer/internal/config"
	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/core/ports"
	"github.com/blak0p/git-courer/internal/delivery/mcp/shared"
	"github.com/blak0p/git-courer/internal/infra/chunkers"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// Handler handles the pr_review MCP tool.
type Handler struct {
	git        ports.Git
	workDir    string
	chunker    *chunkers.DiffChunker
	provider   string
	testRunner testRunnerFunc
}

// testRunnerFunc is the signature for running tests, extracted for testability.
type testRunnerFunc func(ctx context.Context, command string) TestResult

// NewHandler creates a new prreview Handler.
func NewHandler(git ports.Git, workDir string, chunker *chunkers.DiffChunker, provider string) *Handler {
	return &Handler{
		git:        git,
		workDir:    workDir,
		chunker:    chunker,
		provider:   provider,
		testRunner: runTests,
	}
}

// HandlePRReview is the MCP handler for the pr_review tool.
func (h *Handler) HandlePRReview(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	if result, err := shared.ValidateKnownParams(params, []string{"to"}); result != nil || err != nil {
		return result, err
	}

	targetBranch := shared.GetStringParam(params, "to", "main")

	// 1. Get status for branch info and conflict detection
	status, err := h.git.Status()
	if err != nil {
		result := PRReviewResult{
			Status: "error",
			Hint:   fmt.Sprintf("failed to get repository status: %v", err),
		}
		return mcpgo.NewToolResultText(shared.MustJSON(result)), nil
	}

	// 2. Get merge base
	mergeBase, mbErr := h.git.MergeBase(status.Branch, targetBranch)
	if mbErr != nil {
		mergeBase = ""
	}

	// 3. Build branch info
	// Ahead/Behind are *int (nullable): on an unborn repo or a repo with no
	// upstream they are nil. Copy the pointer directly so the null signal
	// propagates to BranchInfo — no dereference needed.
	branchInfo := BranchInfo{
		Name:        status.Branch,
		Ahead:       status.Ahead,
		Behind:      status.Behind,
		MergeBase:   mergeBase,
		HasUpstream: status.HasUpstream,
	}

	// 4. Get diff stats
	var diffStats DiffStats
	if mergeBase != "" {
		diffStats = h.buildDiffStats(mergeBase, status.Branch)
	}

	// 5. Check for conflicts
	if status.Conflicted > 0 {
		conflictInfo := h.buildConflictInfo(status, mergeBase)
		result := PRReviewResult{
			Status:    "conflict",
			Branch:    branchInfo,
			Conflict:  conflictInfo,
			DiffStats: diffStats,
			Hint:      "Resolve merge conflicts before creating a PR.",
		}
		return mcpgo.NewToolResultText(shared.MustJSON(result)), nil
	}

	// 6. Load test_command from project-local config
	testCommand := h.loadTestCommand()

	if testCommand == "" {
		result := PRReviewResult{
			Status: "no_test_command",
			Branch: branchInfo,
			TestResult: &TestResult{
				Status: "skipped",
			},
			DiffStats: diffStats,
			Hint:      "No test command configured. Run config with SET_TEST_COMMAND to enable test checks, e.g.: config SET_TEST_COMMAND \"make test-ci\"",
		}
		return mcpgo.NewToolResultText(shared.MustJSON(result)), nil
	}

	// 7. Run the test command
	testResult := h.testRunner(ctx, testCommand)

	if testResult.Status == "fail" || testResult.Status == "timeout" {
		result := PRReviewResult{
			Status:     "test_fail",
			Branch:     branchInfo,
			TestResult: &testResult,
			DiffStats:  diffStats,
			Hint:       fmt.Sprintf("%d test(s) failed. Fix test failures before creating a PR.", testResult.Failed),
		}
		return mcpgo.NewToolResultText(shared.MustJSON(result)), nil
	}

	// All checks pass
	result := PRReviewResult{
		Status:     "test_ok",
		Branch:     branchInfo,
		TestResult: &testResult,
		DiffStats:  diffStats,
		Hint:       "All checks pass — ready to create a PR.",
	}
	return mcpgo.NewToolResultText(shared.MustJSON(result)), nil
}

// loadTestCommand reads the test_command from .git/git-courer/config.json.
// Returns empty string if the project config doesn't exist or has no test_command.
func (h *Handler) loadTestCommand() string {
	cfg, err := config.LoadProjectConfig(h.workDir)
	if err != nil {
		return ""
	}
	return cfg.TestCommand
}

// buildDiffStats computes file-level diff statistics between base and target.
func (h *Handler) buildDiffStats(base, target string) DiffStats {
	rawDiff, err := h.git.DiffRange(base, target, "..")
	if err != nil || rawDiff == "" {
		return DiffStats{}
	}
	return parseDiffFromOutput(rawDiff)
}

// parseDiffFromOutput extracts per-file statistics from a raw unified diff.
func parseDiffFromOutput(diff string) DiffStats {
	var files []FileInfo
	var totalAdd, totalDel int

	currentFile := ""
	var fileAdd, fileDel int

	lines := strings.Split(diff, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") {
			if currentFile != "" {
				files = append(files, FileInfo{
					Path:      currentFile,
					Additions: fileAdd,
					Deletions: fileDel,
				})
				totalAdd += fileAdd
				totalDel += fileDel
			}
			parts := strings.SplitN(line, " b/", 2)
			if len(parts) == 2 {
				currentFile = strings.TrimSpace(parts[1])
			}
			fileAdd = 0
			fileDel = 0
			continue
		}

		if currentFile != "" {
			if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
				fileAdd++
			} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
				fileDel++
			}
		}
	}

	if currentFile != "" {
		files = append(files, FileInfo{
			Path:      currentFile,
			Additions: fileAdd,
			Deletions: fileDel,
		})
		totalAdd += fileAdd
		totalDel += fileDel
	}

	if files == nil {
		files = []FileInfo{}
	}

	return DiffStats{
		Files:          files,
		TotalAdditions: totalAdd,
		TotalDeletions: totalDel,
	}
}

// buildConflictInfo extracts conflict file paths and annotated diff.
func (h *Handler) buildConflictInfo(status domain.Status, mergeBase string) ConflictInfo {
	var files []string
	for _, f := range status.Files {
		if f.Status == "U" || f.Status == "AA" || f.Status == "UD" ||
			f.Status == "DU" || f.Status == "DD" || f.Status == "AU" ||
			f.Status == "UA" {
			files = append(files, f.Path)
		}
	}
	if files == nil {
		files = []string{}
	}

	var conflictDiff string
	if mergeBase != "" {
		rawDiff, err := h.git.DiffRange(mergeBase, status.Branch, "..")
		if err == nil && rawDiff != "" {
			chunks, chunkErr := h.chunker.Chunk(rawDiff, 0)
			if chunkErr == nil && len(chunks) > 0 {
				var annotated strings.Builder
				for _, c := range chunks {
					if c.AnnotatedDiff != "" {
						fmt.Fprintf(&annotated, "%s\n", c.AnnotatedDiff)
					} else {
						fmt.Fprintf(&annotated, "%s\n", c.Diff)
					}
				}
				sanitized := shared.SanitizeDiffForProvider(annotated.String(), 0, 500, h.provider)
				conflictDiff = sanitized.Diff
			} else {
				sanitized := shared.SanitizeDiffForProvider(rawDiff, 0, 500, h.provider)
				conflictDiff = sanitized.Diff
			}
		}
	}

	return ConflictInfo{
		Files: files,
		Diff:  conflictDiff,
	}
}
