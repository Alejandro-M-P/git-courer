package domain

import "strings"

// CommitTypeWeight returns the priority weight for a commit type string.
// This is the reverse mapping of LabelWeight: commit types like "feat", "fix",
// "refactor" map to their corresponding weight levels.
//
// Weight levels:
//   9 = feat       — new functionality
//   8 = fix        — bug fixes, error handling, signature changes
//   7 = refactor   — structural changes without behavior change
//   6 = chore/ci/docs — configuration, dependencies, CI, documentation
//   5 = test       — test-only changes
//   0 = unknown    — unrecognized types
func CommitTypeWeight(commitType string) int {
	switch commitType {
	case "feat":
		return 9
	case "fix":
		return 8
	case "refactor":
		return 7
	case "chore", "ci", "docs":
		return 6
	case "test":
		return 5
	default:
		return 0
	}
}

// configFilePatterns lists file patterns that indicate configuration/dependency changes.
var configFilePatterns = []string{
	"go.mod", "go.sum", "package.json", "package-lock.json",
	".toml", ".yaml", ".yml", "Dockerfile", "Makefile",
	".github/", ".cfg", ".conf", ".ini", ".env",
}

// docFilePatterns lists file patterns that indicate documentation changes.
var docFilePatterns = []string{
	".md", ".rst", ".txt", "README", "docs/",
}

// InferCommitType infers a conventional commit type from chunk content when
// the classifier returns an empty CommitType. This is a heuristic fallback
// that examines diff patterns and file names.
// Returns only the type string (e.g. "feat", "fix", "chore") — no "!" suffix.
func InferCommitType(chunk DiffChunk) string {
	// Priority 1: If CommitType is already set, return it (stripped of "!")
	if chunk.CommitType != "" {
		return strings.TrimSuffix(chunk.CommitType, "!")
	}

	// Priority 2: New file detection
	if strings.Contains(chunk.Diff, "new file mode") {
		return "feat"
	}

	// Priority 3: Deleted file detection
	if strings.Contains(chunk.Diff, "deleted file mode") {
		return "refactor"
	}

	// Priority 4b: Path-type detection via DefaultPathTypes
	// Fires before config-file patterns (priority 4) because path-type is more specific.
	// Only fires when ALL files match a single path type.
	if len(chunk.Files) > 0 {
		if pt := (&ProjectConfig{}).ResolvePathType(chunk.Files); pt != "" {
			if allFilesMatchPathType(chunk.Files, pt, DefaultPathTypes) {
				return pt
			}
		}
	}

	// Priority 4: Only config/deps files
	if len(chunk.Files) > 0 && allFilesMatch(chunk.Files, configFilePatterns) {
		return "chore"
	}

	// Priority 5: Only test files
	if len(chunk.Files) > 0 && allFilesMatch(chunk.Files, []string{"_test.", ".test.", "test_"}) {
		return "test"
	}

	// Priority 6: Only documentation files
	if len(chunk.Files) > 0 && allFilesMatch(chunk.Files, docFilePatterns) {
		return "docs"
	}

	// Priority 7: Any source modifications (non-empty diff, no new/deleted files)
	if chunk.Diff != "" {
		return "fix"
	}

	// Absolute fallback
	return "chore"
}

// allFilesMatch checks if every file in the list matches at least one pattern.
// Patterns are matched case-insensitively by substring containment.
func allFilesMatch(files []string, patterns []string) bool {
	for _, f := range files {
		lower := strings.ToLower(f)
		matched := false
		for _, p := range patterns {
			if strings.Contains(lower, strings.ToLower(p)) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// allFilesMatchPathType checks if ALL files match the given path type prefixes.
// Used by priority 5b to ensure unanimity before overriding other type detection.
func allFilesMatchPathType(files []string, typeName string, pathTypes map[string][]string) bool {
	prefixes, ok := pathTypes[typeName]
	if !ok {
		return false
	}
	for _, f := range files {
		matched := false
		for _, prefix := range prefixes {
			if strings.HasPrefix(f, prefix) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}