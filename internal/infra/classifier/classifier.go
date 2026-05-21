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
	gitProvider      ports.Git
	patternFreq      *PatternFrequency
	catalog          *chunkers.LanguageCatalog
	binaryClassifier ports.BinaryClassifier // nil = degrade MOD_BODY_CALL to "fix"
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

	totalLabels := len(labels)

	// -------------------------------------------------------------------------
	// 0. TEST-ONLY CHUNKS: if all files are test files, classify as test.
	//     Must run BEFORE weight-based logic so NEW_FUNC in test files
	//     doesn't produce feat instead of test.
	// -------------------------------------------------------------------------
	if c.catalog != nil {
		allTest := len(files) > 0
		hasNonTest := false
		for _, f := range files {
			if !c.catalog.IsTestFile(f) {
				hasNonTest = true
				break
			}
		}
		if allTest && !hasNonTest {
			if len(labels) == 0 {
				return "test", lowConfidence
			}
			// Use the highest-weight label type for confidence calculation
			winnerType, _ := weightWinner(counts)
			return "test", confidenceForPurity(counts, winnerType, totalLabels)
		}
	} else {
		// Fallback: filename-based test detection when no catalog is available
		allTest := len(files) > 0
		hasNonTest := false
		for _, f := range files {
			if !strings.Contains(f, "_test.") &&
				!strings.Contains(f, ".test.") &&
				!strings.Contains(f, "test_") {
				hasNonTest = true
				break
			}
		}
		if allTest && !hasNonTest {
			if len(labels) == 0 {
				return "test", lowConfidence
			}
			winnerType, _ := weightWinner(counts)
			return "test", confidenceForPurity(counts, winnerType, totalLabels)
		}
	}

	// -------------------------------------------------------------------------
	// 1. WEIGHT-BASED SELECTION: find the label type with highest weight.
	//    Ties broken by label count. Breaking is orthogonal (adds "!").
	//    The Fuerza table defines the priority:
	//      9 = feat (NEW_FUNC/NEW_TYPE)
	//      8 = fix (MOD_BODY_LOGIC, MOD_BODY_ERROR, MOD_SIG)
	//      7 = refactor (MOD_BODY_REORDER, DELETED_FUNC/T, MOD_TYPE)
	//      6 = chore/ci/docs/delegate (CONFIG, DEPS, CI, DOCS, MOD_BODY_CALL)
	//      5 = test (TEST)
	//      4 = refactor-low (UNKNOWN_GENERIC)
	// -------------------------------------------------------------------------
	winnerType, _ := weightWinner(counts)

	// Handle special cases that bypass weight selection

	// Breaking deletion override: when BREAKING and DELETED_* labels exist,
	// ensure the winner reflects deletion semantics.
	if hasBreaking {
		hasDeleted := counts["DELETED_FUNC"] > 0 || counts["DELETED_TYPE"] > 0
		if hasDeleted {
			// If NEW_FUNC/NEW_TYPE present, feat wins (weight 9 > 7)
			// but breaking suffix applies → "feat!"
			// Otherwise, DELETED_* at weight 7 may tie with other weight-7 labels;
			// prefer DELETED_FUNC as the winner type for confidence calculation.
			if counts["NEW_FUNC"] == 0 && counts["NEW_TYPE"] == 0 {
				if counts["DELETED_FUNC"] > 0 {
					winnerType = "DELETED_FUNC"
				} else {
					winnerType = "DELETED_TYPE"
				}
			}
		}
	}

	// Map winning label to commit type
	commitType, weight := LabelWeight(winnerType)

	// Handle MOD_BODY_CALL delegate case
	if commitType == "" && weight == 6 {
		if c.binaryClassifier != nil {
			result, err := c.binaryClassifier.ClassifyBinary(diff)
			if err == nil && (result == "fix" || result == "refactor") {
				if hasBreaking {
					return result + "!", 0.97
				}
				return result, 0.97
			}
		}
		// Degraded — no BinaryClassifier or invalid response
		if hasBreaking {
			return "fix!", lowConfidence
		}
		return "fix", lowConfidence
	}

	// -------------------------------------------------------------------------
	// 2. POST-WEIGHT REFINEMENTS: AST identity and operator mutation can
	//     override "fix" to "refactor" for MOD_BODY/MOD_SIG labels.
	//     These detect that a change is actually a rename/move (refactor)
	//     rather than a bug fix, even though the label says MOD_BODY.
	// -------------------------------------------------------------------------
	if commitType == "fix" {
		// AST identity: detect refactor by function rename/move
		if result, conf := c.detectRefactorByASTHash(files, goBefore, goAfter); result != "" {
			if hasBreaking {
				return result + "!", conf
			}
			return result, conf
		}

		// Operator mutation: detect fix from operator change
		if result, conf := detectOperatorMutation(diff); result != "" {
			if hasBreaking {
				return result + "!", conf
			}
			return result, conf
		}

		// Code-test symmetry: paired code+test = fix with high confidence
		// Only applies when the weight winner is fix-relevant (MOD_BODY/MOD_SIG)
		// and NOT when NEW_FUNC/NEW_TYPE is present (feat always wins).
		if hasNewFuncCount := counts["NEW_FUNC"] + counts["NEW_TYPE"]; hasNewFuncCount == 0 {
			if commitTypeSym, confSym := c.detectCodeTestSymmetry(files); commitTypeSym != "" {
				return commitTypeSym, confSym
			}
		}
	}

	// Handle empty/zero-weight labels
	if commitType == "" {
		// Code-Test Symmetry check for mixed/unknown labels
		if commitTypeSym, confSym := c.detectCodeTestSymmetry(files); commitTypeSym != "" {
			return commitTypeSym, confSym
		}

		// Test file detection
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
			return "test", lowConfidence
		}

		// Fallback
		if hasBreaking {
			return "", lowConfidence
		}
		return "", lowConfidence
	}

	// Apply breaking suffix — orthogonal to type selection
	if hasBreaking {
		if !strings.HasSuffix(commitType, "!") {
			commitType += "!"
		}
	}

	// Confidence based on purity of the winning label
	confidence := confidenceForPurity(counts, winnerType, totalLabels)

	return commitType, confidence
}

