package mcp

import (
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/infra/filters"
)

// DiffResult contains sanitized diff output with pagination and filtering metadata.
type DiffResult struct {
	Diff              string `json:"diff"`
	TotalLines        int    `json:"total_lines"`
	LinesShown        int    `json:"lines_shown"`
	Offset            int    `json:"offset"`
	Truncated         bool   `json:"truncated"`
	NextOffset        int    `json:"next_offset,omitempty"`
	Filtered          bool   `json:"filtered"`
	NoiseLinesRemoved int    `json:"noise_lines_removed"`
	Mode              string `json:"mode,omitempty"`   // "working_tree" | "full_delta" | "divergence" | "all"
	Base              string `json:"base,omitempty"`
	Target            string `json:"target,omitempty"`
}

// SanitizeDiff applies noise filtering and pagination to a raw git diff string.
func SanitizeDiff(raw string, offset, limit int) DiffResult {
	if raw == "" {
		return DiffResult{
			Diff:              "",
			TotalLines:        0,
			LinesShown:        0,
			Offset:            offset,
			Truncated:         false,
			Filtered:          false,
			NoiseLinesRemoved: 0,
		}
	}

	// Count noise lines in the original diff
	originalLines := strings.Split(raw, "\n")
	noiseCount := 0
	for _, line := range originalLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "diff --git") ||
			strings.HasPrefix(trimmed, "index ") ||
			strings.HasPrefix(trimmed, "@@") ||
			strings.HasPrefix(trimmed, "\\") {
			noiseCount++
		}
	}

	// Apply noise filter
	filtered := filters.FilterDiffNoise(raw)
	filteredLines := strings.Split(filtered, "\n")

	// Remove a single trailing empty string caused by trailing newline in Split
	if len(filteredLines) > 0 && filteredLines[len(filteredLines)-1] == "" {
		filteredLines = filteredLines[:len(filteredLines)-1]
	}

	total := len(filteredLines)

	// Clamp offset
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}

	end := offset + limit
	if end > total {
		end = total
	}

	sliced := filteredLines[offset:end]
	truncated := end < total

	nextOffset := 0
	if truncated {
		nextOffset = end
	}

	return DiffResult{
		Diff:              strings.Join(sliced, "\n"),
		TotalLines:        total,
		LinesShown:        len(sliced),
		Offset:            offset,
		Truncated:         truncated,
		NextOffset:        nextOffset,
		Filtered:          noiseCount > 0,
		NoiseLinesRemoved: noiseCount,
	}
}

// LogResult contains paginated commit entries from git log output.
type LogResult struct {
	Commits      []CommitEntry `json:"commits"`
	TotalCommits int           `json:"total_commits"`
	Returned     int           `json:"returned"`
	Offset       int           `json:"offset"`
	Truncated    bool          `json:"truncated"`
	NextOffset   int           `json:"next_offset,omitempty"`
	Message      string        `json:"message,omitempty"`
}

// CommitEntry represents a single commit entry in structured form.
type CommitEntry struct {
	Hash    string `json:"hash"`
	Author  string `json:"author,omitempty"`
	Date    string `json:"date,omitempty"`
	Message string `json:"message"`
}

// SanitizeLog parses git log --oneline output and applies pagination.
func SanitizeLog(raw string, offset, limit int) LogResult {
	if raw == "" {
		return LogResult{
			Commits:      nil,
			TotalCommits: 0,
			Returned:     0,
			Offset:       offset,
			Truncated:    false,
		}
	}

	lines := strings.Split(raw, "\n")
	var entries []CommitEntry
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: hash|author|date|message
		parts := strings.SplitN(line, "|", 4)
		if len(parts) >= 4 {
			entries = append(entries, CommitEntry{Hash: parts[0], Author: parts[1], Date: parts[2], Message: parts[3]})
		} else if len(parts) >= 2 {
			// Fallback for old oneline format: hash message
			entries = append(entries, CommitEntry{Hash: parts[0], Message: parts[1]})
		} else {
			entries = append(entries, CommitEntry{Hash: line})
		}
	}

	total := len(entries)

	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}

	end := offset + limit
	if end > total {
		end = total
	}

	sliced := entries[offset:end]
	truncated := end < total

	nextOffset := 0
	if truncated {
		nextOffset = end
	}

	return LogResult{
		Commits:      sliced,
		TotalCommits: total,
		Returned:     len(sliced),
		Offset:       offset,
		Truncated:    truncated,
		NextOffset:   nextOffset,
	}
}

// SanitizeBranchList parses git branch -a output and returns clean branch names.
func SanitizeBranchList(raw string) []string {
	if raw == "" {
		return nil
	}

	lines := strings.Split(raw, "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Remove current branch marker
		if strings.HasPrefix(line, "* ") {
			line = line[2:]
		} else if strings.HasPrefix(line, "*") {
			line = line[1:]
		}
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

// SanitizeTagList returns the tag list as-is (already sanitized by the git adapter).
func SanitizeTagList(tags []string) []string {
	return tags
}
