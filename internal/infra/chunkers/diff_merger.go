package chunkers

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

type parsedLabel struct {
	Name     string
	Type     string
	File     string
	Line     int
	Breaking bool
}

type fileLabelGroup struct {
	filename string
	labels   []parsedLabel
}

type hunkData struct {
	oldStart int
	oldLines int
	newStart int
	newLines int
	lines    []hunkLine
}

type hunkLine struct {
	op      gitdiff.LineOp
	content string
}

var (
	labelLineRe   = regexp.MustCompile(`^(\S+)\s+\[(\w+)(.*?)\]\s+(\S+):(\d+)$`)
	singleLabelRe = regexp.MustCompile(`^(\S+)\s+\[(\w+)\]\s+(\S+)$`)
)

func MergeDiffIntoAnnotations(chunk *domain.DiffChunk, rawDiff string) {
	if chunk.AnnotatedDiff == "" || rawDiff == "" {
		return
	}

	fileHunks := parseDiffHunks(rawDiff)
	if len(fileHunks) == 0 {
		return
	}

	groups := parseLabelGroups(chunk.AnnotatedDiff)
	if len(groups) == 0 {
		return
	}

	result := rebuildAnnotatedDiff(groups, fileHunks)
	if result != "" {
		chunk.AnnotatedDiff = result
	}
}

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

func parseLabelGroups(annotatedDiff string) []fileLabelGroup {
	lines := strings.Split(annotatedDiff, "\n")
	var groups []fileLabelGroup
	var current *fileLabelGroup

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "📄 ") {
			if current != nil {
				groups = append(groups, *current)
			}
			filename := strings.TrimPrefix(line, "📄 ")
			current = &fileLabelGroup{filename: filename}
			continue
		}
		if current != nil {
			if label := parseLabelLine(line); label != nil {
				current.labels = append(current.labels, *label)
			}
		}
	}
	if current != nil {
		groups = append(groups, *current)
	}
	return groups
}

func parseLabelLine(line string) *parsedLabel {
	if m := labelLineRe.FindStringSubmatch(line); m != nil {
		lineNum, _ := strconv.Atoi(m[5])
		breaking := strings.Contains(m[3], "BREAKING")
		return &parsedLabel{
			Name:     m[1],
			Type:     m[2],
			File:     m[4],
			Line:     lineNum,
			Breaking: breaking,
		}
	}
	if m := singleLabelRe.FindStringSubmatch(line); m != nil {
		return &parsedLabel{
			Name: m[1],
			Type: m[2],
			File: m[3],
		}
	}
	return nil
}

func rebuildAnnotatedDiff(groups []fileLabelGroup, fileHunks map[string][]hunkData) string {
	var b strings.Builder

	for gi, group := range groups {
		if gi > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("📄 " + group.filename + "\n")

		sorted := sortLabels(group.labels)

		for i, label := range sorted {
			nextLine := 0
			if label.Type != "DELETED_FUNC" && label.Type != "DELETED_TYPE" {
				nextLine = nextNonDeletedLine(sorted, i)
			}

			lines := hunkLinesForLabel(label, hunksFor(group.filename, fileHunks), nextLine)
			if len(lines) == 0 {
				continue
			}

			b.WriteByte('\n')
			b.WriteString("[" + label.Type)
			if label.Breaking {
				b.WriteString(" ⚠BREAKING")
			}
			b.WriteString("]\n")

			for _, l := range lines {
				b.WriteString(string(l.op.String()) + l.content + "\n")
			}
		}
	}

	return b.String()
}

func hunksFor(filename string, fileHunks map[string][]hunkData) []hunkData {
	if h, ok := fileHunks[filename]; ok {
		return h
	}
	return nil
}

func labelPriority(t string) int {
	if strings.HasPrefix(t, "MOD_") {
		return 0
	}
	if strings.HasPrefix(t, "NEW_") {
		return 1
	}
	if strings.HasPrefix(t, "DELETED_") {
		return 2
	}
	return 3
}

func sortLabels(labels []parsedLabel) []parsedLabel {
	sorted := make([]parsedLabel, len(labels))
	copy(sorted, labels)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			pi, pj := labelPriority(sorted[i].Type), labelPriority(sorted[j].Type)
			if pi > pj || (pi == pj && sorted[j].Line > 0 && sorted[i].Line > sorted[j].Line) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	return sorted
}

func nextNonDeletedLine(sorted []parsedLabel, fromIdx int) int {
	for j := fromIdx + 1; j < len(sorted); j++ {
		t := sorted[j].Type
		if t != "DELETED_FUNC" && t != "DELETED_TYPE" && sorted[j].Line > 0 {
			return sorted[j].Line
		}
	}
	return 0
}

func hunkLinesForLabel(label parsedLabel, hunks []hunkData, nextLine int) []hunkLine {
	var raw []hunkLine

	for _, h := range hunks {
		start, end := labelWindow(label, h, nextLine)
		if start == 0 && end == 0 {
			continue
		}

		newPos, oldPos := h.newStart, h.oldStart
		for _, l := range h.lines {
			if lineInWindow(label.Type, l.op, newPos, oldPos, start, end) {
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

func labelWindow(label parsedLabel, h hunkData, nextLine int) (int, int) {
	if label.Line == 0 {
		return h.newStart, h.newStart + h.newLines + 5
	}

	hardEnd := h.newStart + h.newLines + 5
	if label.Type == "DELETED_FUNC" || label.Type == "DELETED_TYPE" {
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
	case "NEW_FUNC":
		return label.Line - 1, end
	case "NEW_TYPE":
		return label.Line - 1, label.Line + 5
	case "DELETED_FUNC":
		return label.Line - 1, end
	case "DELETED_TYPE":
		return label.Line - 1, label.Line + 5
	case "MOD_SIG":
		return label.Line - 1, label.Line + 2
	default:
		return label.Line - 2, end
	}
}

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
