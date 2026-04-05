package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/git"
	ollama "github.com/Alejandro-M-P/git-courer/internal/adapters/llm"
	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/pkg/mcp"
	"gopkg.in/yaml.v3"
)

func main() {
	// Check for setup command
	if len(os.Args) > 1 && os.Args[1] == "setup" {
		runSetup()
		return
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Verify it's a git repository
	gitAdapter := git.NewExecAdapter(cfg.Git.WorkDir)
	if !gitAdapter.IsRepo() {
		fmt.Fprintf(os.Stderr, "Error: %s is not a git repository\n", cfg.Git.WorkDir)
		os.Exit(1)
	}

	// Create Ollama adapter (lazy start - won't start until first use)
	ollamaAdapter := ollama.NewAdapter(cfg.Ollama.Host, cfg.Ollama.Model)

	// Setup graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// Start MCP server with adapter
	mcpServer := mcp.ServeWithAdapter(cfg, ollamaAdapter)

	log.Printf("Starting git-courer v%s", cfg.MCP.Version)
	log.Printf("Working directory: %s", cfg.Git.WorkDir)
	log.Printf("Ollama host: %s", cfg.Ollama.Host)
	log.Printf("Ollama model: %s", cfg.Ollama.Model)

	// Wait for shutdown signal
	<-stop
	fmt.Println("\n🛑 Cerrando git-courer...")

	// Stop Ollama if we started it
	mcpServer.Stop()

	os.Exit(0)
}

// runSetup automatically configures git-courer for the current project
func runSetup() {
	fmt.Println("╔════════════════════════════════════════╗")
	fmt.Println("║     git-courer Setup                   ║")
	fmt.Println("║     Auto-configuration                  ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println()

	dir, _ := os.Getwd()
	fmt.Printf("Project: %s\n", dir)
	fmt.Println()

	// Detect which tools are installed
	detectedTools := detectTools()

	// Create git-courer.yaml if it doesn't exist
	createGitCourerConfig()

	// Generate config for each detected tool
	for _, tool := range detectedTools {
		generateToolConfig(tool)
		addToolRules(tool)

		// Add git-agent to opencode.json
		if tool == "opencode" {
			addGitAgentToOpencode()
		}
	}

	fmt.Println()

	// Add to .gitignore
	addToGitignore()

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════")
	fmt.Println("  Setup complete!")
	fmt.Println("═══════════════════════════════════════════")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("1. Restart your AI tool (Opencode/Claude/Cursor)")
	fmt.Println("2. Run: git-courer")
	fmt.Println()
}

// detectTools finds which AI tools are installed
func detectTools() []string {
	var tools []string

	// Check Opencode
	if _, err := os.Stat(".opencode/config.json"); err == nil {
		tools = append(tools, "opencode")
	}

	// Check Claude Code
	if _, err := os.Stat(".claude/settings.json"); err == nil {
		tools = append(tools, "claude")
	}

	// Check Cursor
	if _, err := os.Stat(".cursor/mcp.json"); err == nil {
		tools = append(tools, "cursor")
	}

	// Check for global configs
	homeDir := os.Getenv("HOME")
	if homeDir != "" {
		if _, err := os.Stat(homeDir + "/.config/opencode/opencode.json"); err == nil {
			if !contains(tools, "opencode") {
				tools = append(tools, "opencode")
			}
		}
	}

	return tools
}

// createGitCourerConfig creates git-courer.yaml if it doesn't exist
func createGitCourerConfig() {
	if _, err := os.Stat("git-courer.yaml"); err == nil {
		fmt.Println("✓ git-courer.yaml already exists")
		return
	}

	config := `# git-courer configuration
# This file is specific to each project

ollama:
  host: http://localhost:11434
  model: llama3.2

git:
  workdir: .

validation:
  require_confirmation: true

ui:
  theme: dark
  show_icons: true
`

	if err := os.WriteFile("git-courer.yaml", []byte(config), 0644); err != nil {
		fmt.Printf("✗ Error creating config: %v\n", err)
		return
	}

	fmt.Println("✓ Created git-courer.yaml")

	// Add to .gitignore if needed
	addToGitignore()
}

// addToGitignore adds git-courer configs to .gitignore
func addToGitignore() {
	entries := []string{
		"# git-courer config",
		"git-courer.yaml",
		"",
		"# MCP configs",
		".opencode/",
		".claude/",
		".cursor/",
	}

	gitignore := ".gitignore"
	content, err := os.ReadFile(gitignore)
	if err != nil {
		// .gitignore doesn't exist, create it
		if os.IsNotExist(err) {
			content = []byte{}
		} else {
			return
		}
	}

	existing := string(content)

	for _, entry := range entries {
		if entry == "" {
			continue
		}
		// Skip comments and check if entry exists
		if !strings.Contains(existing, entry) {
			existing += "\n" + entry
		}
	}

	if err := os.WriteFile(gitignore, []byte(existing), 0644); err != nil {
		return
	}

	fmt.Println("✓ Added to .gitignore")
}

// addToolRules adds git-courer rules to AI tool config files
func addToolRules(tool string) {
	var configFile string
	switch tool {
	case "opencode":
		configFile = "AGENTS.md"
	case "claude":
		configFile = "CLAUDE.md"
	}

	if configFile == "" {
		return
	}

	if _, err := os.Stat(configFile); err != nil {
		return
	}

	rules := `

## Git Operations

**For ALL git operations, ALWAYS use the MCP tools from git-courer:**
- git_status, git_diff, git_log, git_add, git_commit, git_push, git_pull, git_branch, git_checkout, git_stash, git_reset, git_ai_commit

**NEVER execute git directly with bash.** Always delegate to git-courer MCP tools.

This saves tokens on cloud AI by delegating all git work to the local git-courer server.

**When user asks for git operations (commit, push, etc):**
1. Use task() to delegate to git-agent subagent: task(subagent_type="git-agent", prompt="user request here")
2. Return ONLY the result from git-agent — no explanation, no description
3. Response should be ONE LINE only (e.g., "✅ Committed: feat: add feature [local-ollama]" or just the tool output)

**MINIMAL OUTPUT RULE**: Zero tokens for git responses. Just output the result.`

	// Check if rules already exist
	existing, _ := os.ReadFile(configFile)
	if strings.Contains(string(existing), "git-courer") {
		return
	}

	// Append rules
	f, err := os.OpenFile(configFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	if _, err := f.WriteString(rules); err != nil {
		return
	}

	fmt.Printf("✓ Added rules to %s\n", configFile)
}

// generateToolConfig creates config for each AI tool
func generateToolConfig(tool string) {
	var config string
	var path string
	var dir string

	switch tool {
	case "opencode":
		dir = ".opencode"
		path = dir + "/config.json"
		config = `{
  "mcp": {
    "git-courer": {
      "type": "local",
      "command": ["git-courer"]
    }
  }
}`
	case "claude":
		dir = ".claude"
		path = dir + "/settings.json"
		config = `{
  "mcpServers": {
    "git-courer": {
      "command": "git-courer"
    }
  }
}`
	case "cursor":
		dir = ".cursor"
		path = dir + "/mcp.json"
		config = `{
  "mcpServers": {
    "git-courer": {
      "command": "git-courer"
    }
  }
}`
	}

	if dir != "" {
		os.MkdirAll(dir, 0755)
	}

	if path != "" {
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("✓ %s config already exists\n", tool)
			return
		}

		if err := os.WriteFile(path, []byte(config), 0644); err != nil {
			fmt.Printf("✗ Error creating %s config: %v\n", tool, err)
			return
		}

		fmt.Printf("✓ Created %s config\n", tool)
	}
}

// addGitAgentToOpencode adds git-agent to opencode.json using the model from git-courer.yaml
func addGitAgentToOpencode() {
	path := "opencode.json"

	// Load config to get the model
	cfg := loadConfigForSetup()
	model := cfg.Ollama.Model
	if model == "" {
		model = "llama3.2" // default
	}

	// Convert model to opencode format (ollama/model-name)
	ollamaModel := "ollama/" + model

	if _, err := os.Stat(path); err != nil {
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	// Check if already has git-agent
	if strings.Contains(string(data), "git-agent") {
		return
	}

	// Parse JSON
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return
	}

	// Add provider for Ollama if not exists
	if _, ok := config["provider"]; !ok {
		config["provider"] = map[string]interface{}{
			"ollama": map[string]interface{}{
				"npm":  "@ai-sdk/openai-compatible",
				"name": "Ollama (local)",
				"options": map[string]interface{}{
					"baseURL": "http://localhost:11434/v1",
				},
				"models": map[string]interface{}{
					model: map[string]interface{}{
						"name": model + " (local)",
					},
				},
			},
		}
	}

	// Add git-agent
	if _, ok := config["agent"]; !ok {
		config["agent"] = map[string]interface{}{}
	}

	agentMap := config["agent"].(map[string]interface{})
	agentMap["git-agent"] = map[string]interface{}{
		"description": "Git operations with local Ollama - zero cloud tokens",
		"hidden":      true,
		"mode":        "subagent",
		"model":       ollamaModel,
		"prompt":      "You are a minimal git execution agent. Use git_courer_git_* MCP tools from git-courer MCP server. Rules: 1) NO thinking, NO explanations, NO text before/after. 2) Reply ONE LINE ONLY with the tool output. 3) When user says commit, use git_ai_commit to generate message with local Ollama. 4) Return just the result like '✅ Committed: feat: add feature [local-ollama]' or error.",
		"tools": map[string]interface{}{
			"git_courer_git_status":    true,
			"git_courer_git_diff":      true,
			"git_courer_git_log":       true,
			"git_courer_git_add":       true,
			"git_courer_git_commit":    true,
			"git_courer_git_ai_commit": true,
			"git_courer_git_push":      true,
			"git_courer_git_pull":      true,
			"git_courer_git_branch":    true,
			"git_courer_git_checkout":  true,
			"git_courer_git_stash":     true,
			"git_courer_git_reset":     true,
		},
	}

	// Add permission for task delegation
	if _, ok := config["permission"]; !ok {
		config["permission"] = map[string]interface{}{}
	}
	permMap := config["permission"].(map[string]interface{})
	if _, ok := permMap["task"]; !ok {
		permMap["task"] = map[string]interface{}{}
	}
	taskMap := permMap["task"].(map[string]interface{})
	taskMap["*"] = "deny"
	taskMap["git-*"] = "allow"

	// Write back
	output, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return
	}

	if err := os.WriteFile(path, output, 0644); err != nil {
		return
	}

	fmt.Printf("✓ Added git-agent to opencode.json (model: %s)\n", model)
}

// loadConfigForSetup loads config from global and local yaml files
func loadConfigForSetup() *config.Config {
	// Try local first
	if data, err := os.ReadFile("git-courer.yaml"); err == nil {
		cfg := &config.Config{}
		if err := yaml.Unmarshal(data, cfg); err == nil {
			return cfg
		}
	}

	// Then global
	globalPath := filepath.Join(os.Getenv("HOME"), ".config", "git-courer.yaml")
	if data, err := os.ReadFile(globalPath); err == nil {
		cfg := &config.Config{}
		if err := yaml.Unmarshal(data, cfg); err == nil {
			return cfg
		}
	}

	return config.Default()
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
