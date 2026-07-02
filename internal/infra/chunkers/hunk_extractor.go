package chunkers

import (
	"strings"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

// hunkData captures a single text fragment's header and lines from a unified diff.
type hunkData struct {
	oldStart int
	oldLines int
	newStart int
	newLines int
	lines    []hunkLine
}

// hunkLine is a single line within a hunk, with its diff operation.
type hunkLine struct {
	op      gitdiff.LineOp
	content string
}

// parseDiffHunks parses a unified diff into per-file hunk data.
func parseDiffHunks(diff string) map[string][]hunkData {
	files, _, err := gitdiff.Parse(strings.NewReader(diff))
	if err != nil {
		return nil
	}

	result := make(map[string][]hunkData)
	for _, f := range files {
		name := f.NewName
		if name == "" {
			name = f.OldName
		}
		if name == "" || f.IsBinary {
			continue
		}

		for _, frag := range f.TextFragments {
			h := hunkData{
				oldStart: int(frag.OldPosition),
				oldLines: int(frag.OldLines),
				newStart: int(frag.NewPosition),
				newLines: int(frag.NewLines),
			}
			for _, l := range frag.Lines {
				h.lines = append(h.lines, hunkLine{
					op:      l.Op,
					content: strings.TrimRight(l.Line, "\n\r"),
				})
			}
			result[name] = append(result[name], h)
		}
	}
	return result
}

// hunksFor returns the hunk data for a given filename, or nil if absent.
func hunksFor(filename string, fileHunks map[string][]hunkData) []hunkData {
	if h, ok := fileHunks[filename]; ok {
		return h
	}
	return nil
}

// hunkLinesForLabel returns the diff hunk lines belonging to a single labeled
// symbol, using the symbol's line and the next non-deleted symbol's line to
// bound the window. The returned lines are trimmed of leading/trailing blank
// context.
func hunkLinesForLabel(label domain.Label, hunks []hunkData, nextLine int) []hunkLine {
	var raw []hunkLine

	for _, h := range hunks {
		start, end := labelWindow(label, h, nextLine)
		if start == 0 && end == 0 {
			continue
		}

		newPos, oldPos := h.newStart, h.oldStart
		for _, l := range h.lines {
			if lineInWindow(string(label.Type), l.op, newPos, oldPos, start, end) {
				raw = append(raw, l)
			}
			if l.op != gitdiff.OpAdd {
				oldPos++
			}
			if l.op != gitdiff.OpDelete {
				newPos++
			}
		}
	}

	return trimContext(raw)
}

// lineInWindow decides whether a hunk line at the given old/new position falls
// within the [start, end] window for the label type.
func lineInWindow(labelType string, op gitdiff.LineOp, newPos, oldPos, start, end int) bool {
	switch labelType {
	case "NEW_FUNC", "NEW_TYPE":
		if op == gitdiff.OpDelete {
			return false
		}
		return newPos >= start && newPos <= end
	case "DELETED_FUNC", "DELETED_TYPE":
		if op == gitdiff.OpAdd {
			return false
		}
		return oldPos >= start && oldPos <= end
	default:
		// MOD_* labels use after-file coordinates (newPos). Only deleted lines
		// lack a newPos — they reuse the previous line's position, which still
		// falls within the function's window.
		return newPos >= start && newPos <= end
	}
}

// labelWindow computes the inclusive [start, end] line window for a label within
// a hunk, taking the next non-deleted label's line as a soft upper bound.
func labelWindow(label domain.Label, h hunkData, nextLine int) (int, int) {
	if label.Line == 0 {
		return h.newStart, h.newStart + h.newLines + 5
	}

	hardEnd := h.newStart + h.newLines + 5
	if label.Type == domain.DELETED_FUNC || label.Type == domain.DELETED_TYPE {
		hardEnd = h.oldStart + h.oldLines + 5
	}

	softEnd := label.Line + 50
	if nextLine > 0 {
		softEnd = nextLine - 1
	}
	end := softEnd
	if hardEnd < end {
		end = hardEnd
	}

	switch label.Type {
	case domain.NEW_FUNC:
		return label.Line - 1, end
	case domain.NEW_TYPE:
		return label.Line - 1, label.Line + 5
	case domain.DELETED_FUNC:
		return label.Line - 1, end
	case domain.DELETED_TYPE:
		return label.Line - 1, label.Line + 5
	case domain.MOD_SIG:
		return label.Line - 1, label.Line + 2
	default:
		return label.Line - 2, end
	}
}

// trimContext narrows the hunk line slice to a small window around the first
// and last change lines, dropping blank context lines.
func trimContext(lines []hunkLine) []hunkLine {
	firstChange := -1
	lastChange := -1
	for i, l := range lines {
		if l.op != gitdiff.OpContext {
			if firstChange == -1 {
				firstChange = i
			}
			lastChange = i
		}
	}
	if firstChange == -1 {
		return nil
	}

	from := firstChange - 1
	if from < 0 {
		from = 0
	}
	to := lastChange + 2
	if to > len(lines) {
		to = len(lines)
	}

	filtered := lines[from:to]
	out := make([]hunkLine, 0, len(filtered))
	for _, l := range filtered {
		if l.op == gitdiff.OpContext && strings.TrimSpace(l.content) == "" {
			continue
		}
		out = append(out, l)
	}
	return out
}

// buildAnnotatedEntries converts semantic labels + parsed diff hunks into
// structured AnnotatedEntry records with hunk-only before/after per symbol.
// Labels are sorted per file (MOD_ first, then NEW_, then DELETED_) so the
// nextNonDeletedLine window computation is stable.
func buildAnnotatedEntries(labels []domain.Label, hunksByFile map[string][]hunkData) []domain.AnnotatedEntry {
	if len(labels) == 0 {
		return nil
	}

	// Group labels by file preserving order of first appearance.
	var fileOrder []string
	byFile := map[string][]domain.Label{}
	for _, l := range labels {
		if _, ok := byFile[l.File]; !ok {
			fileOrder = append(fileOrder, l.File)
		}
		byFile[l.File] = append(byFile[l.File], l)
	}

	var entries []domain.AnnotatedEntry
	for _, file := range fileOrder {
		sorted := sortDomainLabels(byFile[file])
		hunks := hunksFor(file, hunksByFile)

		for i, label := range sorted {
			nextLine := 0
			if label.Type != domain.DELETED_FUNC && label.Type != domain.DELETED_TYPE {
				nextLine = nextNonDeletedLineDomain(sorted, i)
			}
			lines := hunkLinesForLabel(label, hunks, nextLine)

			entry := domain.AnnotatedEntry{
				File:     label.File,
				Symbol:   label.Name,
				Type:     string(label.Type),
				Breaking: label.Breaking,
				Line:     label.Line,
			}
			entry.Before, entry.After = splitBeforeAfter(lines)
			entries = append(entries, entry)
		}
	}
	return entries
}

// splitBeforeAfter partitions hunk lines into before (deleted + context) and
// after (added + context) strings, preserving diff prefixes so the LLM sees
// the original unified-diff format.
func splitBeforeAfter(lines []hunkLine) (string, string) {
	var beforeB, afterB strings.Builder
	for _, l := range lines {
		rendered := l.op.String() + l.content + "\n"
		switch l.op {
		case gitdiff.OpAdd:
			afterB.WriteString(rendered)
		case gitdiff.OpDelete:
			beforeB.WriteString(rendered)
		case gitdiff.OpContext:
			beforeB.WriteString(rendered)
			afterB.WriteString(rendered)
		}
	}
	return strings.TrimRight(beforeB.String(), "\n"), strings.TrimRight(afterB.String(), "\n")
}

func sortDomainLabels(labels []domain.Label) []domain.Label {
	sorted := make([]domain.Label, len(labels))
	copy(sorted, labels)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			pi, pj := domainLabelPriority(sorted[i].Type), domainLabelPriority(sorted[j].Type)
			if pi > pj || (pi == pj && sorted[j].Line > 0 && sorted[i].Line > sorted[j].Line) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return sorted
}

func domainLabelPriority(t domain.LabelType) int {
	s := string(t)
	if strings.HasPrefix(s, "MOD_") {
		return 0
	}
	if strings.HasPrefix(s, "NEW_") {
		return 1
	}
	if strings.HasPrefix(s, "DELETED_") {
		return 2
	}
	return 3
}

func nextNonDeletedLineDomain(sorted []domain.Label, fromIdx int) int {
	for j := fromIdx + 1; j < len(sorted); j++ {
		t := sorted[j].Type
		if t != domain.DELETED_FUNC && t != domain.DELETED_TYPE && sorted[j].Line > 0 {
			return sorted[j].Line
		}
	}
	return 0
}