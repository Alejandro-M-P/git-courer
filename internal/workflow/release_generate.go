package workflow

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// Generate filters commits, groups by area, and translates to user-facing markdown.
// Always returns isBackground=false; the bool is kept for interface compatibility.
func (s *ReleaseService) Generate(commits string) (string, []string, bool, error) {
	s.taskLog.logStartChangelog()

	groups := FilterAndGroupCommits(commits)
	if len(groups) == 0 {
		s.taskLog.logChangelogDone(0)
		return "", nil, false, nil
	}

	formatted := FormatGroupedCommits(groups)
	changelog, err := s.llm.GenerateChangelogByArea(formatted)
	if err != nil {
		s.taskLog.logError(fmt.Sprintf("changelog failed: %v", err))
		return "", []string{err.Error()}, false, err
	}

	md := formatChangelogByAreaMarkdown(changelog)
	s.taskLog.logChangelogDone(1)
	return md, nil, false, nil
}

// formatChangelogByAreaMarkdown renders a ChangelogByArea as markdown.
// Named areas are sorted alphabetically; "general" is always last.
func formatChangelogByAreaMarkdown(ch domain.ChangelogByArea) string {
	if len(ch) == 0 {
		return ""
	}
	var areas []string
	for k := range ch {
		if k != "general" {
			areas = append(areas, k)
		}
	}
	sort.Strings(areas)
	if _, ok := ch["general"]; ok {
		areas = append(areas, "general")
	}

	var sb strings.Builder
	for _, area := range areas {
		items := ch[area]
		if len(items) == 0 {
			continue
		}
		sb.WriteString("## " + titleCase(area) + "\n")
		for _, item := range items {
			sb.WriteString("- " + item + "\n")
		}
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// formatChangelogMarkdown renders a legacy domain.Changelog as markdown.
// Kept for backward compatibility with existing tests.
func formatChangelogMarkdown(ch *domain.Changelog) string {
	var sb strings.Builder
	if len(ch.Features) > 0 {
		sb.WriteString("## Features\n")
		for _, f := range ch.Features {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
		sb.WriteString("\n")
	}
	if len(ch.Fixes) > 0 {
		sb.WriteString("## Fixes\n")
		for _, f := range ch.Fixes {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
		sb.WriteString("\n")
	}
	if len(ch.Breaking) > 0 {
		sb.WriteString("## Breaking Changes\n")
		for _, f := range ch.Breaking {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
		sb.WriteString("\n")
	}
	if len(ch.Docs) > 0 {
		sb.WriteString("## Documentation\n")
		for _, f := range ch.Docs {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
		sb.WriteString("\n")
	}
	if len(ch.Perf) > 0 {
		sb.WriteString("## Performance\n")
		for _, f := range ch.Perf {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
		sb.WriteString("\n")
	}
	if len(ch.Internal) > 0 {
		sb.WriteString("## Internal\n")
		for _, f := range ch.Internal {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
	}
	return strings.TrimSpace(sb.String())
}
