package chunkers

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

// AnnotateDiffForRead takes a raw unified diff and a content provider,
// and returns an annotated string with semantic AST labels inline in @@ headers.
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
func AnnotateDiffForRead(rawDiff string, cp ports.ContentProvider) string {
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

	// Create a new UnifiedASTPass for annotation
	astPass := NewUnifiedASTPass(NewLanguageCatalog())

	// Build annotated output per file
	var sb strings.Builder

	for _, name := range filenames {
		fc, hasContent := contentMap[name]
		if !hasContent {
			continue // no content for this file → skip
		}

		// Check for category label first
		catLabel := astPass.categoryLabel(name)
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
		entry, ok := astPass.catalog.ExtensionToLanguage(ext)
		if !ok || !entry.HasGrammar {
			// No grammar → skip file entirely (no UNKNOWN_GENERIC in reading mode)
			continue
		}

		// Run ProcessWithContent to get semantic labels
		labels, _, procErr := astPass.ProcessWithContent(name, fc.Before, fc.After, nil)
		if procErr != nil || len(labels) == 0 {
			continue // per-file: skip on failure, continue others
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
// If no label falls within the hunk range, falls back to closest by line distance.
func findBestLabel(frag *gitdiff.TextFragment, labels []domain.Label) *domain.Label {
	if len(labels) == 0 {
		return nil
	}

	hunkStart := int(frag.NewPosition)
	hunkEnd := hunkStart + int(frag.NewLines)

	// Priority order: higher = more significant for reader mode
	labelPriority := map[domain.LabelType]int{
		domain.NEW_FUNC:      10,
		domain.MOD_SIG:       9,
		domain.DELETED_FUNC:  8,
		domain.NEW_TYPE:      7,
		domain.MOD_TYPE:      6,
		domain.DELETED_TYPE:  5,
		domain.MOD_BODY:      4,
		domain.MOD_BODY_LOGIC:  3,
		domain.MOD_BODY_ERROR:  3,
		domain.MOD_BODY_REORDER: 2,
		domain.MOD_BODY_CALL:    1,
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

	// Fallback: find the label closest to the hunk start
	bestIdx := 0
	bestDist := abs(labels[0].Line - hunkStart)
	for i, l := range labels {
		dist := abs(l.Line - hunkStart)
		if dist < bestDist {
			bestDist = dist
			bestIdx = i
		}
	}
	return &labels[bestIdx]
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