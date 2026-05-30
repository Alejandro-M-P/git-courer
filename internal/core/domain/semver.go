package domain

import (
	"fmt"
	"strconv"
	"strings"
)

// CalculateBump analyzes conventional commit messages and returns the required semver bump.
// Priority: major > minor > patch.
// Rules:
//   - Any commit with "!" before ":" in the type → "major"
//   - Any commit with "BREAKING CHANGE" in the body → "major"
//   - Any commit with "feat:" or "feat(" prefix → "minor"
//   - All other commits → "patch"
func CalculateBump(messages []string) string {
	bump := "patch"
	for _, msg := range messages {
		msg = strings.TrimSpace(msg)
		if msg == "" {
			continue
		}
		if isMajorCommit(msg) {
			return "major"
		}
		if bump != "major" && isMinorCommit(msg) {
			bump = "minor"
		}
	}
	return bump
}

func isMajorCommit(msg string) bool {
	if strings.Contains(msg, "BREAKING CHANGE") {
		return true
	}
	colonIdx := strings.Index(msg, ":")
	if colonIdx > 0 && strings.Contains(msg[:colonIdx], "!") {
		return true
	}
	return false
}

func isMinorCommit(msg string) bool {
	return strings.HasPrefix(msg, "feat:") ||
		strings.HasPrefix(msg, "feat(") ||
		strings.Contains(msg, " feat:") ||
		strings.Contains(msg, " feat(") ||
		strings.HasPrefix(msg, "perf:") || // perf counts as minor
		strings.HasPrefix(msg, "perf(")
}

// BumpVersion applies the given bump to currentTag and returns the new version string.
// currentTag must be in "vMAJOR.MINOR.PATCH" or "MAJOR.MINOR.PATCH" format, optionally with a
// prerelease suffix (e.g., "-alpha", "-rc.1").
// The returned tag preserves the "v" prefix and prerelease suffix if present.
func BumpVersion(currentTag string, bump string) (string, error) {
	prefix := ""
	s := currentTag
	if strings.HasPrefix(currentTag, "v") {
		prefix = "v"
		s = currentTag[1:]
	}

	// Extract prerelease suffix before parsing (e.g., "-alpha", "-rc.1")
	prerelease := ""
	if idx := strings.Index(s, "-"); idx != -1 {
		prerelease = s[idx:] // includes the dash
		s = s[:idx]
	}

	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid semver tag %q: expected vMAJOR.MINOR.PATCH", currentTag)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", fmt.Errorf("invalid major in tag %q: %w", currentTag, err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid minor in tag %q: %w", currentTag, err)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", fmt.Errorf("invalid patch in tag %q: %w", currentTag, err)
	}

	switch bump {
	case "major":
		major++
		minor = 0
		patch = 0
	case "minor":
		minor++
		patch = 0
	case "patch":
		patch++
	default:
		return "", fmt.Errorf("unknown bump type %q: expected major, minor, or patch", bump)
	}

	return fmt.Sprintf("%s%d.%d.%d%s", prefix, major, minor, patch, prerelease), nil
}
