// Package commitstore provides a filesystem-backed CommitStore adapter
// that persists commit entries as JSONL in .git-courer/commits.json.
package commitstore

import "strings"

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