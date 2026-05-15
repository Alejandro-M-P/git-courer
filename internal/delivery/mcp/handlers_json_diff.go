package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/delivery/mcp/shared"
)

func diffResultJSON(res shared.DiffResult) string {
	resp, _ := json.Marshal(map[string]interface{}{
		"diff":                res.Diff,
		"total_lines":         res.TotalLines,
		"lines_shown":         res.LinesShown,
		"offset":              res.Offset,
		"truncated":           res.Truncated,
		"next_offset":         res.NextOffset,
		"filtered_file":       res.Filtered,
		"noise_lines_removed": res.NoiseLinesRemoved,
		"mode":                res.Mode,
		"base":                res.Base,
		"target":              res.Target,
	})
	return string(resp)
}

func whatChangedJSON(raw string) string {
	files := 0
	additions := 0
	deletions := 0
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "-") {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) == 2 {
			files++
			stats := strings.TrimSpace(parts[1])
			additions += strings.Count(stats, "+")
			deletions += strings.Count(stats, "-")
		}
	}
	var summary string
	switch {
	case files == 0:
		summary = "No changes"
	case additions > 0 && deletions > 0:
		summary = fmt.Sprintf("Modified %d files (+%d -%d)", files, additions, deletions)
	case additions > 0:
		summary = fmt.Sprintf("Added %d files, +%d lines", files, additions)
	case deletions > 0:
		summary = fmt.Sprintf("Removed %d files, -%d lines", files, deletions)
	default:
		summary = fmt.Sprintf("Modified %d files", files)
	}
	resp, _ := json.Marshal(map[string]interface{}{
		"summary":   summary,
		"files":     files,
		"additions": additions,
		"deletions": deletions,
	})
	return string(resp)
}

func whatChangedJSONWithSummary(raw string, llmSummary string, filterMode string, llmUsed bool) string {
	files := 0
	additions := 0
	deletions := 0
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "-") {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) == 2 {
			files++
			stats := strings.TrimSpace(parts[1])
			additions += strings.Count(stats, "+")
			deletions += strings.Count(stats, "-")
		}
	}

	summary := llmSummary
	if summary == "" {
		switch {
		case files == 0:
			summary = "No changes"
		case additions > 0 && deletions > 0:
			summary = fmt.Sprintf("Modified %d files (+%d -%d)", files, additions, deletions)
		case additions > 0:
			summary = fmt.Sprintf("Added %d files, +%d lines", files, additions)
		case deletions > 0:
			summary = fmt.Sprintf("Removed %d files, -%d lines", files, deletions)
		default:
			summary = fmt.Sprintf("Modified %d files", files)
		}
	}

	if filterMode == "" {
		filterMode = "all"
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"summary":   summary,
		"files":     files,
		"additions": additions,
		"deletions": deletions,
		"mode":      filterMode,
		"llm_used":  llmUsed,
	})
	return string(resp)
}

func extractSummary(jsonStr string) string {
	jsonStr = strings.TrimSpace(jsonStr)
	if !strings.HasPrefix(jsonStr, "{") {
		return ""
	}
	for _, line := range strings.Split(jsonStr, "\n") {
		idx := strings.Index(line, `"summary"`)
		if idx >= 0 {
			remaining := line[idx+len(`"summary"`):]
			remaining = strings.TrimLeft(remaining, `: "`)
			if idxEnd := strings.Index(remaining, `"`); idxEnd >= 0 {
				return remaining[:idxEnd]
			}
		}
	}
	return jsonStr
}
