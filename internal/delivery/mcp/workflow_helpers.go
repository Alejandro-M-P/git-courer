package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/blak0p/git-courer/internal/core/domain"
)

// Operation types constants
const (
	OpCommit     = "commit"
	OpRelease    = "release"
	OpBranch     = "branch"
	OpTag        = "tag"
	OpMerge      = "merge"
	OpReset      = "reset"
	OpCherryPick = "cherry_pick"
	OpRevert     = "revert"
	OpClean      = "clean"
	OpRemote     = "remote"
	OpClone      = "clone"
	OpInit       = "init"
	OpGeneric    = "generic"
	OpProcessing = "processing"
)

// StructuredPreview represents a preview broken down into sections with actionable options
type StructuredPreview struct {
	Header   string           `json:"header"`
	Sections []PreviewSection `json:"sections"`
	Actions  []Action         `json:"actions"`
}

// PreviewSection is a categorized section of the preview content
type PreviewSection struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Type    string `json:"type,omitempty"` // "text", "list", "code", "warning"
}

// Action represents an actionable option for the user
type Action struct {
	Label string `json:"label"` // Human-readable (e.g., "Execute")
	Key   string `json:"key"`   // Machine-actionable (e.g., "apply")
}

// commitSections returns the sections for a commit preview
func commitSections(plan *domain.OperationPlan) []PreviewSection {
	sections := []PreviewSection{
		{
			Title:   "Operation",
			Content: fmt.Sprintf("Commit with %d message(s)", len(plan.Messages)),
			Type:    "text",
		},
	}

	if len(plan.Messages) > 0 {
		msgs := strings.Join(plan.Messages, "\n")
		sections = append(sections, PreviewSection{
			Title:   "Messages",
			Content: msgs,
			Type:    "code",
		})
	}

	files := []string{}
	if len(plan.Chunks) > 0 {
		files = gatherFilesFromChunks(plan.Chunks)
		sections = append(sections, PreviewSection{
			Title:   "Files",
			Content: strings.Join(files, "\n"),
			Type:    "list",
		})
	}

	if plan.Reasoning != "" {
		sections = append(sections, PreviewSection{
			Title:   "Reasoning",
			Content: plan.Reasoning,
			Type:    "text",
		})
	}

	// Impact section based on file count and operation type
	impact := fmt.Sprintf("This commit will modify %d file(s).", len(files))
	sections = append(sections, PreviewSection{
		Title:   "Impact",
		Content: impact,
		Type:    "warning",
	})

	return sections
}

// releaseSections returns the sections for a release preview
func releaseSections(intent *domain.ReleaseIntent, changelog string, warnings []string, ghAuth string) []PreviewSection {
	sections := []PreviewSection{
		{
			Title:   "Operation",
			Content: fmt.Sprintf("Create release tag %s", intent.TagName),
			Type:    "text",
		},
		{
			Title:   "Version",
			Content: fmt.Sprintf("Applying %s bump", intent.VersionBump),
			Type:    "text",
		},
	}

	if changelog != "" {
		sections = append(sections, PreviewSection{
			Title:   "Changelog",
			Content: changelog,
			Type:    "code",
		})
	}

	if ghAuth != "" {
		sections = append(sections, PreviewSection{
			Title:   "GitHub Auth",
			Content: ghAuth,
			Type:    "text",
		})
	}

	if len(warnings) > 0 {
		warningContent := strings.Join(warnings, "\n")
		sections = append(sections, PreviewSection{
			Title:   "Warnings",
			Content: warningContent,
			Type:    "warning",
		})
	}

	// Impact section based on warnings
	impact := "This release will create a new tag and potentially trigger CI/CD pipelines."
	if len(warnings) > 0 {
		impact += fmt.Sprintf(" Contains %d warning(s).", len(warnings))
	}
	sections = append(sections, PreviewSection{
		Title:   "Impact",
		Content: impact,
		Type:    "warning",
	})

	return sections
}

// genericSections returns the sections for a generic operation preview
func genericSections(operation, preview string) []PreviewSection {
	return []PreviewSection{
		{
			Title:   "Operation",
			Content: operation,
			Type:    "text",
		},
		{
			Title:   "Preview",
			Content: preview,
			Type:    "code",
		},
		{
			Title:   "Impact",
			Content: "This operation will modify git state. Verify details before execution.",
			Type:    "warning",
		},
	}
}

// processingSections returns the sections for a processing preview
func processingSections(message string) []PreviewSection {
	return []PreviewSection{
		{
			Title:   "Processing",
			Content: message,
			Type:    "text",
		},
		{
			Title:   "Status",
			Content: "Operation is being processed. Wait for completion.",
			Type:    "text",
		},
	}
}

// commitActions returns the actionable options for a commit preview
func commitActions() []Action {
	return []Action{
		{Label: "Execute", Key: "apply"},
		{Label: "Regenerate message", Key: "regenerate"},
		{Label: "Edit message", Key: "edit"},
		{Label: "Cancel", Key: "abort"},
	}
}

// releaseActions returns the actionable options for a release preview
func releaseActions() []Action {
	return []Action{
		{Label: "Execute", Key: "apply"},
		{Label: "Cancel", Key: "abort"},
	}
}

// genericActions returns the actionable options for a generic operation preview
func genericActions() []Action {
	return []Action{
		{Label: "Continue", Key: "apply"},
		{Label: "Cancel", Key: "abort"},
		{Label: "View details", Key: "details"},
	}
}

func stashListResultJSON(entries []domain.StashEntry, limit, offset int) string {
	type stItem struct {
		Index   int    `json:"index"`
		Message string `json:"message"`
		Hash    string `json:"hash"`
	}
	if offset > len(entries) {
		offset = len(entries)
	}
	if limit <= 0 || limit > 50 {
		limit = 50
	}
	end := offset + limit
	if end > len(entries) {
		end = len(entries)
	}
	items := make([]stItem, 0, end-offset)
	for _, e := range entries[offset:end] {
		items = append(items, stItem{Index: e.Index, Message: e.Message, Hash: e.Hash})
	}
	resp, _ := json.Marshal(map[string]interface{}{
		"stashes":     items,
		"total":       len(entries),
		"offset":      offset,
		"limit":       limit,
		"next_offset": end,
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
