package ports

import "github.com/Alejandro-M-P/git-courer/internal/core/domain"

// CommitTypeHelper provides commit type inference and weight calculation.
// This is a port for the pure domain functions that were previously called
// directly from the infra/classifier package.
type CommitTypeHelper interface {
	// InferCommitType infers a conventional commit type from chunk content
	// when the classifier returns an empty CommitType.
	InferCommitType(chunk domain.DiffChunk) string

	// CommitTypeWeight returns the priority weight for a commit type string.
	// Higher weights indicate higher priority (feat=9, fix=8, refactor=7, etc.).
	CommitTypeWeight(commitType string) int
}
