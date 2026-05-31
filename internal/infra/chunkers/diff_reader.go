package chunkers

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

// maxLabelDistance is the maximum number of lines a label can be from a
// hunk to be considered as a proximity fallback when no in-range label exists.
const maxLabelDistance = 3

// renamedSimilarityThreshold is the minimum Levenshtein similarity ratio
// for two entity names to be classified as a rename rather than delete+new.
const renamedSimilarityThreshold = 0.6

// AnnotateDiffForReadStandalone is a convenience wrapper for AnnotateDiffForRead
// that creates its own catalog and computes labels internally. This is intended
// for callers that don't have pre-computed labels (e.g., the MCP diff tool).
// For the adapter code path, use AnnotateDiffForRead with pre-computed labels
// to avoid double parsing.
func AnnotateDiffForReadStandalone(rawDiff string, cp ports.ContentProvider) string {
	if rawDiff == "" || cp == nil {
		return ""
	}

	catalog := NewLanguageCatalog()
	pass := NewUnifiedASTPass(catalog)

	// Extract filenames from raw diff and compute labels
	files, _, err := gitdiff.Parse(strings.NewReader(rawDiff))
	if err != nil {
		return ""
	}

	var filenames []string
	for _, f := range files {
		name := f.NewName
		if name == "" {
			name = f.OldName
		}
		if name != "" && !f.IsBinary {
			filenames = append(filenames, name)
		}
	}

	labelsMap := make(map[string][]domain.Label)
	if len(filenames) > 0 {
		if contents, err := cp.GetContents(filenames); err == nil {
			for _, fc := range contents {
				labels, _, _ := pass.ProcessWithContent(fc.Filename, fc.Before, fc.After, nil)
				if len(labels) > 0 {
					labelsMap[fc.Filename] = labels
				}
			}
		}
	}

	return AnnotateDiffForRead(rawDiff, cp, labelsMap, catalog)
}

// AnnotateDiffForRead takes a raw unified diff, a content provider, and
// pre-computed labels per file, and returns an annotated string with semantic
// AST labels inline in @@ headers.
//
// The labelsMap parameter contains labels previously computed by ProcessWithContent
// keyed by filename. This ensures ProcessWithContent is called exactly once per file
// (by the adapter), not duplicated here.
//
// Format per file section:
//
//	filename
//	@@ L{start},{count} +{start},{count} @@ [TYPE: Name ⚠BREAKING]
//	+hunk lines...
//
// Category-only labels (DEPS, CONFIG, DOCS, CI) have no name suffix:
//
//	go.mod
//	@@ L1,3 +1,3 @@ [DEPS]
//
// Returns empty string on any error (parse failure, no grammar, content unavailable).
// Error boundary is per-file: if one file fails, others are still annotated.
func AnnotateDiffForRead(rawDiff string, cp ports.ContentProvider, labelsMap map[string][]domain.Label, catalog *LanguageCatalog) string {
	// Guard: nil or empty input
	if rawDiff == "" || cp == nil {
		return ""
	}

	// Parse the raw diff using go-gitdiff (same parser as existing diff.go)
	files, _, err := gitdiff.Parse(strings.NewReader(rawDiff))
	if err != nil {
		return "" // parse failure → graceful degradation
	}

	if len(files) == 0 {
		return ""
	}

	// Extract filenames from parsed diff, skipping binary files
	var filenames []string
	fileMap := make(map[string]*gitdiff.File)
	for _, f := range files {
		name := f.NewName
		if name == "" {
			name = f.OldName
		}
		if name == "" || f.IsBinary {
			continue
		}
		filenames = append(filenames, name)
		fileMap[name] = f
	}

	if len(filenames) == 0 {
		return ""
	}

	// Get file contents via ContentProvider
	contents, err := cp.GetContents(filenames)
	if err != nil {
		return "" // content unavailable → graceful degradation
	}

	// Build a lookup map for file contents
	contentMap := make(map[string]ports.FileContent)
	for _, fc := range contents {
		contentMap[fc.Filename] = fc
	}

	// Build annotated output per file
	var sb strings.Builder

	for _, name := range filenames {
		// Check for category label first (no ProcessWithContent needed)
		catLabel := categoryLabel(name)
		if catLabel != "" {
			// Category file: annotate with [CATEGORY] (no name suffix)
			sb.WriteString(name)
			sb.WriteByte('\n')
			file, hasFile := fileMap[name]
			if hasFile && len(file.TextFragments) > 0 {
				sb.WriteString(annotateCategoryHunks(file.TextFragments, catLabel))
			}
			continue
		}

		// Check if grammar exists for this file's language
		ext := filepath.Ext(name)
		entry, ok := catalog.ExtensionToLanguage(ext)
		var labels []domain.Label
		if ok && entry.HasGrammar {
			labels = labelsMap[name]
		} else {
			slog.Warn("no grammar available for file", "file", name, "reason", "no grammar")
		}

		// Build annotated output for this file
		sb.WriteString(name)
		sb.WriteByte('\n')

		file, hasFile := fileMap[name]
		if !hasFile || len(file.TextFragments) == 0 {
			continue
		}

		// Merge labels into hunks
		annotateHunksWithLabels(file.TextFragments, labels, &sb)
	}

	return sb.String()
}

