package classifier

import (
	"regexp"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// compile-time interface check
var _ ports.MessageClassifier = (*Classifier)(nil)

// confidence thresholds
const (
	highConfidence   = 0.95
	mediumConfidence = 0.85
	lowConfidence    = 0.65
	fallbackThreshold = 0.90
)

// Classifier implements ports.MessageClassifier with regex-based pattern matching
// on AST annotations.
type Classifier struct {
	gitProvider ports.Git
}

// labelInfo holds a single parsed label extracted from AnnotatedDiff.
type labelInfo struct {
	Type     string // e.g. "NEW_FUNC", "CONFIG", "DOCS"
	Breaking bool
}

// labelRegex matches AST labels in AnnotatedDiff: Name [LABEL_TYPE] file:line
// Optionally includes ⚠ BREAKING marker.
var labelRegex = regexp.MustCompile(`\[(\w+)(?:\s*⚠\s*BREAKING)?\]`)

// ---------------------------------------------------------------------------
// Classify — main entry point
// ---------------------------------------------------------------------------

// Classify analyzes chunk.AnnotatedDiff and sets chunk.CommitType and
// chunk.ConfidenceScore. Returns (commitType, confidence).
func (c *Classifier) Classify(chunk *domain.DiffChunk) (string, float64) {
	labels := parseLabels(chunk.AnnotatedDiff)

	commitType, confidence := determineType(labels, chunk.Files)

	chunk.CommitType = commitType
	chunk.ConfidenceScore = confidence

	return commitType, confidence
}

// LearnFromHistory analyzes git history to improve pattern recognition.
// PR 2 implementation. For now returns nil.
func (c *Classifier) LearnFromHistory() error {
	return nil
}

// ---------------------------------------------------------------------------
// parseLabels extracts label type information from AnnotatedDiff via regex.
// ---------------------------------------------------------------------------

func parseLabels(annotatedDiff string) []labelInfo {
	if annotatedDiff == "" {
		return nil
	}

	matches := labelRegex.FindAllStringSubmatch(annotatedDiff, -1)
	if len(matches) == 0 {
		return nil
	}

	labels := make([]labelInfo, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		labelType := m[1]

		// Check if the full match contains BREAKING marker
		breaking := strings.Contains(m[0], "BREAKING")

		labels = append(labels, labelInfo{
			Type:     labelType,
			Breaking: breaking,
		})
	}

	return labels
}

// ---------------------------------------------------------------------------
// determineType maps parsed labels to commit types with confidence scoring.
// ---------------------------------------------------------------------------

func determineType(labels []labelInfo, files []string) (string, float64) {
	if len(labels) == 0 {
		return "", 0.0
	}

	// Count label types
	counts := make(map[string]int)
	for _, l := range labels {
		counts[l.Type]++
	}

	hasBreaking := false
	for _, l := range labels {
		if l.Breaking {
			hasBreaking = true
			break
		}
	}

	// Check for test files in the chunk using generic patterns
	hasTestFiles := false
	for _, f := range files {
		if strings.Contains(f, "_test.") ||
			strings.Contains(f, ".test.") ||
			strings.Contains(f, "test_") {
			hasTestFiles = true
			break
		}
	}

	// Determine the dominant category
	dominant := dominantCategory(counts)
	totalLabels := len(labels)

	// If test files are present, the commit is test-oriented regardless of labels.
	if hasTestFiles {
		// Purity check: if all labels are consistent with a single type AND
		// there are test files, still classify as test.
		confidence := confidenceForPurity(counts, dominant, totalLabels)
		if purityRatio(counts, dominant, totalLabels) < 0.70 {
			// Mixed labels + test files → ambiguous, return empty
			return "", lowConfidence
		}
		// Test files dominate → test
		return "test", confidence
	}

	// Mixed check: if no single category dominates, return empty with low confidence.
	if purityRatio(counts, dominant, totalLabels) < 0.70 {
		return "", lowConfidence
	}

	// Map category → commit type with confidence
	switch dominant {
	case "NEW_FUNC", "NEW_TYPE":
		// New function or type → feat
		confidence := confidenceForPurity(counts, dominant, totalLabels)
		if hasBreaking {
			return "feat!", confidence
		}
		return "feat", confidence

	case "MOD_BODY", "MOD_SIG", "DELETED_FUNC", "DELETED_TYPE":
		// Modifications/deletions → fix or refactor
		if dominant == "MOD_SIG" && hasBreaking {
			return "fix!", confidenceForPurity(counts, dominant, totalLabels)
		}
		if dominant == "DELETED_FUNC" || dominant == "DELETED_TYPE" {
			return "refactor", confidenceForPurity(counts, dominant, totalLabels)
		}
		return "fix", confidenceForPurity(counts, dominant, totalLabels)

	case "MOD_TYPE":
		// Type modification → refactor
		return "refactor", confidenceForPurity(counts, dominant, totalLabels)

	case "CONFIG", "DEPS":
		// Configuration or dependency changes → chore
		return "chore", highConfidence

	case "CI":
		// CI/CD changes → ci
		return "ci", highConfidence

	case "DOCS":
		// Documentation changes → docs
		return "docs", highConfidence

	case "TEST":
		// Test file changes → test
		return "test", confidenceForPurity(counts, dominant, totalLabels)

	default:
		return "", lowConfidence
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// dominantCategory returns the label type with the highest count.
// Ties are broken by preferring the "more specific" type (NEW_FUNC > MOD_BODY > CONFIG).
func dominantCategory(counts map[string]int) string {
	if len(counts) == 0 {
		return ""
	}

	maxCount := 0
	var dominant string
	for typ, cnt := range counts {
		if cnt > maxCount || (cnt == maxCount && labelPriority(typ) > labelPriority(dominant)) {
			maxCount = cnt
			dominant = typ
		}
	}

	// If the dominant type only appears once and there are other diverse types
	// with the same count, it's truly mixed.
	// But dominantCategory returns the most frequent → determineType caller
	// checks purity via confidenceForPurity.

	return dominant
}

// confidenceForPurity calculates confidence based on label purity.
// Pure chunks (100% one label type) get high confidence.
// Mixed chunks get proportionally lower confidence.
func confidenceForPurity(counts map[string]int, dominant string, total int) float64 {
	if total == 0 {
		return 0.0
	}

	pureCount := counts[dominant]
	purity := float64(pureCount) / float64(total)

	if purity >= 1.0 {
		return highConfidence
	}
	if purity >= 0.85 {
		return highConfidence * 0.97
	}
	if purity >= 0.70 {
		return mediumConfidence
	}
	return lowConfidence
}

// purityRatio returns the fraction of labels belonging to the dominant type.
func purityRatio(counts map[string]int, dominant string, total int) float64 {
	if total == 0 {
		return 0.0
	}
	return float64(counts[dominant]) / float64(total)
}

// labelPriority assigns a priority score for tie-breaking.
// Higher = more significant.
func labelPriority(typ string) int {
	switch typ {
	case "NEW_FUNC", "NEW_TYPE":
		return 5
	case "MOD_SIG":
		return 4
	case "MOD_BODY", "MOD_TYPE":
		return 3
	case "DELETED_FUNC", "DELETED_TYPE":
		return 2
	default:
		return 1
	}
}
