package shared

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/blak0p/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func MustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(b)
}

// ValidateRequiredParam checks that a required parameter is present and non-empty.
func ValidateRequiredParam(params map[string]any, key, command string) (*mcpgo.CallToolResult, error) {
	val := GetStringParam(params, key, "")
	if val == "" {
		errResult, _ := JSONErrorResult(command, fmt.Errorf("%s is required for %s", key, command))
		return errResult, nil
	}
	return nil, nil
}

// suggestCommand returns a "Did you mean X?" hint for a mistyped command.
func SuggestCommand(input string, validCommands []string) string {
	if len(validCommands) == 0 {
		return ""
	}
	bestCmd := ""
	bestDist := len(input) + 1

	for _, cmd := range validCommands {
		if len(input) >= 2 && (cmd[:min(len(cmd), len(input))] == input || input[:min(len(input), len(cmd))] == cmd) {
			return cmd
		}
		d := Levenshtein(input, cmd)
		if d < bestDist {
			bestDist = d
			bestCmd = cmd
		}
	}

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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func min3(a, b, c int) int {
	return min(a, min(b, c))
}

func Levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

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
			curr[j] = min3(
				prev[j]+1,
				curr[j-1]+1,
				prev[j-1]+cost,
			)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

// ValidateKnownParams checks that all params in the request are in the allowed set.
func ValidateKnownParams(params map[string]any, allowedKeys []string) (*mcpgo.CallToolResult, error) {
	allowed := make(map[string]bool, len(allowedKeys))
	for _, k := range allowedKeys {
		allowed[k] = true
	}

	for key := range params {
		if !allowed[key] {
			errResult, _ := JSONErrorResult("", fmt.Errorf("unknown parameter: %s", key))
			return errResult, nil
		}
	}
	return nil, nil
}

// JSONErrorResult returns a structured JSON error.
func JSONErrorResult(command string, err error) (*mcpgo.CallToolResult, error) {
	errJSON := fmt.Sprintf(`{"status":"error","command":%q,"error":%q}`, command, err.Error())
	return mcpgo.NewToolResultError(errJSON), nil
}

func ParsePagination(params map[string]any) (limit, offset int) {
	if v, ok := params["limit"].(float64); ok {
		limit = int(v)
	}
	if v, ok := params["offset"].(float64); ok {
		offset = int(v)
	}
	return
}

func GetStringParam(params map[string]any, key, def string) string {
	if v, ok := params[key].(string); ok && v != "" {
		return v
	}
	return def
}

func FilterStringSlice(items []string, pattern string) []string {
	var result []string
	for _, item := range items {
		if MatchesFilter(item, pattern) {
			result = append(result, item)
		}
	}
	return result
}

func FilterCommits(commits []CommitEntry, pattern string) []CommitEntry {
	var result []CommitEntry
	for _, c := range commits {
		if MatchesFilter(c.Message, pattern) {
			result = append(result, c)
		}
	}
	return result
}

func MatchesFilter(s, pattern string) bool {
	if pattern == "" {
		return true
	}
	if strings.Contains(s, pattern) {
		return true
	}
	// Note: requires "path/filepath" import
	return strings.Contains(filepath.Base(s), pattern)
}

// destructiveCommands lists operations that require confirmed=true when dry_run=false.
var destructiveCommands = map[string]bool{
	"push":              true,
	"branch_delete":     true,
	"clean":             true,
	"rewrite_amend":     true,
	"rewrite_revert":    true,
	"rewrite_hard":      true,
	"integrate_merge":   true,
	"integrate_update":  true,
	"integrate_pick":    true,
	"integrate_continue": true,
	"integrate_abort":   true,
}

// CheckSafetyGate validates dry_run and confirmed for destructive commands.
// Returns nil when execution may proceed, otherwise an MCP result with an error message.
func CheckSafetyGate(cmd string, dryRun, confirmed bool) (*mcpgo.CallToolResult, error) {
	// If dry_run is true, allow preview (computeImpact will be handled by caller).
	if dryRun {
		return nil, nil
	}
	// If command is not flagged as destructive, proceed without confirmation.
	if !destructiveCommands[cmd] {
		return nil, nil
	}
	// Destructive command requires confirmed=true.
	if !confirmed {
		impact, _ := ComputeImpact(cmd, nil)
		impact["status"] = "blocked"
		impact["reason"] = fmt.Sprintf("%s requires explicit confirmation. Set confirmed=true to proceed.", cmd)
		jsonBytes, err := json.Marshal(impact)
		if err != nil {
			return nil, err
		}
		return mcpgo.NewToolResultError(string(jsonBytes)), nil
	}
	return nil, nil
}

// ComputeImpact returns a preview JSON map describing the operation's effects.
func ComputeImpact(cmd string, params map[string]any) (map[string]any, error) {
	result := map[string]any{
		"operation": cmd,
		"undoable":  OperationUndoability[cmd],
	}

	switch cmd {
	case "push":
		result["affected_refs"] = []string{"HEAD"}
		result["remote"] = "origin"
		result["hint"] = "Will push local commits to remote. Not undoable via backup RESTORE."
	case "clean":
		result["affected_files"] = "untracked"
		result["hint"] = "Will remove all untracked files. Not undoable via backup RESTORE."
	case "rewrite_hard":
		result["affected_refs"] = []string{"HEAD", "working tree"}
		result["hint"] = "Will reset working tree and index. Not undoable via backup RESTORE."
	case "branch_delete":
		result["affected_refs"] = []string{"local branch"}
		result["hint"] = "Will delete a local branch. Not undoable via backup RESTORE."
	case "integrate_merge":
		result["affected_refs"] = []string{"HEAD", "branch"}
		result["hint"] = "Will merge the specified branch. Undoable via backup RESTORE."
	case "integrate_update":
		result["affected_refs"] = []string{"HEAD", "branch"}
		result["hint"] = "Will rebase current branch onto target. Undoable via backup RESTORE. Use integrate ABORT if conflicts arise."
	case "integrate_pick":
		result["affected_refs"] = []string{"HEAD"}
		result["hint"] = "Will apply commit on top of current branch. Undoable via backup RESTORE."
	case "rewrite_revert":
		result["affected_refs"] = []string{"HEAD"}
		result["hint"] = "Will create a revert commit. Undoable via backup RESTORE."
	case "rewrite_amend":
		result["affected_refs"] = []string{"HEAD"}
		result["hint"] = "Will amend the last commit. Undoable via backup RESTORE."
	default:
		result["hint"] = "Operation preview not available. Proceed with caution."
	}

	return result, nil
}

var OperationUndoability = map[string]bool{
	"commit":                true,
	"integrate_merge":       true,
	"integrate_update":      true,
	"integrate_pick":        true,
	"branch_create":         true,
	"branch_rename":         true,
	"push":                  false,
	"branch_delete":         false,
	"rewrite_soft":          true,
	"rewrite_hard":          false,
	"clean":                 false,
	"rewrite_revert":        true,
	"rewrite_amend":         true,
	"add":                   true,
	"rm":                    true,
	"restore":               true,
	"stash_save":            true,
	"stash_pop":             true,
	"stash_drop":            false,
	"stash_clear":           false,
}

func WriteResultJSON(command string, ok bool, message string) string {
	return MustJSON(map[string]interface{}{
		"success":   ok,
		"operation": command,
		"message":   message,
	})
}

// WriteHintedResultJSON returns a JSON response with an additional "hint" field
// that suggests the next natural action after a successful operation.
func WriteHintedResultJSON(command string, ok bool, message, hint string) string {
	return MustJSON(map[string]interface{}{
		"success":   ok,
		"operation": command,
		"message":   message,
		"hint":      hint,
	})
}

func TagResultJSON(op, tag string) string {
	return fmt.Sprintf(`{"operation": "tag_%s", "tag": %q, "status": "success"}`, op, tag)
}

type ConflictResult struct {
	Status  string   `json:"status"`
	Message string   `json:"message"`
	Files   []string `json:"conflicted_files"`
}

func ConflictResultJSON(files []string, hint string) string {
	if files == nil {
		files = []string{}
	}
	result := ConflictResult{
		Status:  "conflict",
		Message: hint,
		Files:   files,
	}

	bytes, err := json.Marshal(result)
	if err != nil {
		return `{"error": "Failed to marshal ConflictResult"}`
	}

	return string(bytes)
}

// FormatStatusJSON formats a domain.Status into a paginated, filtered JSON string.
func FormatStatusJSON(s domain.Status, limit, offset int, filter string, userName, userEmail, testCommand, remotes string) string {
	files := s.Files
	if filter != "" {
		var filtered []domain.FileStatus
		for _, f := range files {
			if MatchesFilter(f.Path, filter) {
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

	type fileItem struct {
		Path   string `json:"path"`
		Status string `json:"status"`
		Staged bool   `json:"staged"`
	}

	fItems := make([]fileItem, 0, len(page))
	for _, f := range page {
		fItems = append(fItems, fileItem{
			Path:   f.Path,
			Status: f.Status,
			Staged: f.Staged,
		})
	}

	return MustJSON(map[string]interface{}{
		"branch":       s.Branch,
		"ahead":        s.Ahead,
		"behind":       s.Behind,
		"has_upstream": s.HasUpstream,
		"clean":        s.IsClean,
		"total":        total,
		"returned":     len(page),
		"offset":       offset,
		"truncated":    truncated,
		"next_offset":  nextOffset,
		"staged":       s.Staged,
		"modified":     s.Modified,
		"untracked":    s.Untracked,
		"files":        fItems,
		"user_name":    userName,
		"user_email":   userEmail,
		"test_command": testCommand,
		"remotes":      remotes,
	})
}

// DiffResultJSON formats a DiffResult into a JSON string.
func DiffResultJSON(res DiffResult) string {
	m := map[string]interface{}{
		"diff":                res.Diff,
		"total_lines":         res.TotalLines,
		"lines_shown":         res.LinesShown,
		"offset":              res.Offset,
		"truncated":           res.Truncated,
		"filtered_file":       res.Filtered,
		"noise_lines_removed": res.NoiseLinesRemoved,
	}

	if res.NextOffset != 0 {
		m["next_offset"] = res.NextOffset
	}
	if res.Mode != "" {
		m["mode"] = res.Mode
	}
	if res.Base != "" {
		m["base"] = res.Base
	}
	if res.Target != "" {
		m["target"] = res.Target
	}
	if res.Annotated != "" {
		m["annotated"] = res.Annotated
	}

	return MustJSON(m)
}
