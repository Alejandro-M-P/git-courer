package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

func processingJSON(message string) string {
	structuredPreview := StructuredPreview{
		Header:   "Processing operation",
		Sections: processingSections(message),
		Actions:  []Action{},
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"status":             "pending_approval",
		"show_to_user":       "IMPORTANT: Display ALL fields below to the user before asking for confirmation. Do not summarize.",
		"preview":            message,
		"structured_preview": structuredPreview,
	})
	return string(resp)
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

	resp, _ := json.Marshal(map[string]interface{}{
		"status":             "pending_approval",
		"show_to_user":       "IMPORTANT: Display ALL fields below to the user before asking for confirmation. Do not summarize.",
		"preview":            preview,
		"options":            options,
		"structured_preview": structuredPreview,
	})
	return string(resp)
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

	// Calculate impact based on warnings
	impact := "Medium - Standard release operation"
	if len(warnings) > 0 {
		impact = fmt.Sprintf("High - %d warning(s) detected", len(warnings))
	}

	resp, _ := json.Marshal(map[string]interface{}{
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
	return string(resp)
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

	resp, _ := json.Marshal(map[string]interface{}{
		"status":             "pending_approval",
		"show_to_user":       "IMPORTANT: Display ALL fields below to the user before asking for confirmation. Do not summarize.",
		"preview":            plan.Preview,
		"messages":           plan.Messages,
		"files":              gatherFilesFromChunks(plan.Chunks),
		"reasoning":          plan.Reasoning,
		"options":            options,
		"structured_preview": structuredPreview,
	})
	return string(resp)
}

// --- JSON helpers for git_read structued responses ---

func formatStatusJSON(s domain.Status, limit, offset int, filter string) string {
	files := s.Files
	if filter != "" {
		var filtered []domain.FileStatus
		for _, f := range files {
			if matchesFilter(f.Path, filter) {
				filtered = append(filtered, f)
			}
		}
		files = filtered
	}

	total := len(files)
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
	page := files[offset:end]
	truncated := end < total

	nextOffset := 0
	if truncated {
		nextOffset = end
	}

	paths := make([]string, 0, len(page))
	for _, f := range page {
		paths = append(paths, f.Path)
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"branch":        s.Branch,
		"clean":         s.IsClean,
		"total_files":   total,
		"returned":      len(page),
		"offset":        offset,
		"truncated":     truncated,
		"next_offset":   nextOffset,
		"staged":        s.Staged,
		"modified":      s.Modified,
		"untracked":     s.Untracked,
		"files":         paths,
	})
	return string(resp)
}

func formatBranchListJSON(branches []string, current string, limit, offset int) string {
	total := len(branches)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	page := branches[offset:end]
	truncated := end < total

	nextOffset := 0
	if truncated {
		nextOffset = end
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"current":     current,
		"total":       total,
		"returned":    len(page),
		"offset":      offset,
		"truncated":   truncated,
		"next_offset": nextOffset,
		"branches":    page,
	})
	return string(resp)
}

func formatTagListJSON(tags []string, limit, offset int) string {
	total := len(tags)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	page := tags[offset:end]
	truncated := end < total

	nextOffset := 0
	if truncated {
		nextOffset = end
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"total":       total,
		"returned":    len(page),
		"offset":      offset,
		"truncated":   truncated,
		"next_offset": nextOffset,
		"tags":        page,
	})
	return string(resp)
}

func formatRemoteBranchListJSON(branches []string, limit, offset int) string {
	return formatBranchListJSON(branches, "", limit, offset)
}

func formatRemoteTagListJSON(tags []string, limit, offset int) string {
	return formatTagListJSON(tags, limit, offset)
}

func diffResultJSON(res DiffResult) string {
	resp, _ := json.Marshal(map[string]interface{}{
		"diff":                res.Diff,
		"total_lines":         res.TotalLines,
		"lines_shown":         res.LinesShown,
		"offset":              res.Offset,
		"truncated":           res.Truncated,
		"next_offset":         res.NextOffset,
		"filtered":            res.Filtered,
		"noise_lines_removed": res.NoiseLinesRemoved,
		"mode":                res.Mode,
		"base":                res.Base,
		"target":              res.Target,
	})
	return string(resp)
}

func logResultJSON(res LogResult) string {
	resp, _ := json.Marshal(map[string]interface{}{
		"commits":       res.Commits,
		"total_commits": res.TotalCommits,
		"returned":      res.Returned,
		"offset":        res.Offset,
		"truncated":     res.Truncated,
		"next_offset":   res.NextOffset,
	})
	return string(resp)
}

func writeResultJSON(command string, ok bool, message string) string {
	status := "ok"
	if !ok {
		status = "error"
	}
	resp, _ := json.Marshal(map[string]interface{}{
		"status":  status,
		"command": command,
		"message": message,
	})
	return string(resp)
}

// --- Filter helpers ---

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
	// Simple substring match for now; could be extended to regex
	return strings.Contains(s, pattern)
}
