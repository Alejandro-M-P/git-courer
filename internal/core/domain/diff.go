package domain

// CFGCount represents control-flow node counts per category.
// Zero value means "computed but none found". Use *CFGCount on DiffChunk
// where nil means "not computed".
type CFGCount struct {
	Branch int `json:"branch"`
	Loop   int `json:"loop"`
	Return int `json:"return"`
	Error  int `json:"error"`
}

// CFGDiff represents control-flow changes between before and after snapshots.
type CFGDiff struct {
	Before CFGCount `json:"before"`
	After  CFGCount `json:"after"`
}


// small enough to be processed by an LLM in a single context window.
type DiffChunk struct {
	// Files is the list of files included in this chunk.
	Files []string `json:"files"`
	// Diff is the unified diff content for these files.
	Diff string `json:"diff"`
	// CommitType is the pre-decided type (feat, fix, refactor, docs, test, chore, ci) or empty.
	CommitType string `json:"commit_type"`
	// AnnotatedDiff is the diff grouped by file with semantic labels per function/type.
	AnnotatedDiff string `json:"annotated_diff"`
	// ConfidenceScore is the classifier's confidence (0.0–1.0) in the pre-classified CommitType.
	// 0.0 means no classification was performed.
	ConfidenceScore float64 `json:"confidence_score"`
	// Scope is the functional area resolved from project config areas (e.g. "security", "core").
	// Empty if no area matches or init hasn't been run.
	Scope string `json:"scope"`
	// GoBefore holds the before-version Go source code per file path.
	// Populated by the annotation step for .go files to enable AST identity detection.
	// Nil or empty map means no source available (fall through to next pillar).
	GoBefore map[string]string `json:"go_before,omitempty"`
	// GoAfter holds the after-version Go source code per file path.
	// Populated by the annotation step for .go files to enable AST identity detection.
	// Nil or empty map means no source available (fall through to next pillar).
	GoAfter map[string]string `json:"go_after,omitempty"`
	// CFGBefore holds the before-version CFG snapshot; nil means not computed.
	CFGBefore *CFGCount `json:"cfg_before,omitempty"`
	// CFGAfter holds the after-version CFG snapshot; nil means not computed.
	CFGAfter *CFGCount `json:"cfg_after,omitempty"`
}
