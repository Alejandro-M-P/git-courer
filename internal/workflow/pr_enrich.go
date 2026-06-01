package workflow

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/blak0p/git-courer/internal/core/domain"
	giturls "github.com/whilp/git-urls"
)

var prRefRE = regexp.MustCompile(`Merge pull request #(\d+)|\(#(\d+)\)`)

// detectPRNumbers scans commit messages for PR references.
// Supports squash-merges "(#N)" and merge-commits "Merge pull request #N".
// Returns deduplicated PR numbers preserving first-appearance order.
func detectPRNumbers(commits string) []int {
	seen := make(map[int]struct{})
	var result []int
	for _, m := range prRefRE.FindAllStringSubmatch(commits, -1) {
		var numStr string
		if m[1] != "" {
			numStr = m[1]
		} else {
			numStr = m[2]
		}
		var n int
		if _, err := fmt.Sscanf(numStr, "%d", &n); err != nil {
			continue
		}
		if _, ok := seen[n]; !ok {
			seen[n] = struct{}{}
			result = append(result, n)
		}
	}
	return result
}

// mergeEnrichedCommits replaces lines that reference a PR with the PR's commits.
// Lines without PR references pass through unchanged.
// order of non-replaced lines is preserved.
func mergeEnrichedCommits(raw string, enriched map[int][]domain.PRCommit) string {
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		found := false
		for _, m := range prRefRE.FindAllStringSubmatch(line, -1) {
			var numStr string
			if m[1] != "" {
				numStr = m[1]
			} else {
				numStr = m[2]
			}
			var pr int
			if _, err := fmt.Sscanf(numStr, "%d", &pr); err != nil {
				continue
			}
			if commits, ok := enriched[pr]; ok {
				for _, c := range commits {
					out = append(out, c.Message)
				}
				found = true
				break
			}
		}
		if !found {
			// keep the original line only if it wasn't a newline-only or whitespace-only
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			out = append(out, trimmed)
		}
	}
	return strings.Join(out, "\n")
}

// resolveOwnerRepo parses a git remote URL and extracts the owner and repo name.
// Supports all standard git URL formats (HTTPS, SSH, SCP-like) via git-urls.
// Returns isGitHub=true only when the host is "github.com".
func resolveOwnerRepo(remoteURL string) (owner, repo string, isGitHub bool, err error) {
	if remoteURL == "" {
		return "", "", false, nil
	}

	u, err := giturls.Parse(remoteURL)
	if err != nil {
		return "", "", false, fmt.Errorf("parse remote URL: %w", err)
	}

	if u.Hostname() != "github.com" {
		return "", "", false, nil
	}

	// Path is like "blak0p/git-courer.git" or "blak0p/git-courer"
	path := strings.TrimPrefix(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")

	parts := strings.SplitN(path, "/", 3) // max 3 to prevent extra segments
	if len(parts) < 2 {
		return "", "", false, fmt.Errorf("invalid repo path: %s", path)
	}

	return parts[0], parts[1], true, nil
}
