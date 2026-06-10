package workflow

import (
	"fmt"
	"sort"
	"strings"

	"github.com/blak0p/git-courer/internal/core/domain"
)

// Generate filters commits, groups them, and translates to user-facing markdown.
// When BaseBranch is configured (non-empty), uses stack-based grouping.
// Otherwise, uses freeform LLM-based categorization.
// Always returns isBackground=false; the bool is kept for interface compatibility.
// If a custom message was set via SetCustomMessage, it is injected into the LLM prompt.
func (s *ReleaseService) Generate(commits string) (string, []string, bool, error) {
	// Stack-based path: when BaseBranch is configured, group by StackID
	if s.projectCfg != nil && s.projectCfg.BaseBranch != "" {
		return s.generateWithStacks(commits)
	}

	// Freeform path: LLM invents its own categories
	return s.generateFreeform(commits)
}

// generateFreeform produces a freeform LLM-categorized changelog.
// Filters commits, groups by scope, and lets the LLM invent category names.
// No predefined keys, no area concepts, no obfuscation.
func (s *ReleaseService) generateFreeform(commits string) (string, []string, bool, error) {
	filtered := filterForChangelog(commits, s.projectCfg)
	if len(filtered) == 0 {
		return "", nil, false, nil
	}

	formatted := FormatGroupedCommits(filtered)
	md, err := s.llm.GenerateChangelogGrouped(formatted, nil, s.customMessage, "freeform")
	if err != nil {
		return "", []string{err.Error()}, false, err
	}

	return md, nil, false, nil
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

	// If no stack entries (e.g., on trunk with no entries), fall back to freeform
	if len(filteredEntries) == 0 {
		if len(commits) == 0 {
			return "", nil, false, nil
		}
		// No stack data available — fall back to freeform
		return s.generateFreeform(commits)
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

	md, err := s.llm.GenerateChangelogGrouped(formatted, nameMap, s.customMessage, "stack")
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
			sb.WriteString("## " + label + "\n")
			for _, entry := range entries {
				sb.WriteString("- " + entry.Message() + "\n")
			}
			sb.WriteString("\n")
		}
		return strings.TrimSpace(sb.String()), warnings, false, nil
	}

	return md, nil, false, nil
}