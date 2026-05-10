package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func bgJobResultJSON(j *bgJob) string {
	j.mu.Lock()
	m := map[string]any{
		"job_id":  j.ID,
		"op":      j.Op,
		"status":  string(j.Status),
		"elapsed": time.Since(j.StartedAt).Round(time.Second).String(),
	}
	if j.Progress != "" {
		m["progress"] = j.Progress
	}
	if j.Result != "" {
		m["result"] = j.Result
	}
	if j.Error != "" {
		m["error"] = j.Error
	}
	j.mu.Unlock()
	return mustJSON(m)
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(b)
}

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

// validateRequiredParam checks that a required parameter is present and non-empty.
// Returns a structured JSON error result if the param is missing or empty.
// Returns nil if the param is valid — callers should check for non-nil result.
func validateRequiredParam(params map[string]any, key, command string) (*mcpgo.CallToolResult, error) {
	val := getStringParam(params, key, "")
	if val == "" {
		errResult, _ := jsonErrorResult(command, fmt.Errorf("%s is required for %s", key, command))
		return errResult, nil
	}
	return nil, nil
}

// suggestCommand returns a "Did you mean X?" hint for a mistyped command.
// It uses Levenshtein distance to find the closest valid command.
// If no command is close enough, it returns an empty string.
func suggestCommand(input string, validCommands []string) string {
	if len(validCommands) == 0 {
		return ""
	}
	bestCmd := ""
	bestDist := len(input) + 1 // threshold: must be at most half the input length

	// Also try prefix matching (e.g., "CREAT" matches "CREATE")
	for _, cmd := range validCommands {
		// Prefix match first
		if strings.HasPrefix(cmd, input) || strings.HasPrefix(input, cmd) {
			if len(input) >= 2 {
				return cmd
			}
		}

		// Levenshtein distance
		d := levenshtein(input, cmd)
		if d < bestDist {
			bestDist = d
			bestCmd = cmd
		}
	}

	// Only suggest if distance is reasonable (<= half the length of input or 3, whichever is smaller)
	threshold := len(input) / 2
	if threshold > 3 {
		threshold = 3
	}
	if threshold < 1 {
		threshold = 1
	}
	if bestDist <= threshold {
		return bestCmd
	}
	return ""
}

// levenshtein computes the Levenshtein distance between two strings.
func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	// Use two-row optimization
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(
				prev[j]+1,     // deletion
				curr[j-1]+1,   // insertion
				prev[j-1]+cost, // substitution
			)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

// validateKnownParams checks that all params in the request are in the allowed set.
// Returns a structured JSON error result if an unknown param is found.
// Returns nil if all params are known — callers should check for non-nil result.
func validateKnownParams(params map[string]any, allowedKeys []string) (*mcpgo.CallToolResult, error) {
	allowed := make(map[string]bool, len(allowedKeys))
	for _, k := range allowedKeys {
		allowed[k] = true
	}

	for key := range params {
		if !allowed[key] {
			errResult, _ := jsonErrorResult("", fmt.Errorf("unknown parameter: %s", key))
			return errResult, nil
		}
	}
	return nil, nil
}

