package workflow

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
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
	s.taskLog.logStart()

	// Security Check 1: Validate tag name
	if !domain.IsValidTagName(intent.TagName) {
		s.taskLog.logError(fmt.Sprintf("invalid tag name: %s", intent.TagName))
		return "", fmt.Errorf("invalid tag name: %s (must be semver like v1.0.0 or 1.0.0)", intent.TagName)
	}

	// Security Check 2: Check if tag already exists
	exists, err := s.git.TagExists(intent.TagName)
	if err != nil {
		s.taskLog.logError(fmt.Sprintf("failed to check tag existence: %v", err))
		return "", fmt.Errorf("failed to check tag existence: %w", err)
	}
	if exists {
		 s.taskLog.logError(fmt.Sprintf("tag already exists: %s", intent.TagName))
		return "", fmt.Errorf("tag %s already exists — check the proposed version", intent.TagName)
	}

	// Create git tag with changelog annotation
	_, err = s.git.Tag(intent.TagName, changelog)
	if err != nil {
		s.taskLog.logError(fmt.Sprintf("failed to create tag: %v", err))
		return "", fmt.Errorf("failed to create tag: %w", err)
	}
	s.taskLog.logTag(intent.TagName)

	// Push tag to remote — ALWAYS, not optional
	_, err = s.git.PushTag(intent.TagName)
	if err != nil {
		errStr := err.Error()
		// If tag already exists on remote (global), we can continue
		if strings.Contains(errStr, "already exists") {
			s.taskLog.logTag(intent.TagName + " (already remote)")
		} else {
			s.taskLog.logError(fmt.Sprintf("failed to push tag: %v", err))
			return "", fmt.Errorf("failed to push tag: %w", err)
		}
	}

	s.taskLog.logDone()

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

// DetectBranchFlow detects if the repository uses git flow.
// Returns the detected branch flow pattern: "gitflow", "trunk", or "unknown".
func (s *ReleaseService) DetectBranchFlow() (string, error) {
	branches, err := s.git.ListBranches()
	if err != nil {
		return "unknown", err
	}

	hasDevelop := strings.Contains(branches, "develop")
	hasDev := strings.Contains(branches, "dev")
	hasMain := strings.Contains(branches, "main")
	hasMaster := strings.Contains(branches, "master")

	// Git Flow: has develop and main/master
	if (hasDevelop || hasDev) && (hasMain || hasMaster) {
		return "gitflow", nil
	}

	// Trunk-based: only main/master
	if hasMain || hasMaster {
		return "trunk", nil
	}

	return "unknown", nil
}

func (s *ReleaseService) countLines(ss string) int {
	if ss == "" {
		return 0
	}
	return strings.Count(ss, "\n") + 1
}

// PrepareAndGenerateAsync runs the full Prepare+Generate flow in a goroutine.
// If userBump is provided, use it instead of LLM's proposal.
// Returns immediately — the caller should check LoadState() and LoadIntent()/LoadChangelog() for results.
func (s *ReleaseService) PrepareAndGenerateAsync(instruction string, userBump string) {
	s.setPendingState("processing")

	go func() {
		intent, commits, _, err := s.Prepare(instruction, userBump)
		if err != nil {
			s.taskLog.logError(fmt.Sprintf("background prepare failed: %v", err))
			s.setPendingState("error: " + err.Error())
			return
		}

		s.setIntent(intent)

		if !intent.IsRelease || commits == "" {
			s.setPendingState("")
			return
		}

		changelog, _, err := s.Generate(commits)
		if err != nil {
			s.taskLog.logError(fmt.Sprintf("background generate failed: %v", err))
			s.setPendingState("error: " + err.Error())
			return
		}

		if strings.HasPrefix(strings.TrimSpace(changelog), "{") {
			return
		}

		s.setChangelog(changelog)
		s.setPendingState("")
	}()
}