package workflow

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// skipTypes are commit types excluded from user-facing changelogs.
var skipTypes = map[string]bool{
	"test": true, "chore": true, "ci": true, "build": true,
}

// conventionalRe matches: [hash ]type[(scope)][!]: subject
// The optional hash prefix accepts any non-space word to handle both real git hashes and test data.
var conventionalRe = regexp.MustCompile(`^(?:\w+ )?([a-z]+)(?:\(([^)]+)\))?(!)?: (.+)$`)

type parsedCommit struct {
	commitType string
	scope      string
	breaking   bool
	subject    string
}

func parseConventionalCommit(line string) (parsedCommit, bool) {
	m := conventionalRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return parsedCommit{}, false
	}
	return parsedCommit{
		commitType: m[1],
		scope:      m[2],
		breaking:   m[3] == "!",
		subject:    m[4],
	}, true
}

// FilterAndGroupCommits parses commits from git log --oneline output,
// filters non-user-facing types (test, chore, ci, build) unless breaking,
// and groups the rest by scope. Empty scope → key "".
func FilterAndGroupCommits(commits string) map[string][]string {
	groups := make(map[string][]string)
	for _, line := range strings.Split(commits, "\n") {
		c, ok := parseConventionalCommit(line)
		if !ok {
			continue
		}
		if skipTypes[c.commitType] && !c.breaking {
			continue
		}
		groups[c.scope] = append(groups[c.scope], formatCommitLine(c))
	}
	return groups
}

// FormatGroupedCommits formats area-grouped commits as a string for LLM input.
// Named areas come first (sorted), then the empty-scope group as "general".
func FormatGroupedCommits(groups map[string][]string) string {
	if len(groups) == 0 {
		return ""
	}
	var named []string
	for k := range groups {
		if k != "" {
			named = append(named, k)
		}
	}
	sort.Strings(named)

	var sb strings.Builder
	for _, area := range named {
		sb.WriteString(area + ":\n")
		for _, c := range groups[area] {
			sb.WriteString("- " + c + "\n")
		}
		sb.WriteString("\n")
	}
	if items, ok := groups[""]; ok {
		sb.WriteString("general:\n")
		for _, c := range items {
			sb.WriteString("- " + c + "\n")
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

func formatCommitLine(c parsedCommit) string {
	t := c.commitType
	if c.scope != "" {
		t = fmt.Sprintf("%s(%s)", c.commitType, c.scope)
	}
	if c.breaking {
		t += "!"
	}
	return t + ": " + c.subject
}

// filterForChangelog filters commits for changelog generation.
// It removes commits whose scope matches excluded paths (via IsExcluded)
// and skips non-user-facing types (chore, test, ci, build) unless breaking.
// If cfg is nil, DefaultExcluded is used by IsExcluded.
func filterForChangelog(commits string, cfg *domain.ProjectConfig) map[string][]string {
	groups := make(map[string][]string)
	for _, line := range strings.Split(commits, "\n") {
		c, ok := parseConventionalCommit(line)
		if !ok {
			continue
		}
		if skipTypes[c.commitType] && !c.breaking {
			continue
		}
		// Exclude by scope matching excluded paths
		if cfg != nil && c.scope != "" {
			if cfg.IsExcluded(c.scope) {
				continue
			}
		}
		groups[c.scope] = append(groups[c.scope], formatCommitLine(c))
	}
	return groups
}
