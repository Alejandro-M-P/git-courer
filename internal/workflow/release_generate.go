package workflow

import (
	"fmt"
	"sort"
	"strings"

	"github.com/blak0p/git-courer/internal/core/domain"
)

// Generate filters commits, groups them, and translates to user-facing markdown.
// When BaseBranch is configured (non-empty), uses stack-based grouping.
// When areas are configured (and BaseBranch is empty), uses area-based grouping.
// Always returns isBackground=false; the bool is kept for interface compatibility.
// If a custom message was set via SetCustomMessage, it is injected into the LLM prompt.
func (s *ReleaseService) Generate(commits string) (string, []string, bool, error) {
	// Stack-based path: when BaseBranch is configured, group by StackID
	if s.projectCfg != nil && s.projectCfg.BaseBranch != "" {
		return s.generateWithStacks(commits)
	}

	// Area-based path (original): requires areas to be configured
	if s.projectCfg == nil || len(s.projectCfg.Areas) == 0 {
		return "", nil, false, fmt.Errorf("changelog generation requires project areas to be configured — run git-courer init")
	}
	return s.generateWithAreas(commits)
}

// generateWithAreas produces an area-based changelog when areas are configured.
// Filters by IsExcluded, groups by area with group_N obfuscation, calls LLM,
// then remaps group_N keys back to area names.
// If the formatted groups exceed the token threshold, splits into per-area calls.
func (s *ReleaseService) generateWithAreas(commits string) (string, []string, bool, error) {
	filtered := filterForChangelog(commits, s.projectCfg)
	if len(filtered) == 0 {
		return "", nil, false, nil
	}

	areaGroups, nameMap := groupByArea(filtered, s.projectCfg)
	if len(areaGroups) == 0 {
		return "", nil, false, nil
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
		changelog, err = s.llm.GenerateChangelogGrouped(formatted, nameMap, s.customMessage, "area")
	}
	if err != nil {
		return "", []string{err.Error()}, false, err
	}

	// Remap group_N keys back to area names (already done in adapter with nameMap,
	// but also safe to do here for chunked partial results)
	remapped := remapGroupKeys(changelog, nameMap)
	md := formatChangelogByAreaMarkdown(remapped)
	return md, nil, false, nil
}

// generateChangelogByAreaChunked calls LLM once per area group when the total
// context exceeds the threshold, then merges results.
// It also splits large commit lists within a single group into smaller chunks
// of 25 commits to prevent LLM attention overload and hallucination loops.
func (s *ReleaseService) generateChangelogByAreaChunked(areaGroups map[string][]string, nameMap map[string]string) (domain.ChangelogByArea, error) {
	result := make(domain.ChangelogByArea)
	const chunkSize = 25

	for groupKey, commits := range areaGroups {
		if len(commits) == 0 {
			continue
		}

		var allBullets []string
		for i := 0; i < len(commits); i += chunkSize {
			end := i + chunkSize
			if end > len(commits) {
				end = len(commits)
			}
			chunkCommits := commits[i:end]
			singleGroup := map[string][]string{groupKey: chunkCommits}
			formatted := FormatGroupedCommits(singleGroup)

			partial, err := s.llm.GenerateChangelogGrouped(formatted, nameMap, s.customMessage, "area")
			if err != nil {
				allBullets = append(allBullets, fmt.Sprintf("(could not generate for commits %d-%d: %v)", i+1, end, err))
				continue
			}

			for _, v := range partial {
				allBullets = append(allBullets, v...)
			}
		}

		if len(allBullets) > 0 {
			result[groupKey] = allBullets
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

// generateWithStacks produces a stack-grouped changelog when BaseBranch is configured.
// It groups CommitEntry values by StackID, calls the LLM once per group to infer a label,
// and assembles a markdown changelog with section headers per group.
// ProjectConfig.Areas is NOT used — the LLM infers labels from commit messages + context.
func (s *ReleaseService) generateWithStacks(commits string) (string, []string, bool, error) {
	// Use stored entries from Prepare() if available (they have stack metadata)
	// Otherwise fall back to parsing the raw commit string (no stack grouping possible)
	var filteredEntries []domain.CommitEntry
	if len(s.pendingEntries) > 0 {
		// Bug 5: Filter pendingEntries before grouping — exclude skipTypes (unless breaking) and excluded paths
		filteredEntries = filterEntriesForChangelog(s.pendingEntries, s.projectCfg)
	}

	// If no stack entries (e.g., on trunk with no entries), fall back to area-based
	if len(filteredEntries) == 0 {
		if len(commits) == 0 {
			return "", nil, false, nil
		}
		// No stack data available — fall back to area-based if possible
		if s.projectCfg != nil && len(s.projectCfg.Areas) > 0 {
			return s.generateWithAreas(commits)
		}
		return "", nil, false, nil
	}

	groups := domain.GroupByStackID(filteredEntries)

	// Build formatted groups and name map for LLM
	// For each stack group, format commit messages for the LLM prompt.
	// Stack groups use group_N keys for obfuscation, same as area groups.
	sortedKeys := make([]string, 0, len(groups))
	for k := range groups {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	formattedGroups := make(map[string][]string)
	nameMap := make(map[string]string)

	for i, key := range sortedKeys {
		groupKey := fmt.Sprintf("group_%d", i+1)
		// Use the StackBranch or key as the display label
		displayLabel := key
		entries := groups[key]
		if len(entries) > 0 && entries[0].StackBranch() != "" {
			displayLabel = entries[0].StackBranch()
		}
		nameMap[groupKey] = displayLabel
		var msgs []string
		for _, entry := range entries {
			msgs = append(msgs, entry.Message())
		}
		formattedGroups[groupKey] = msgs
	}

	formatted := FormatGroupedCommits(formattedGroups)

	changelog, err := s.llm.GenerateChangelogGrouped(formatted, nameMap, s.customMessage, "stack")
	if err != nil {
		// If LLM fails for a group, fall back to using branch names as headers
		var warnings []string
		warnings = append(warnings, fmt.Sprintf("LLM changelog generation failed: %v", err))
		// Build a simple changelog using branch names as section headers
		var sb strings.Builder
		for _, key := range sortedKeys {
			entries := groups[key]
			label := key
			if len(entries) > 0 && entries[0].StackBranch() != "" {
				label = entries[0].StackBranch()
			}
			sb.WriteString("## " + titleCase(label) + "\n")
			for _, entry := range entries {
				sb.WriteString("- " + entry.Message() + "\n")
			}
			sb.WriteString("\n")
		}
		return strings.TrimSpace(sb.String()), warnings, false, nil
	}

	// Remap group_N keys to display labels
	remapped := remapGroupKeys(changelog, nameMap)

	// If the LLM returned obfuscated or empty keys, replace them with
	// the display labels from nameMap. Since nameMap maps group_N → branch name,
	// any key still starting with "group_" after remapping means the LLM
	// didn't infer a good label — use the branch name instead.
	replacements := make(map[string][]string)
	for key, items := range remapped {
		if strings.HasPrefix(key, "group_") || key == "" {
			if label, ok := nameMap[key]; ok && label != "" {
				replacements[label] = items
				delete(remapped, key)
			}
		}
	}
	for k, v := range replacements {
		remapped[k] = v
	}

	md := formatChangelogByAreaMarkdown(remapped)
	return md, nil, false, nil
}
