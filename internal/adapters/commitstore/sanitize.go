// Package commitstore provides a filesystem-backed CommitStore adapter
// that persists commit entries as JSONL in .git/git-courer/commits.json.
package commitstore

import (
	"regexp"
	"strings"
)

// uuidPattern matches RFC 4122 form: 8-4-4-4-12 hex digits.
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// isUUID reports whether workspaceID is a valid RFC 4122 UUID string.
// It is used by SetWorkspace to decide whether sanitization can be skipped
// (UUIDs are already filesystem-safe).
func isUUID(workspaceID string) bool {
	return uuidPattern.MatchString(workspaceID)
}

// SanitizeBranchName converts a git branch name into a safe directory name.
// It is the single source of truth for branch-name sanitization; callers
// (e.g. workflow/release.go) MUST import and reuse this instead of keeping
// private duplicates.
//
// Rules:
//   - Replace '/' with '-'
//   - Remove '~', '^', ':', '\\', ' ' (space)
//   - Collapse consecutive '-' into single '-'
//   - Trim leading/trailing '-'
//   - If result is empty, return "HEAD"
func SanitizeBranchName(name string) string {
	return sanitizeBranchName(name)
}

// sanitizeBranchName converts a git branch name into a safe directory name.
// Rules:
//   - Replace '/' with '-'
//   - Remove '~', '^', ':', '\\', ' ' (space)
//   - Collapse consecutive '-' into single '-'
//   - Trim leading/trailing '-'
//   - If result is empty, return "HEAD"
func sanitizeBranchName(name string) string {
	r := strings.ReplaceAll(name, "/", "-")
	for _, ch := range []string{"~", "^", ":", "\\", " "} {
		r = strings.ReplaceAll(r, ch, "")
	}
	// Collapse consecutive dashes
	for strings.Contains(r, "--") {
		r = strings.ReplaceAll(r, "--", "-")
	}
	r = strings.Trim(r, "-")
	if r == "" {
		return "HEAD"
	}
	return r
}
