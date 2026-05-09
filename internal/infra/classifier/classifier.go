package classifier

import (
	"regexp"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/infra/chunkers"
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
// on AST annotations, optionally boosted by historical commit frequency.
type Classifier struct {
	gitProvider ports.Git
	patternFreq *PatternFrequency
	catalog     *chunkers.LanguageCatalog
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
	commitType, confidence := c.determineType(labels, chunk.Files, chunk.GoBefore, chunk.GoAfter, chunk.Diff)

	if c.patternFreq != nil && commitType != "" {
		if boost := c.patternFreq.ConfidenceBoost(commitType); boost > 0 {
			confidence = min(confidence+boost, 1.0)
		}
	}

	chunk.CommitType = commitType
	chunk.ConfidenceScore = confidence
	return commitType, confidence
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

func (c *Classifier) determineType(labels []labelInfo, files []string, goBefore, goAfter map[string]string, diff string) (string, float64) {
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

	dominant := dominantCategory(counts)
	totalLabels := len(labels)

	// -------------------------------------------------------------------------
	// 1.5 BUG #7 FIX: Breaking deletions override dominance.
	// When hasBreaking is true and any DELETED_* label exists, override the
	// dominant category to the most specific DELETED label. This prevents
	// MOD_BODY from masking a breaking deletion (e.g., DELETED_FUNC ⚠BREAKING
	// alongside MOD_BODY should classify as a deletion, not empty/low-conf).
	// -------------------------------------------------------------------------
	if hasBreaking {
		for _, l := range labels {
			if l.Breaking && (l.Type == "DELETED_FUNC" || l.Type == "DELETED_TYPE") {
				// Override dominant to the first breaking DELETED label found.
				// If multiple breaking deletions exist, the most frequent one wins
				// via dominantCategory logic, so resolve ties by priority.
				if counts["DELETED_FUNC"] > 0 && counts["DELETED_TYPE"] > 0 {
					// Both DELETED types present — prefer DELETED_FUNC (higher priority)
					dominant = "DELETED_FUNC"
				} else if counts["DELETED_FUNC"] > 0 {
					dominant = "DELETED_FUNC"
				} else {
					dominant = "DELETED_TYPE"
				}
				break
			}
		}
	}

	// -------------------------------------------------------------------------
	// 1. UNAMBIGUOUS CASES — labels with single clear meaning, no pillar needed
	// -------------------------------------------------------------------------
	switch dominant {
	case "CONFIG", "DEPS":
		return "chore", highConfidence
	case "CI":
		return "ci", highConfidence
	case "DOCS":
		return "docs", highConfidence
	case "DELETED_FUNC", "DELETED_TYPE":
		suffix := ""
		if hasBreaking {
			suffix = "!"
		}
		return "refactor" + suffix, confidenceForPurity(counts, dominant, totalLabels)
	case "MOD_TYPE":
		return "refactor", confidenceForPurity(counts, dominant, totalLabels)
	case "MOD_SIG":
		// MOD_SIG is breaking only when hasBreaking is true (public API change).
		// Private (unexported) signature changes are MOD_SIG but NOT breaking.
		suffix := ""
		if hasBreaking {
			suffix = "!"
		}
		if hasNewFunc(counts) || hasNewType(counts) {
			return "feat" + suffix, confidenceForPurity(counts, dominant, totalLabels)
		}
		return "fix" + suffix, confidenceForPurity(counts, dominant, totalLabels)
	}

	// -------------------------------------------------------------------------
	// 2. PILAR 1 — Code-Test Symmetry: paired code+test = fix
	//     Must run BEFORE test file detection so symmetry takes precedence.
	// -------------------------------------------------------------------------
	if commitType, confidence := c.detectCodeTestSymmetry(files); commitType != "" {
		return commitType, confidence
	}

	// -------------------------------------------------------------------------
	// 3. Test file detection — test files dominate classification
	// -------------------------------------------------------------------------
	hasTestFiles := false
	if c.catalog != nil {
		for _, f := range files {
			if c.catalog.IsTestFile(f) {
				hasTestFiles = true
				break
			}
		}
	} else {
		for _, f := range files {
			if strings.Contains(f, "_test.") ||
				strings.Contains(f, ".test.") ||
				strings.Contains(f, "test_") {
				hasTestFiles = true
				break
			}
		}
	}
	if hasTestFiles {
		confidence := confidenceForPurity(counts, dominant, totalLabels)
		if purityRatio(counts, dominant, totalLabels) < 0.70 {
			return "", lowConfidence
		}
		return "test", confidence
	}

	// -------------------------------------------------------------------------
	// 4. PILAR 3 — AST Identity: detect refactor by function rename/move
	//    Only for MOD_BODY and non-breaking MOD_SIG (ambiguous modifications).
	//    Requires GoBefore/GoAfter source content. Falls through gracefully
	//    if source content is unavailable.
	// -------------------------------------------------------------------------
	if dominant == "MOD_BODY" || dominant == "MOD_SIG" {
		if result, confidence := c.detectRefactorByASTHash(files, goBefore, goAfter); result != "" {
			return result, confidence
		}
	}

	// -------------------------------------------------------------------------
	// 5. PILAR 2 — Operator Mutation: operator change = fix
	// -------------------------------------------------------------------------
	if dominant == "MOD_BODY" || dominant == "MOD_SIG" {
		if commitType, confidence := detectOperatorMutation(diff); commitType != "" {
			return commitType, confidence
		}
	}

	// -------------------------------------------------------------------------
	// 6. NEW_FUNC/NEW_TYPE — clear feat signal AFTER pillar checks
	// -------------------------------------------------------------------------
	switch dominant {
	case "NEW_FUNC", "NEW_TYPE":
		confidence := confidenceForPurity(counts, dominant, totalLabels)
		if hasBreaking {
			return "feat!", confidence
		}
		return "feat", confidence
	case "TEST":
		return "test", confidenceForPurity(counts, dominant, totalLabels)
	}

	// -------------------------------------------------------------------------
	// 6. Mixed check: if no single category dominates, return empty
	// -------------------------------------------------------------------------
	if purityRatio(counts, dominant, totalLabels) < 0.70 {
		return "", lowConfidence
	}

	// -------------------------------------------------------------------------
	// 7. FALLBACK for MOD_BODY/MOD_SIG without pillar resolution
	// -------------------------------------------------------------------------
	switch dominant {
	case "MOD_BODY", "MOD_SIG":
		return "fix", confidenceForPurity(counts, dominant, totalLabels)
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

// ---------------------------------------------------------------------------
// detectCodeTestSymmetry checks if a chunk contains paired code and test files
// and returns fix classification with high confidence if symmetry is detected.
// ---------------------------------------------------------------------------

func (c *Classifier) detectCodeTestSymmetry(files []string) (string, float64) {
	if c.catalog == nil || len(files) < 2 {
		return "", 0.0
	}

	// Group files into code files and test files
	var codeFiles, testFiles []string
	for _, file := range files {
		if c.catalog.IsTestFile(file) {
			testFiles = append(testFiles, file)
		} else {
			codeFiles = append(codeFiles, file)
		}
	}

	// If we have exactly one code file and one test file, check if they're paired
	if len(codeFiles) == 1 && len(testFiles) == 1 {
		if c.catalog.ArePaired(codeFiles[0], testFiles[0]) {
			// High confidence fix classification for code-test symmetry
			return "fix", 0.99
		}
	}

	return "", 0.0
}

// hasNewFunc devuelve true si hay al menos un label NEW_FUNC
func hasNewFunc(counts map[string]int) bool {
	return counts["NEW_FUNC"] > 0
}

// hasNewType devuelve true si hay al menos un label NEW_TYPE
func hasNewType(counts map[string]int) bool {
	return counts["NEW_TYPE"] > 0
}
