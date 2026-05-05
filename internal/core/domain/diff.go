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
}