// weightWinner finds the label type with the highest weight.
// Ties are broken by label count (more labels of same weight wins).
func weightWinner(counts map[string]int) (string, int) {
	bestType := ""
	bestWeight := -1
	bestCount := 0

	for typ, cnt := range counts {
		_, w := LabelWeight(typ)
		if w > bestWeight || (w == bestWeight && cnt > bestCount) {
			bestWeight = w
			bestType = typ
			bestCount = cnt
		}
	}

	return bestType, bestWeight
}

// LabelWeight maps AST annotation labels to commit types and priority weights
// using the Fuerza classification table.
//
// Weight levels and their semantic meaning:
//   9 = feat      — new functionality (NEW_FUNC, NEW_TYPE)
//   8 = fix       — bug fixes, error handling, signature changes (MOD_BODY_LOGIC, MOD_BODY_ERROR, MOD_SIG)
//   7 = refactor  — structural changes without behavior change (DELETED_FUNC/T, MOD_TYPE, MOD_BODY_REORDER, MOD_BODY_CALL)
//   6 = chore/ci/docs — configuration, dependencies, CI, documentation (CONFIG, DEPS, CI, DOCS)
//   5 = test      — test-only changes (TEST)
//   4 = refactor-low — unknown changes with low confidence (UNKNOWN_GENERIC)
//
// Breaking ("!") is orthogonal to weight — it's appended to the winner type
// if any label in the chunk carries the ⚠ BREAKING marker.
//
// Tie-breaking: when labels have equal weight, count wins (more labels of same
// type). If still tied, LabelWeight returns the first match in the switch.
func LabelWeight(labelType string) (commitType string, weight int) {
	switch labelType {
	case "NEW_FUNC", "NEW_TYPE":
		return "feat", 9
	case "MOD_BODY_LOGIC", "MOD_BODY_ERROR":
		return "fix", 8
	case "MOD_BODY_REORDER":
		return "refactor", 7
	case "MOD_BODY_CALL":
		return "fix", 7 // more significant than CONFIG/DEPS — behavioral change
	case "DELETED_FUNC", "DELETED_TYPE":
		return "refactor", 7
	case "MOD_SIG":
		return "fix", 8
	case "MOD_TYPE":
		return "refactor", 7
	case "CONFIG", "DEPS":
		return "chore", 6
	case "CI":
		return "ci", 6
	case "DOCS":
		return "docs", 6
	case "TEST":
		return "test", 5
	case "UNKNOWN_GENERIC":
		return "refactor", 4
	default:
		if strings.HasPrefix(labelType, "MOD_BODY") {
			return "fix", 8
		}
		return "", 0
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

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
