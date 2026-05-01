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
