package installer

import (
	_ "embed" // Required for //go:embed
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed prompts/agent-instructions.md
var embeddedInstructions string

// readInstructions returns embedded instructions.
// Uses //go:embed so it works from any location (installed binary).
func readInstructions() ([]byte, error) {
	return []byte(embeddedInstructions), nil
}

// RuleFile represents an agent's instruction/rules file.
type RuleFile struct {
	Name     string
	Path    string
	Content string
}

// GetRuleFiles returns the rule files to create for all agents.
func GetRuleFiles(binPath string) ([]RuleFile, error) {
	home := homeDir()

	content, err := readInstructions()
	if err != nil {
		return nil, fmt.Errorf("failed to read instructions: %w", err)
	}

	strContent := string(content)

	return []RuleFile{
		{
			Name:     "claude-code",
			Path:    filepath.Join(home, "CLAUDE.md"),
			Content: strContent,
		},
		{
			Name:     "cursor",
			Path:    filepath.Join(home, ".cursorrules"),
			Content: strContent,
		},
		{
			Name:     "windsurf",
			Path:    filepath.Join(home, ".windsurfrules"),
			Content: strContent,
		},
		{
			Name:     "cline",
			Path:    filepath.Join(home, ".clinerules", "rules.md"),
			Content: strContent,
		},
		{
			Name:     "zed",
			Path:    filepath.Join(home, ".zed", "rules.md"),
			Content: strContent,
		},
		{
			Name:     "codex",
			Path:    filepath.Join(home, "AGENTS.md"),
			Content: strContent,
		},
		{
			Name:     "opencode-skill",
			Path:    filepath.Join(home, ".config", "opencode", "skills", "git-courer", "SKILL.md"),
			Content: strContent,
		},
		{
			Name:     "opencode-plugin",
			Path:    filepath.Join(home, ".config", "opencode", "plugins", "git-courer.js"),
			Content: getPluginJS(binPath),
		},
		// Gemini CLI support
		{
			Name:     "gemini",
			Path:    filepath.Join(home, ".gemini", "GEMINI.md"),
			Content: strContent,
		},
	}, nil
}

func getPluginJS(binPath string) string {
	return `// Git-Courer plugin for OpenCode
exports.gitCourer = async ({ project, client, $, directory, worktree }) => {
  return {
    "tool.execute.before": async (input, output) => {
      // git-courer context hooks
    }
  };
};
`
}

// WriteRuleFiles writes agent rule files to the filesystem.
// It creates CLAUDE.md, .cursorrules, opencode skill, etc.
// If a file exists, it merges: keeps user customizations + adds new default content.
func WriteRuleFiles(binPath string) (int, error) {
	files, err := GetRuleFiles(binPath)
	if err != nil {
		return 0, fmt.Errorf("failed to get rule files: %w", err)
	}

	var written int
	for _, file := range files {
		// Check if file exists
		existing, err := os.ReadFile(file.Path)
		if err != nil {
			// File doesn't exist - create from scratch
			if err := os.MkdirAll(filepath.Dir(file.Path), 0755); err != nil {
				fmt.Fprintf(os.Stderr, "  ⚠ %s: failed to create dir: %v\n", file.Name, err)
				continue
			}
			if err := os.WriteFile(file.Path, []byte(file.Content), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "  ⚠ %s: %v\n", file.Name, err)
				continue
			}
			fmt.Printf("  ✓ %s created\n", file.Name)
			written++
			continue
		}

		// File exists - check if it's our default or user-customized
		existingStr := string(existing)
		if strings.HasPrefix(existingStr, "# Git-courer AI Assistant Rules") {
			// It's our default file - safe to overwrite with new version
			if existingStr != file.Content {
				if err := os.WriteFile(file.Path, []byte(file.Content), 0644); err != nil {
					fmt.Fprintf(os.Stderr, "  ⚠ %s: %v\n", file.Name, err)
					continue
				}
				fmt.Printf("  ✓ %s updated\n", file.Name)
				written++
			}
			continue
		}

		// User has custom content - merge: append new default to existing
		// This preserves user's customizations while adding our updates
		merged := existingStr + "\n\n---\n\n# Git-courer Updates\n" + file.Content
		if err := os.WriteFile(file.Path, []byte(merged), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠ %s: merge failed: %v\n", file.Name, err)
			continue
		}
		fmt.Printf("  ✓ %s merged (user content preserved)\n", file.Name)
		written++
	}

	return written, nil
}