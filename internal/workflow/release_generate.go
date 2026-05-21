package workflow

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// Generate filters commits, groups by area, and translates to user-facing markdown.
// Always returns isBackground=false; the bool is kept for interface compatibility.
//
// Routing:
//   - No areas configured → generic changelog (Features/Fixes/Breaking format)
//   - Areas configured → area-based changelog with group_N obfuscation
func (s *ReleaseService) Generate(commits string) (string, []string, bool, error) {
	s.taskLog.logStartChangelog()

	// Route based on project config
	if s.projectCfg == nil || len(s.projectCfg.Areas) == 0 {
		return s.generateGeneric(commits)
	}
	return s.generateWithAreas(commits)
}

// generateGeneric produces a changelog when no areas are configured.
// Uses changelog_generate prompt, returns Features/Fixes/Breaking format.
func (s *ReleaseService) generateGeneric(commits string) (string, []string, bool, error) {
	groups := FilterAndGroupCommits(commits)
	if len(groups) == 0 {
		s.taskLog.logChangelogDone(0)
		return "", nil, false, nil
	}

	formatted := FormatGroupedCommits(groups)
	changelog, err := s.llm.GenerateChangelogGeneric(formatted, "", "")
	if err != nil {
		s.taskLog.logError(fmt.Sprintf("changelog generic failed: %v", err))
		return "", []string{err.Error()}, false, err
	}

	md := formatChangelogMarkdown(changelog)
	s.taskLog.logChangelogDone(1)
	return md, nil, false, nil
}

// generateWithAreas produces an area-based changelog when areas are configured.
// Filters by IsExcluded, groups by area with group_N obfuscation, calls LLM,
// then remaps group_N keys back to area names.
// If the formatted groups exceed the token threshold, splits into per-area calls.
func (s *ReleaseService) generateWithAreas(commits string) (string, []string, bool, error) {
	filtered := filterForChangelog(commits, s.projectCfg)
	if len(filtered) == 0 {
		s.taskLog.logChangelogDone(0)
		return "", nil, false, nil
	}

	areaGroups, nameMap := groupByArea(filtered, s.projectCfg)
	if len(areaGroups) == 0 {
		// No commits match any configured area — fall back to generic
		return s.generateGeneric(commits)
	}

	threshold := s.cfg.ContextWindow / 4
	if threshold < 500 {
		threshold = 500
	}

	var changelog domain.ChangelogByArea
	var err error

	if shouldChunkChangelog(areaGroups, threshold) {
		changelog, err = s.generateChangelogByAreaChunked(areaGroups, nameMap)
	} else {
		formatted := FormatGroupedCommits(areaGroups)
		changelog, err = s.llm.GenerateChangelogByArea(formatted, nameMap)
	}
	if err != nil {
		s.taskLog.logError(fmt.Sprintf("changelog by area failed: %v", err))
		return "", []string{err.Error()}, false, err
	}

	// Remap group_N keys back to area names (already done in adapter with nameMap,
	// but also safe to do here for chunked partial results)
	remapped := remapGroupKeys(changelog, nameMap)
	md := formatChangelogByAreaMarkdown(remapped)
	s.taskLog.logChangelogDone(1)
	return md, nil, false, nil
}

// generateChangelogByAreaChunked calls LLM once per area group when the total
// context exceeds the threshold, then merges results.
func (s *ReleaseService) generateChangelogByAreaChunked(areaGroups map[string][]string, nameMap map[string]string) (domain.ChangelogByArea, error) {
	result := make(domain.ChangelogByArea)
	for groupKey, commits := range areaGroups {
		singleGroup := map[string][]string{groupKey: commits}
		formatted := FormatGroupedCommits(singleGroup)
		partial, err := s.llm.GenerateChangelogByArea(formatted, nameMap)
		if err != nil {
			// Skip failed areas but continue with others
			result[groupKey] = []string{fmt.Sprintf("(could not generate: %v)", err)}
			continue
		}
		for k, v := range partial {
			result[k] = v
		}
	}
	return result, nil
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

// groupByArea maps scope-grouped commits to group_N keys for LLM obfuscation.
// Areas are sorted alphabetically → group_1, group_2, etc.
// Only scopes that match configured area names are included in area groups.
// Returns the grouped map (keyed by group_N) and nameMap (group_N → area name).
func groupByArea(groups map[string][]string, cfg *domain.ProjectConfig) (map[string][]string, map[string]string) {
	if cfg == nil || len(cfg.Areas) == 0 {
		return nil, nil
	}

	// Sort area names for deterministic group assignment
	sortedAreas := make([]string, 0, len(cfg.Areas))
	for area := range cfg.Areas {
		sortedAreas = append(sortedAreas, area)
	}
	sort.Strings(sortedAreas)

	// Build name map: area → group_N
	areaToGroup := make(map[string]string)
	nameMap := make(map[string]string)
	for i, area := range sortedAreas {
		groupKey := fmt.Sprintf("group_%d", i+1)
		areaToGroup[area] = groupKey
		nameMap[groupKey] = area
	}

	// Map commits from scope to group_N
	result := make(map[string][]string)
	for area, groupKey := range areaToGroup {
		if commits, ok := groups[area]; ok {
			result[groupKey] = commits
		}
	}
	// Any scopeless commits go to a "general" group if present
	if commits, ok := groups[""]; ok {
		result["group_general"] = commits
		nameMap["group_general"] = "general"
	}

	return result, nameMap
}

// remapGroupKeys replaces group_N keys in ChangelogByArea with the actual area names
// using the nameMap obtained from groupByArea.
func remapGroupKeys(ch domain.ChangelogByArea, nameMap map[string]string) domain.ChangelogByArea {
	result := make(domain.ChangelogByArea)
	for groupKey, items := range ch {
		areaName, ok := nameMap[groupKey]
		if !ok {
			areaName = groupKey // fallback: keep group_N key
		}
		result[areaName] = items
	}
	return result
}

// estimateTokens provides a rough token estimate for a text string.
// Uses ~4 chars per token as a conservative estimate.
func estimateTokens(text string) int {
	return len(text) / 4
}

// shouldChunkChangelog determines if the total formatted groups exceed the
// token threshold and should be split into per-area calls.
func shouldChunkChangelog(groups map[string][]string, tokenThreshold int) bool {
	totalTokens := 0
	for _, commits := range groups {
		for _, c := range commits {
			totalTokens += estimateTokens(c)
		}
	}
	return totalTokens > tokenThreshold
}
