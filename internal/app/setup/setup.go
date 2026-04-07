// Package setup provides project setup: config creation and gitignore management.
package setup

import (
	"fmt"
	"os"
	"strings"
)

// Run configures git-courer for the current project.
func Run() {
	fmt.Println("git-courer Setup (PROJECT)")
	fmt.Println()

	dir, _ := os.Getwd()
	fmt.Printf("Project: %s\n", dir)
	fmt.Println()

	createConfig()
	addToGitignore()

	fmt.Println()
	fmt.Println("Setup complete!")
	fmt.Println()
	fmt.Println("This creates a local .gcourer/config.yaml that overrides")
	fmt.Println("global settings (model, context window, etc)")
	fmt.Println()
}

func createConfig() {
	os.MkdirAll(".gcourer", 0755)
	configPath := ".gcourer/config.yaml"

	if _, err := os.Stat(configPath); err == nil {
		fmt.Println(".gcourer/config.yaml already exists")
		return
	}

	content := `# git-courer configuration
# This file is specific to each project

ollama:
  host: http://localhost:11434
  model: qwen3.5
  context_window: 0
  auto_start: false
  models_dir: ""

git:
  workdir: .
  auto_add_secrets: true
  require_clean_repo: false

validation:
  require_confirmation: true
  max_commit_length: 500

secrets:
  detection_mode: regex+ai
  patterns: []

ui:
  theme: dark
  show_icons: true

mcp:
  name: git-courer
  version: ""
`

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		fmt.Printf("Error creating config: %v\n", err)
		return
	}

	fmt.Println("Created " + configPath)
}

func addToGitignore() {
	entries := []string{".gcourer/"}

	gitignore := ".gitignore"
	content, err := os.ReadFile(gitignore)
	if err != nil {
		if os.IsNotExist(err) {
			content = []byte{}
		} else {
			return
		}
	}

	existing := string(content)

	for _, entry := range entries {
		if !contains(existing, entry) {
			existing += "\n" + entry
		}
	}

	if err := os.WriteFile(gitignore, []byte(existing), 0644); err != nil {
		return
	}

	fmt.Println("Added to .gitignore")
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
