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

// ChangelogByArea is the area-keyed output of the v2 changelog generator.
// Keys are area names (from commit scopes); "general" holds scopeless commits.
// Values are translated, user-facing descriptions.
type ChangelogByArea map[string][]string
