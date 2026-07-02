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

// AnnotatedEntry is a structured semantic annotation for a single symbol change.
// It replaces the lossy emoji-prefixed plain-text annotation with machine-readable
// JSON carrying per-symbol before/after hunk lines, call edges, and metadata.
type AnnotatedEntry struct {
	File     string   `json:"file"`
	Symbol   string   `json:"symbol"`
	Type     string   `json:"type"`
	Breaking bool     `json:"breaking"`
	Line     int      `json:"line"`
	Calls    []string `json:"calls,omitempty"`
	Before   string   `json:"before"`
	After    string   `json:"after"`
}

// CallGraphEntry describes a single cross-file call edge observed in the chunk.
type CallGraphEntry struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Symbol string `json:"symbol"`
}

// CFGEntry is a before/after count pair for one control-flow category.
type CFGEntry struct {
	Before int `json:"before"`
	After  int `json:"after"`
}

// CFGSummary aggregates before/after counts per control-flow category. A null
// (unset) value means "not computed"; zero values mean "computed but none found".
type CFGSummary struct {
	Conditionals CFGEntry `json:"conditionals"`
	Loops        CFGEntry `json:"loops"`
	Returns      CFGEntry `json:"returns"`
	Errors       CFGEntry `json:"errors"`
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
	// BeforeSource holds the before-version source code per file path.
	// Populated by the annotation step for files whose extension maps to a
	// supported language in the LanguageCatalog (HasGrammar=true).
	// Used by the classifier for AST identity detection (rename/move).
	// Nil or empty map means no source available (fall through to next pillar).
	BeforeSource map[string]string `json:"before_source,omitempty"`
	// AfterSource holds the after-version source code per file path.
	// Populated by the annotation step for files whose extension maps to a
	// supported language in the LanguageCatalog (HasGrammar=true).
	// Used by the classifier for AST identity detection (rename/move).
	// Nil or empty map means no source available (fall through to next pillar).
	AfterSource map[string]string `json:"after_source,omitempty"`
	// CFGBefore holds the before-version CFG snapshot; nil means not computed.
	CFGBefore *CFGCount `json:"cfg_before,omitempty"`
	// CFGAfter holds the after-version CFG snapshot; nil means not computed.
	CFGAfter *CFGCount `json:"cfg_after,omitempty"`
	// AnnotatedEntries is the structured per-symbol annotation list. Additive to
	// AnnotatedDiff (kept for backward compat). Empty means not populated.
	AnnotatedEntries []AnnotatedEntry `json:"annotated_entries,omitempty"`
	// CallGraph is the list of cross-file call edges observed in the chunk.
	CallGraph []CallGraphEntry `json:"call_graph,omitempty"`
}