// annotateCategoryHunks produces annotated hunk lines for a category file
// using the category label (e.g. DEPS, CONFIG, DOCS, CI) with no name suffix.
func annotateCategoryHunks(frags []*gitdiff.TextFragment, catLabel string) string {
	var sb strings.Builder
	for _, frag := range frags {
		sb.WriteString(formatAnnotatedHeader(frag, catLabel, ""))
		sb.WriteByte('\n')
		sb.WriteString(fragBody(frag))
	}
	return sb.String()
}

// formatAnnotatedHeader produces an @@ header with a label appended.
func formatAnnotatedHeader(frag *gitdiff.TextFragment, labelType, labelName string) string {
	return fmt.Sprintf("@@ -%d,%d +%d,%d @@ %s",
		frag.OldPosition, frag.OldLines,
		frag.NewPosition, frag.NewLines,
		formatLabel(labelType, labelName, false))
}

// formatLabel produces a label string like [NEW_FUNC: helper] or [DEPS].
// Category labels (DEPS, CONFIG, DOCS, CI) have no name suffix.
func formatLabel(labelType, labelName string, breaking bool) string {
	var sb strings.Builder
	sb.WriteByte('[')
	sb.WriteString(labelType)
	if labelName != "" {
		sb.WriteString(": ")
		sb.WriteString(labelName)
	}
	if breaking {
		sb.WriteString(" ⚠BREAKING")
	}
	sb.WriteByte(']')
	return sb.String()
}

// annotateHunksWithLabels merges semantic labels into diff hunks.
// Each hunk gets the label of the entity whose line is closest to the hunk's new start line.
func annotateHunksWithLabels(frags []*gitdiff.TextFragment, labels []domain.Label, sb *strings.Builder) {
	for _, frag := range frags {
		// Find the best matching label for this hunk
		bestLabel := findBestLabel(frag, labels)

		header := fmt.Sprintf("@@ -%d,%d +%d,%d @@",
			frag.OldPosition, frag.OldLines,
			frag.NewPosition, frag.NewLines)

		if bestLabel != nil {
			header += " " + formatLabel(string(bestLabel.Type), bestLabel.Name, bestLabel.Breaking)
		}
		sb.WriteString(header)
		sb.WriteByte('\n')

		// Write hunk body lines
		sb.WriteString(fragBody(frag))
	}
}

