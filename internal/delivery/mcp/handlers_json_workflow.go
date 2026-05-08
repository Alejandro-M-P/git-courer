package mcp

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

func processingJSON(message string) string {
	structuredPreview := StructuredPreview{
		Header:   "Processing operation",
		Sections: processingSections(message),
		Actions:  []Action{},
	}

	return mustJSON(map[string]interface{}{
		"status":             "pending_approval",
		"show_to_user":       "IMPORTANT: Display ALL fields below to the user before asking for confirmation. Do not summarize.",
		"preview":            message,
		"structured_preview": structuredPreview,
	})
}

func readyJSON(preview string) string {
	structuredPreview := StructuredPreview{
		Header:   "Review operation details",
		Sections: genericSections("Generic git operation", preview),
		Actions:  genericActions(),
	}

	options := make([]string, 0, len(structuredPreview.Actions))
	for _, action := range structuredPreview.Actions {
		options = append(options, action.Label)
	}

	return mustJSON(map[string]interface{}{
		"status":             "pending_approval",
		"show_to_user":       "IMPORTANT: Display ALL fields below to the user before asking for confirmation. Do not summarize.",
		"preview":            preview,
		"options":            options,
		"structured_preview": structuredPreview,
	})
}

func formatStatus(s domain.Status) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Branch: %s\n", s.Branch))
	if len(s.Files) == 0 {
		sb.WriteString("Working tree clean\n")
		return sb.String()
	}
	for _, f := range s.Files {
		sb.WriteString(fmt.Sprintf("%s%s\n", f.Status, f.Path))
	}
	return sb.String()
}

func releasePlanJSON(intent *domain.ReleaseIntent, changelog string, warnings []string, ghAuth string) string {
	structuredPreview := StructuredPreview{
		Header:   "Review release details",
		Sections: releaseSections(intent, changelog, warnings, ghAuth),
		Actions:  releaseActions(),
	}

	options := make([]string, 0, len(structuredPreview.Actions))
	for _, action := range structuredPreview.Actions {
		options = append(options, action.Label)
	}

	impact := "Medium - Standard release operation"
	if len(warnings) > 0 {
		impact = fmt.Sprintf("High - %d warning(s) detected", len(warnings))
	}

	return mustJSON(map[string]interface{}{
		"status":             "pending_approval",
		"show_to_user":       "IMPORTANT: Display ALL fields below to the user before asking for confirmation. Do not summarize.",
		"tag_name":           intent.TagName,
		"version":            intent.VersionBump,
		"changelog":          changelog,
		"github_auth":        ghAuth,
		"warnings":           warnings,
		"impact":             impact,
		"options":            options,
		"structured_preview": structuredPreview,
	})
}

func commitPlanJSON(plan *domain.OperationPlan) string {
	structuredPreview := StructuredPreview{
		Header:   "Review commit details",
		Sections: commitSections(plan),
		Actions:  commitActions(),
	}

	options := make([]string, 0, len(structuredPreview.Actions))
	for _, action := range structuredPreview.Actions {
		options = append(options, action.Label)
	}

	return mustJSON(map[string]interface{}{
		"status":             "pending_approval",
		"show_to_user":       "IMPORTANT: Display ALL fields below to the user before asking for confirmation. Do not summarize.",
		"preview":            plan.Preview,
		"messages":           plan.Messages,
		"files":              gatherFilesFromChunks(plan.Chunks),
		"reasoning":          plan.Reasoning,
		"options":            options,
		"structured_preview": structuredPreview,
	})
}

func gatherFilesFromChunks(chunks [][]string) []string {
	seen := make(map[string]bool)
	var files []string
	for _, chunk := range chunks {
		for _, f := range chunk {
			if !seen[f] {
				seen[f] = true
				files = append(files, f)
			}
		}
	}
	return files
}

func tagResultJSON(op, tag string) string {
	return fmt.Sprintf(`{"operation": "tag_%s", "tag": %q, "status": "success"}`, op, tag)
}

func writeResultJSON(command string, ok bool, message string) string {
	return mustJSON(map[string]interface{}{
		"success":   ok,
		"operation": command,
		"message":   message,
	})
}

func filterStringSlice(items []string, pattern string) []string {
	var result []string
	for _, item := range items {
		if matchesFilter(item, pattern) {
			result = append(result, item)
		}
	}
	return result
}

func filterCommits(commits []CommitEntry, pattern string) []CommitEntry {
	var result []CommitEntry
	for _, c := range commits {
		if matchesFilter(c.Message, pattern) {
			result = append(result, c)
		}
	}
	return result
}

func matchesFilter(s, pattern string) bool {
	if pattern == "" {
		return true
	}
	if strings.Contains(s, pattern) {
		return true
	}
	return strings.Contains(filepath.Base(s), pattern)
}

func filterDiffByFile(diff string, fileFilter string) string {
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

func filterDiffCompact(diff string) string {
	if diff == "" {
		return diff
	}

	lines := strings.Split(diff, "\n")
	var sb strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "+") || strings.HasPrefix(trimmed, "-") {
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}

	return sb.String()
}
