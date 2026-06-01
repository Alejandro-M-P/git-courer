package shared

import (
	"path/filepath"
	"strings"
)

// FilterDiffByFile filters diff output to sections matching a given file pattern.
// It preserves diff structure by tracking which file section we're inside.
func FilterDiffByFile(diff string, fileFilter string) string {
	if diff == "" || fileFilter == "" {
		return diff
	}

	lines := strings.Split(diff, "\n")
	var sb strings.Builder
	inFile := false

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") || strings.HasPrefix(line, "index ") {
			if strings.Contains(line, fileFilter) || strings.Contains(filepath.Base(line), fileFilter) {
				inFile = true
				sb.WriteString(line)
				sb.WriteByte('\n')
			} else {
				inFile = false
			}
			continue
		}

		if inFile {
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}

	return sb.String()
}
