package chunkers

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/blak0p/git-courer/internal/core/domain"
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

			lines := hunkLinesForLabel(toDomainLabel(label), hunksFor(group.filename, fileHunks), nextLine)
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

// toDomainLabel converts a parsedLabel (emoji-era parsed form) to a domain.Label
// so it can be passed to the extracted hunk helpers.
func toDomainLabel(l parsedLabel) domain.Label {
	return domain.Label{Name: l.Name, Type: domain.LabelType(l.Type), File: l.File, Line: l.Line, Breaking: l.Breaking}
}
