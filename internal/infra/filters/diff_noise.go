package filters

import "strings"

// FilterDiffNoise removes structural git noise lines from a unified diff.
// It is a pure function with no side effects.
// Blacklisted prefixes (removed): "diff --git", "index ", "\"
// Preserved: "@@" hunk headers (needed by cloud LLMs for context).
// All other lines, including "--- ", "+++ ", "+", and "-", are preserved.
func FilterDiffNoise(diff string) string {
	if diff == "" {
		return ""
	}
	lines := strings.Split(diff, "\n")
	var out strings.Builder
	out.Grow(len(diff))
	first := true
	for _, line := range lines {
		if line == "" {
			if !first {
				out.WriteByte('\n')
			}
			first = false
			continue
		}
		skip := strings.HasPrefix(line, "diff --git") ||
			strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "\\")
		if skip {
			continue
		}
		if !first {
			out.WriteByte('\n')
		}
		first = false
		out.WriteString(line)
	}
	return out.String()
}
