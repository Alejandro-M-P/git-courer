package prreview

// PRReviewResult is the structured output of the pr-review tool.
// Status is one of: no_test_command, test_fail, conflict, test_ok, error.
type PRReviewResult struct {
	Status     string       `json:"status"`
	Branch     BranchInfo   `json:"branch"`
	Conflict   ConflictInfo `json:"conflict,omitempty"`
	TestResult *TestResult  `json:"test_result,omitempty"`
	DiffStats  DiffStats    `json:"diff_stats"`
	Hint       string       `json:"hint"`
}

// BranchInfo holds branch divergence data.
type BranchInfo struct {
	Name        string `json:"name"`
	Ahead       *int   `json:"ahead"`
	Behind      *int   `json:"behind"`
	MergeBase   string `json:"merge_base"`
	HasUpstream bool   `json:"has_upstream"`
}

// ConflictInfo holds conflict file list and annotated diff.
type ConflictInfo struct {
	Files []string `json:"files"`
	Diff  string   `json:"diff"`
}

// TestResult holds the outcome of running the test command.
type TestResult struct {
	Status       string        `json:"status"` // pass | fail | skipped | timeout
	Total        int           `json:"total"`
	Failed       int           `json:"failed"`
	FailingTests []FailingTest `json:"failing_tests,omitempty"`
	Output       string        `json:"output,omitempty"`
}

// FailingTest represents a single failed test extracted from go test -json output.
type FailingTest struct {
	Package  string `json:"package"`
	TestName string `json:"test_name"`
	Output   string `json:"output"`
}

// DiffStats holds per-file diff statistics.
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
