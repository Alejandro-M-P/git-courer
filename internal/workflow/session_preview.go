// Package workflow contains the PreviewEngine used by session finish to
// validate a session before merging: uncommitted changes check, test
// command, and a dry-run merge conflict check.
package workflow

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/blak0p/git-courer/internal/config"
	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/core/ports"
)

const previewTestTimeout = 120 * time.Second

// TestResult holds the outcome of running the configured test command.
// Mirrors prreview.TestResult so callers can serialize it the same way.
type TestResult struct {
	Status       string        `json:"status"` // pass | fail | skipped | timeout
	Total        int           `json:"total"`
	Failed       int           `json:"failed"`
	FailingTests []FailingTest `json:"failing_tests,omitempty"`
	Output       string        `json:"output,omitempty"`
}

// FailingTest represents a single failed test extracted from go test -json.
type FailingTest struct {
	Package  string `json:"package"`
	TestName string `json:"test_name"`
	Output   string `json:"output"`
}

// DiffStats holds per-file diff statistics for the preview.
type DiffStats struct {
	Files          []FileInfo `json:"files"`
	TotalAdditions int        `json:"total_additions"`
	TotalDeletions int        `json:"total_deletions"`
}

// FileInfo holds additions/deletions for a single changed file.
type FileInfo struct {
	Path      string `json:"path"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

// PreviewResult is the outcome of PreviewEngine.Preview.
type PreviewResult struct {
	HasUncommitted bool        `json:"has_uncommitted"`
	TestResult     *TestResult `json:"test_result,omitempty"`
	HasConflict    bool        `json:"has_conflict"`
	ConflictFiles  []string    `json:"conflict_files,omitempty"`
	DiffStats      DiffStats   `json:"diff_stats"`
}

// PreviewEngine validates a session before finishing it. It is shared in
// spirit with pr_review's pre-PR gate but operates from the session worktree.
type PreviewEngine struct {
	git          ports.Git
	workDir      string
	testRunner   func(ctx context.Context, command string) TestResult
	configLoader func(workDir string) (string, error)
}

// NewPreviewEngine builds a PreviewEngine rooted at workDir using git.
func NewPreviewEngine(git ports.Git, workDir string) *PreviewEngine {
	return &PreviewEngine{
		git:          git,
		workDir:      workDir,
		testRunner:   runPreviewTests,
		configLoader: loadTestCommand,
	}
}

// Preview runs all validation checks for a session branch against baseBranch.
// It leaves the repository unchanged: any in-progress merge is aborted and,
// when the dry-run merge succeeded cleanly, HEAD is reset back to its
// pre-merge position so no merge commit pollutes the session branch.
func (p *PreviewEngine) Preview(ctx context.Context, baseBranch string) (*PreviewResult, error) {
	result := &PreviewResult{ConflictFiles: []string{}}

	// 1. Uncommitted/untracked changes check.
	status, err := p.git.Status()
	if err != nil {
		return nil, fmt.Errorf("preview: status: %w", err)
	}
	if !status.IsClean {
		result.HasUncommitted = true
	}

	// 2. Run the configured test command, if any.
	cmd, cerr := p.configLoader(p.workDir)
	if cerr == nil && cmd != "" {
		tr := p.testRunner(ctx, cmd)
		result.TestResult = &tr
	} else {
		result.TestResult = &TestResult{Status: "skipped"}
	}

	// 3. Dry-run merge conflict check. We capture HEAD before the merge so a
	// clean merge can be rolled back, leaving the session branch untouched.
	preMergeHead, _ := p.git.Head()
	if _, merr := p.git.Merge(baseBranch); merr != nil {
		// Merge may fail due to conflicts; conflict detection is via status.
	}
	afterStatus, serr := p.git.Status()
	if serr == nil && afterStatus.Conflicted > 0 {
		result.HasConflict = true
		for _, f := range afterStatus.Files {
			if isConflictStatus(f.Status) {
				result.ConflictFiles = append(result.ConflictFiles, f.Path)
			}
		}
	}
	// Abort the in-progress merge (no-op + error when there is nothing to
	// abort, which we ignore).
	_, _ = p.git.MergeAbort()
	// If the merge succeeded cleanly (no conflict), HEAD moved to a merge
	// commit. Reset back to the pre-merge HEAD so the session branch is
	// unchanged. best-effort: ignore reset errors.
	if preMergeHead != "" && !result.HasConflict {
		_, _ = p.git.Reset("--hard", preMergeHead)
	}

	// 4. Diff stats against the merge base (best-effort).
	mb, mbErr := p.git.MergeBase(baseBranch, currentBranchName(status))
	if mbErr == nil && mb != "" {
		result.DiffStats = buildPreviewDiffStats(p.git, mb, currentBranchName(status))
	}

	return result, nil
}

// isConflictStatus reports whether a porcelain status code indicates a conflict.
func isConflictStatus(s string) bool {
	switch s {
	case "U", "AA", "DD", "AU", "UD", "DU", "UA":
		return true
	}
	return false
}

func currentBranchName(s domain.Status) string {
	if s.Branch != "" {
		return s.Branch
	}
	return "HEAD"
}

// loadTestCommand reads the test_command from .git/git-courer/config.json.
// Returns an empty string (with nil error semantics handled by caller) when
// no test command is configured.
func loadTestCommand(workDir string) (string, error) {
	cfg, err := config.LoadProjectConfig(workDir)
	if err != nil {
		return "", err
	}
	return cfg.TestCommand, nil
}

// runPreviewTests executes the test command with a 120s timeout and parses
// the output. Mirrors prreview.runTests so the workflow layer stays free of
// delivery-package imports.
func runPreviewTests(ctx context.Context, command string) TestResult {
	ctx, cancel := context.WithTimeout(ctx, previewTestTimeout)
	defer cancel()

	isGoTest := strings.HasPrefix(strings.TrimSpace(command), "go test")
	execCmd := buildPreviewCommand(ctx, command, isGoTest)

	output, execErr := execCmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return TestResult{
			Status: "timeout",
			Output: fmt.Sprintf("test command timed out after %v", previewTestTimeout),
		}
	}

	exitCode := 0
	if execErr != nil {
		if exitErr, ok := execErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return TestResult{
				Status: "fail",
				Output: fmt.Sprintf("failed to run test command: %v", execErr),
			}
		}
	}

	outputStr := string(output)

	if isGoTest {
		result := parseGoTestJSONPreview(outputStr)
		if result.Status == "fail" {
			return result
		}
		if exitCode != 0 {
			result.Status = "fail"
		}
		return result
	}

	if exitCode != 0 {
		return TestResult{
			Status: "fail",
			Output: truncatePreview(outputStr, 500),
		}
	}

	return TestResult{
		Status: "pass",
		Output: truncatePreview(outputStr, 500),
	}
}

func buildPreviewCommand(ctx context.Context, command string, isGoTest bool) *exec.Cmd {
	parts := strings.Fields(command)
	if isGoTest && !hasJSONFlagPreview(parts) {
		parts = append(parts, "-json")
	}
	return exec.CommandContext(ctx, parts[0], parts[1:]...)
}

func hasJSONFlagPreview(parts []string) bool {
	for _, p := range parts {
		if p == "-json" {
			return true
		}
	}
	return false
}

func parseGoTestJSONPreview(output string) TestResult {
	var failingTests []FailingTest
	var totalTests int
	failedPkgs := make(map[string]bool)
	testOutputs := make(map[string]string)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		action, _ := entry["Action"].(string)
		pkg, _ := entry["Package"].(string)
		testName, _ := entry["Test"].(string)

		switch action {
		case "pass":
			if testName != "" {
				totalTests++
			}
		case "fail":
			if testName != "" {
				totalTests++
				key := pkg + "/" + testName
				failingTests = append(failingTests, FailingTest{
					Package:  pkg,
					TestName: testName,
					Output:   truncatePreview(testOutputs[key], 500),
				})
			} else {
				failedPkgs[pkg] = true
			}
		case "output":
			if testName != "" {
				key := pkg + "/" + testName
				text, _ := entry["Output"].(string)
				testOutputs[key] += text + "\n"
			}
		}
	}

	if len(failingTests) > 0 || len(failedPkgs) > 0 {
		return TestResult{
			Status:       "fail",
			Total:        totalTests,
			Failed:       len(failingTests) + len(failedPkgs),
			FailingTests: failingTests,
			Output:       truncatePreview(output, 500),
		}
	}
	return TestResult{
		Status: "pass",
		Total:  totalTests,
		Output: truncatePreview(output, 500),
	}
}

func truncatePreview(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// buildPreviewDiffStats computes file-level diff statistics between base and
// target. Best-effort: returns empty DiffStats on error.
func buildPreviewDiffStats(git ports.Git, base, target string) DiffStats {
	raw, err := git.DiffRange(base, target, "..")
	if err != nil || raw == "" {
		return DiffStats{Files: []FileInfo{}}
	}
	return parsePreviewDiff(raw)
}

func parsePreviewDiff(diff string) DiffStats {
	var files []FileInfo
	var totalAdd, totalDel int
	currentFile := ""
	var fileAdd, fileDel int

	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git") {
			if currentFile != "" {
				files = append(files, FileInfo{Path: currentFile, Additions: fileAdd, Deletions: fileDel})
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
		files = append(files, FileInfo{Path: currentFile, Additions: fileAdd, Deletions: fileDel})
		totalAdd += fileAdd
		totalDel += fileDel
	}
	if files == nil {
		files = []FileInfo{}
	}
	return DiffStats{Files: files, TotalAdditions: totalAdd, TotalDeletions: totalDel}
}