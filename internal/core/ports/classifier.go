package ports

import "github.com/Alejandro-M-P/git-courer/internal/core/domain"

// MessageClassifier analyzes AST annotations to pre-classify commit types
// with confidence scoring, reducing LLM token usage for unambiguous changes.
type MessageClassifier interface {
	// Classify analyzes chunk.AnnotatedDiff and sets chunk.CommitType and
	// chunk.ConfidenceScore based on pattern matching.
	// Returns (commitType, confidence) — also accessible via chunk fields.
	Classify(chunk *domain.DiffChunk) (commitType string, confidence float64)

	// LearnFromHistory analyzes recent git log to improve pattern recognition.
	LearnFromHistory() error
}

// BinaryClassifier delegates binary classification decisions (e.g., fix vs refactor)
// to an external service like an LLM. This follows the narrow-interface escape pattern
// to avoid coupling the classifier to a fat LLM interface.
type BinaryClassifier interface {
	ClassifyBinary(prompt string) (string, error)
}
