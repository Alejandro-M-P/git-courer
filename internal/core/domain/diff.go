package domain

// DiffChunk represents a logical subset of a git diff,
// small enough to be processed by an LLM in a single context window.
type DiffChunk struct {
	// Files is the list of files included in this chunk.
	Files []string
	// Diff is the unified diff content for these files.
	Diff string
	// CommitType is the pre-decided type (feat, fix, refactor, docs, test, chore, ci) or empty.
	CommitType string
	// AnnotatedDiff is the diff grouped by file with semantic labels per function/type.
	AnnotatedDiff string
	// ConfidenceScore is the classifier's confidence (0.0–1.0) in the pre-classified CommitType.
	// 0.0 means no classification was performed.
	ConfidenceScore float64
	// Scope is the functional area resolved from project config areas (e.g. "security", "core").
	// Empty if no area matches or init hasn't been run.
	Scope string
	// GoBefore holds the before-version Go source code per file path.
	// Populated by the annotation step for .go files to enable AST identity detection.
	// Nil or empty map means no source available (fall through to next pillar).
	GoBefore map[string]string
	// GoAfter holds the after-version Go source code per file path.
	// Populated by the annotation step for .go files to enable AST identity detection.
	// Nil or empty map means no source available (fall through to next pillar).
	GoAfter map[string]string
}
