package workflow

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/blak0p/git-courer/internal/core/domain"
)

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

	// Create git tag with changelog annotation or github release
	if s.cfg.ReleaseType == "github" {
		authed, authErr := s.git.IsGHAuthenticated()
		if authErr != nil || !authed {
			return "", fmt.Errorf("github release requested but gh CLI is not authenticated. Run 'gh auth login' first")
		}
		_, err = s.git.CreateRelease(intent.TagName, changelog)
		if err != nil {
			return "", fmt.Errorf("failed to create github release: %w", err)
		}
	} else {
		// Create git tag with changelog annotation (always uses LLM-generated changelog)
		tagMessage := changelog
		_, err = s.git.Tag(intent.TagName, tagMessage)
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
		Changelog: changelog,
		Type:      "write",
		Message:   fmt.Sprintf("Tag %s created", intent.TagName),
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
