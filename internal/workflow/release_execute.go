package workflow

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/blak0p/git-courer/internal/core/domain"
)

const releaseChangelogFilename = "release_changelog.md"

// BuildPreview formats the release preview for user confirmation.
// Returns a formatted string for preview.
func (s *ReleaseService) BuildPreview(intent *domain.ReleaseIntent, changelog string) string {
	var b strings.Builder
	b.WriteString("📦 Release Preview\n")
	b.WriteString("==================\n\n")
	b.WriteString(fmt.Sprintf("Tag: %s\n", intent.TagName))
	if intent.VersionBump != "" {
		b.WriteString(fmt.Sprintf("Version Bump: %s\n", intent.VersionBump))
	}
	b.WriteString("\n--- Changelog ---\n")
	b.WriteString(changelog)
	b.WriteString("\n\n")
	b.WriteString("Do you want to proceed?")
	return b.String()
}

// Execute creates the git tag and pushes it to remote.
// Includes security checks:
// - Validate tag with IsValidTagName
// - Check TagExists before creating
// - Always push tag after creation
func (s *ReleaseService) Execute(intent *domain.ReleaseIntent, changelog string) (string, error) {

	// 1. Tag
	s.mu.Lock()
	if s.progressCb != nil {
		s.progressCb(2, 4)
	}
	s.mu.Unlock()

	// Security Check 1: Validate tag name
	if !domain.IsValidTagName(intent.TagName) {
		return "", fmt.Errorf("invalid tag name: %s (must be semver like v1.0.0 or 1.0.0)", intent.TagName)
	}

	// Security Check 2: Check if tag already exists
	exists, err := s.git.TagExists(intent.TagName)
	if err != nil {
		return "", fmt.Errorf("failed to check tag existence: %w", err)
	}
	if exists {
		return "", fmt.Errorf("tag %s already exists — check the proposed version", intent.TagName)
	}

	body := buildReleaseBody(intent.TagName, changelog)

	// Create git tag with changelog annotation or github release
	if s.cfg.ReleaseType == "github" {
		authed, authErr := s.git.IsGHAuthenticated()
		if authErr != nil || !authed {
			return "", fmt.Errorf("github release requested but gh CLI is not authenticated. Run 'gh auth login' first: %w", authErr)
		}
		_, err = s.git.CreateRelease(intent.TagName, body)
		if err != nil {
			return "", fmt.Errorf("failed to create github release: %w", err)
		}
	} else {
		// Write changelog to file for user editing and tag from file
		if err := s.writeChangelogFile(body); err != nil {
			return "", fmt.Errorf("failed to write changelog file: %w", err)
		}
		_, err = s.git.TagFromFile(intent.TagName, releaseChangelogFilename)
		if err != nil {
			return "", fmt.Errorf("failed to create tag: %w", err)
		}

		// 2. Push
		s.mu.Lock()
		if s.progressCb != nil {
			s.progressCb(3, 4)
		}
		s.mu.Unlock()

		// Push tag to remote — ALWAYS, not optional
		_, err = s.git.PushTag(intent.TagName)
		if err != nil {
			errStr := err.Error()
			// If tag already exists on remote (global), we can continue
			if !strings.Contains(errStr, "already exists") {
				return "", fmt.Errorf("failed to push tag: %w", err)
			}
		}
	}

	// Clear CommitStore after successful tag push (no-op if store is nil)
	if s.commitStore != nil {
		if err := s.commitStore.Clear(); err != nil {
			log.Printf("[WARN] Failed to clear CommitStore: %v", err)
		}
		// Clean up all branch directories after release
		if err := s.commitStore.RemoveAllBranchDirs(); err != nil {
			log.Printf("[WARN] Failed to remove branch directories: %v", err)
		}
	}

	result := ReleaseResult{
		Operation: "release",
		TagName:   intent.TagName,
		Changelog: body,
		Type:      "write",
		Message:   fmt.Sprintf("Tag %s created — changelog saved to release_changelog.md", intent.TagName),
	}

	resp, _ := json.Marshal(result)
	return string(resp), nil
}

func (s *ReleaseService) countLines(ss string) int {
	if ss == "" {
		return 0
	}
	return strings.Count(ss, "\n") + 1
}

func buildReleaseBody(tagName, changelog string) string {
	prefix := tagName + " — "
	if strings.HasPrefix(changelog, prefix) {
		return changelog
	}
	if strings.TrimSpace(changelog) == "" {
		return tagName
	}
	lines := strings.Split(changelog, "\n")
	idx := 0
	for idx < len(lines) && strings.TrimSpace(lines[idx]) == "" {
		idx++
	}
	if idx >= len(lines) {
		return tagName
	}
	return prefix + strings.Join(lines[idx:], "\n")
}

// writeChangelogFile writes the release body markdown to the changelog file
// so the user can edit it before tagging, and git tag -F can read it.
func (s *ReleaseService) writeChangelogFile(changelog string) error {
	path := releaseChangelogFilename
	if s.cfg.WorkDir != "" {
		path = filepath.Join(s.cfg.WorkDir, releaseChangelogFilename)
	}
	return os.WriteFile(path, []byte(changelog), 0644)
}
