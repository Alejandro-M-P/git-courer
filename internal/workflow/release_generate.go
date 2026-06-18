package workflow

// Generate filters commits, groups them, and translates to user-facing markdown.
// Uses freeform LLM-based categorization — the LLM invents its own categories.
// Always returns isBackground=false; the bool is kept for interface compatibility.
// If a custom message was set via SetCustomMessage, it is injected into the LLM prompt.
func (s *ReleaseService) Generate(commits string) (string, []string, bool, error) {
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