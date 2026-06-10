package domain

// Changelog is the structured output from the changelog_generate LLM prompt.
type Changelog struct {
	Features []string `json:"features"`
	Fixes    []string `json:"fixes"`
	Breaking []string `json:"breaking"`
	Docs     []string `json:"docs"`
	Perf     []string `json:"perf"`
	Internal []string `json:"internal"`
}

// GroupByStackID groups CommitEntry values by their StackID.
// Entries with an empty StackID are placed in a single "Unspecified" group.
// Returns nil for nil or empty input.
func GroupByStackID(entries []CommitEntry) map[string][]CommitEntry {
	if len(entries) == 0 {
		return nil
	}

	groups := make(map[string][]CommitEntry)
	for _, entry := range entries {
		key := entry.StackID()
		if key == "" {
			key = "Unspecified"
		}
		groups[key] = append(groups[key], entry)
	}
	return groups
}