// findBestLabel finds the best label for a hunk.
// It prefers labels whose line falls within the hunk's new-side range,
// and among those, picks the most semantically significant one:
// NEW_FUNC > MOD_SIG > DELETED_FUNC > NEW_TYPE > MOD_TYPE > DELETED_TYPE > MOD_BODY variants.
// If no label falls within the hunk range, falls back to the closest label
// within maxLabelDistance lines. If no label is within that distance,
// returns a generic CHANGED label.
func findBestLabel(frag *gitdiff.TextFragment, labels []domain.Label) *domain.Label {
	hunkStart := int(frag.NewPosition)
	hunkEnd := hunkStart + int(frag.NewLines)

	if len(labels) == 0 {
		// No labels available — return generic CHANGED label
		return &domain.Label{
			Type: domain.CHANGED,
			Name: "",
			Line: hunkStart,
		}
	}

	// Priority order: higher = more significant for reader mode
	labelPriority := map[domain.LabelType]int{
		domain.NEW_FUNC:         10,
		domain.MOD_SIG:          9,
		domain.DELETED_FUNC:     8,
		domain.RENAMED_FUNC:     7,
		domain.NEW_TYPE:         6,
		domain.MOD_TYPE:         5,
		domain.DELETED_TYPE:     4,
		domain.RENAMED_TYPE:     4,
		domain.MOD_BODY:         3,
		domain.MOD_BODY_LOGIC:   2,
		domain.MOD_BODY_ERROR:   2,
		domain.MOD_BODY_REORDER: 1,
		domain.MOD_BODY_CALL:    0,
		domain.CHANGED:          0, // lowest priority — generic fallback label
	}

	// First: find labels within the hunk's new-side line range
	var inRange []int
	for i, l := range labels {
		if l.Line >= hunkStart && l.Line <= hunkEnd {
			inRange = append(inRange, i)
		}
	}

	if len(inRange) > 0 {
		// Pick the highest-priority label within range
		bestIdx := inRange[0]
		bestPri := labelPriority[labels[bestIdx].Type]
		for _, idx := range inRange {
			if pri, ok := labelPriority[labels[idx].Type]; ok && pri > bestPri {
				bestPri = pri
				bestIdx = idx
			}
		}
		return &labels[bestIdx]
	}

	// Fallback: find the closest label within maxLabelDistance lines.
	// Distance is measured from the nearest edge of the hunk range
	// (start or end), so labels just past the hunk boundary are included.
	bestIdx := -1
	bestDist := maxLabelDistance + 1 // sentinel: any valid distance beats this
	for i, l := range labels {
		// Distance from the nearest edge of the hunk
		dist := labelDistance(l.Line, hunkStart, hunkEnd)
		if dist <= maxLabelDistance && dist < bestDist {
			bestDist = dist
			bestIdx = i
		}
	}

	if bestIdx >= 0 {
		return &labels[bestIdx]
	}

	// No label within proximity — return generic CHANGED label
	return &domain.Label{
		Type: domain.CHANGED,
		Name: "",
		Line: hunkStart,
	}
}

// fragBody returns the body of a TextFragment (all lines after the @@ header).
func fragBody(frag *gitdiff.TextFragment) string {
	var sb strings.Builder
	for _, line := range frag.Lines {
		switch line.Op {
		case gitdiff.OpAdd:
			sb.WriteByte('+')
		case gitdiff.OpDelete:
			sb.WriteByte('-')
		default:
			sb.WriteByte(' ')
		}
		sb.WriteString(line.Line)
		sb.WriteByte('\n')
	}
	return sb.String()
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// labelDistance returns the distance from a label line to the nearest
// edge of a hunk range [hunkStart, hunkEnd]. If the label is within the
// range, the distance is 0.
func labelDistance(labelLine, hunkStart, hunkEnd int) int {
	if labelLine >= hunkStart && labelLine <= hunkEnd {
		return 0
	}
	if labelLine < hunkStart {
		return hunkStart - labelLine
	}
	return labelLine - hunkEnd
}
