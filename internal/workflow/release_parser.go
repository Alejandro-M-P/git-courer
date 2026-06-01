package workflow

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/blak0p/git-courer/internal/core/domain"
)

// parseReleaseIntent parses user's instruction using regex (NO LLM).
// Detects version number, bump type, and merge branch from natural language.
func parseReleaseIntent(instruction string, releasesList []string) *domain.ReleaseIntent {
	inst := strings.ToLower(strings.TrimSpace(instruction))

	// Detect bump type from instruction
	bump := ""
	if strings.Contains(inst, "major") || strings.Contains(inst, "romper") {
		bump = "major"
	} else if strings.Contains(inst, "minor") || strings.Contains(inst, "peque") ||
		strings.Contains(inst, "perf") { // perf es como minor
		bump = "minor"
	} else if strings.Contains(inst, "patch") || strings.Contains(inst, "fix") ||
		strings.Contains(inst, "hotfix") || strings.Contains(inst, "bugfix") {
		bump = "patch"
	}

	// Detect version from instruction (e.g., "v1.2.0", "2.0.0", "version 1.0")
	// Also captures optional prerelease suffix (e.g., "-rc.1", "-alpha.2")
	tagName := ""
	userSpecified := false
	versionRe := regexp.MustCompile(`v?(\d+)\.(\d+)\.(\d+)(-[a-zA-Z0-9.]+)?`)
	if match := versionRe.FindStringSubmatch(inst); match != nil {
		tagName = "v" + match[1] + "." + match[2] + "." + match[3]
		if match[4] != "" {
			tagName += match[4]
		}
		userSpecified = true
	}

	// If no version in instruction, calculate from releases
	if tagName == "" && len(releasesList) > 0 {
		// Get latest tag and calculate next version
		prevTag := releasesList[len(releasesList)-1] // already sorted
		if bump != "" {
			if newTag, err := domain.BumpVersion(prevTag, bump); err == nil {
				tagName = newTag
			}
		} else {
			// default to patch
			if newTag, err := domain.BumpVersion(prevTag, "patch"); err == nil {
				tagName = newTag
			}
		}
	}

	return &domain.ReleaseIntent{
		TagName:              tagName,
		IsRelease:            true,
		VersionBump:          bump,
		UserSpecifiedVersion: userSpecified,
	}
}

// parseSemver extracts major, minor, patch numbers from a semver tag.
// Handles both "v1.2.3" and "1.2.3" formats.
func parseSemver(tag string) (major, minor, patch int) {
	s := strings.TrimPrefix(tag, "v")
	parts := strings.Split(s, ".")
	if len(parts) >= 1 {
		major, _ = strconv.Atoi(strings.Split(parts[0], "-")[0])
	}
	if len(parts) >= 2 {
		minor, _ = strconv.Atoi(strings.Split(parts[1], "-")[0])
	}
	if len(parts) >= 3 {
		patch, _ = strconv.Atoi(strings.Split(parts[2], "-")[0])
	}
	return
}

// isStableTag returns true if the tag matches vMAJOR.MINOR.PATCH strictly — three dot-separated
// numbers with optional "v" prefix, and nothing else.
func isStableTag(tag string) bool {
	s := strings.TrimPrefix(tag, "v")
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if _, err := strconv.Atoi(p); err != nil {
			return false
		}
	}
	return true
}

// previousTag returns the tag immediately before target in semver-sorted order,
// considering only stable release tags (vMAJOR.MINOR.PATCH with no prerelease suffix).
// Returns empty string if target is the first tag or no stable tags exist.
func previousTag(tags []string, target string) string {
	var stable []string
	for _, t := range tags {
		if isStableTag(t) {
			stable = append(stable, t)
		}
	}
	if len(stable) == 0 {
		return ""
	}
	sort.Slice(stable, func(i, j int) bool {
		mi, mni, pi := parseSemver(stable[i])
		mj, mnj, pj := parseSemver(stable[j])
		if mi != mj {
			return mi < mj
		}
		if mni != mnj {
			return mni < mnj
		}
		return pi < pj
	})
	for i, t := range stable {
		if t == target && i > 0 {
			return stable[i-1]
		}
	}
	// target not in list — return the last stable tag as reference
	return stable[len(stable)-1]
}
