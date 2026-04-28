package git

import "strings"

// SplitPaths normalizes a path string by replacing commas with spaces
// and splitting on whitespace. Handles both comma-separated and
// space-separated input formats.
func SplitPaths(input string) []string {
	if input == "" {
		return nil
	}
	return strings.Fields(strings.ReplaceAll(input, ",", " "))
}