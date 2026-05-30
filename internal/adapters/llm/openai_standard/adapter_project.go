package openai_standard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

const (
	projectInitTemp      = 0.1
	projectInitMaxTokens = 512
)

// ProjectInit analyzes the codebase and returns a suggested project description
// and area-scope mappings for project initialization.
// Makes two sequential LLM calls: description (from docs) then areas (from dir tree).
func (a *OpenAIStandardAdapter) ProjectInit(repoRoot string) (*domain.ProjectConfig, error) {
	description, err := a.getProjectDescription(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("project description: %w", err)
	}
	areas, err := a.getProjectAreas(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("project areas: %w", err)
	}
	return &domain.ProjectConfig{Description: description, Areas: areas}, nil
}

func (a *OpenAIStandardAdapter) getProjectDescription(repoRoot string) (string, error) {
	input := readDocContents(repoRoot)
	if input == "" {
		input = buildDirectoryTree(repoRoot)
	}
	prompt := "Summarize the following project documentation into a single concise description.\n\n" + input + "\n\nRules:\n- Use ONLY information from the documents above\n- Do NOT invent or assume project purpose\n- Do NOT reference the directory structure\n- Output ONLY this JSON — never wrap in markdown or add text:\n\n{\"description\": \"your one-sentence summary here\"}"
	result, err := a.chatCompletion(prompt, chatCompletionOpts{
		operation:       "project_description",
		jsonMode:        true,
		reasoningEffort: "none",
		temperature:     floatPtr(projectInitTemp),
		maxTokens:       256,
	})
	if err != nil {
		return "", err
	}
	var out struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		return "", fmt.Errorf("parse project_description: %w", err)
	}
	return out.Description, nil
}

func (a *OpenAIStandardAdapter) getProjectAreas(repoRoot string) (map[string][]string, error) {
	// TODO(file://Betterar_commits_y_releases): SDD-5 resolves areas in Go.
	// Return nil here — areas will be configured via config.Areas.
	return nil, nil
}

// readDocContents reads README.md, CONTRIBUTING.md, SECURITY.md, and docs/*.md,
// combining up to ~4KB with README prioritized.
func readDocContents(repoRoot string) string {
	const maxTotal = 4 * 1024
	var sections []string
	totalLen := 0

	for _, name := range []string{"README.md", "CONTRIBUTING.md", "SECURITY.md"} {
		data, err := os.ReadFile(filepath.Join(repoRoot, name))
		if err != nil || len(data) == 0 {
			continue
		}
		content := string(data)
		if totalLen+len(content) > maxTotal {
			content = content[:maxTotal-totalLen]
		}
		sections = append(sections, fmt.Sprintf("--- %s ---\n%s", name, content))
		totalLen += len(content)
		if totalLen >= maxTotal {
			break
		}
	}

	if totalLen < maxTotal {
		for _, path := range globFiles(filepath.Join(repoRoot, "docs"), "*.md") {
			data, err := os.ReadFile(path)
			if err != nil || len(data) == 0 {
				continue
			}
			content := string(data)
			if totalLen+len(content) > maxTotal {
				content = content[:maxTotal-totalLen]
			}
			sections = append(sections, fmt.Sprintf("--- %s ---\n%s", filepath.Base(path), content))
			totalLen += len(content)
			if totalLen >= maxTotal {
				break
			}
		}
	}

	return strings.Join(sections, "\n\n")
}

// buildDirectoryTree returns top-level and one-deep subdirectories, skipping hidden/noise dirs.
func buildDirectoryTree(repoRoot string) string {
	skip := map[string]bool{
		".git": true, "node_modules": true, "vendor": true,
		".cache": true, "__pycache__": true, "dist": true, "build": true,
	}
	var lines []string
	entries, err := os.ReadDir(repoRoot)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() || skip[e.Name()] || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		lines = append(lines, e.Name()+"/")
		subs, err := os.ReadDir(filepath.Join(repoRoot, e.Name()))
		if err != nil {
			continue
		}
		for _, sub := range subs {
			if !sub.IsDir() || skip[sub.Name()] || strings.HasPrefix(sub.Name(), ".") {
				continue
			}
			lines = append(lines, e.Name()+"/"+sub.Name()+"/")
		}
	}
	return strings.Join(lines, "\n")
}

func globFiles(dir, pattern string) []string {
	matches, _ := filepath.Glob(filepath.Join(dir, pattern))
	return matches
}
