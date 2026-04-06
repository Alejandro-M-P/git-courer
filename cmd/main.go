package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/git"
	ollama "github.com/Alejandro-M-P/git-courer/internal/adapters/llm"
	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/pkg/mcp"
)

func main() {
	// Check for setup command (per project)
	if len(os.Args) > 1 && os.Args[1] == "setup" {
		runSetup()
		return
	}

	// Check for install command (global)
	if len(os.Args) > 1 && os.Args[1] == "install" {
		fmt.Println("For global installation, please run the install.sh script:")
		fmt.Println("  curl -fsSL https://raw.githubusercontent.com/Alejandro-M-P/git-courer/main/install.sh | sh")
		fmt.Println("Or:")
		fmt.Println("  ./install.sh")
		return
	}

	// Check for version
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("git-courer v%s\n", config.Default().MCP.Version)
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
	fmt.Println("\nCerrando git-courer...")

	// Stop Ollama if we started it
	mcpServer.Stop()

	os.Exit(0)
}

// runSetup configures git-courer for the CURRENT PROJECT
func runSetup() {
	fmt.Println("git-courer Setup (PROJECT)")
	fmt.Println()

	dir, _ := os.Getwd()
	fmt.Printf("Project: %s\n", dir)
	fmt.Println()

	// Create git-courer.yaml LOCAL (overrides global config)
	createGitCourerConfig()

	// Add to .gitignore
	addToGitignore()

	fmt.Println()
	fmt.Println("Setup complete!")
	fmt.Println()
	fmt.Println("This creates a local git-courer.yaml that overrides")
	fmt.Println("global settings (model, context window, etc)")
	fmt.Println()
}

// createGitCourerConfig creates git-courer.yaml if it doesn't exist
func createGitCourerConfig() {
	if _, err := os.Stat("git-courer.yaml"); err == nil {
		fmt.Println("git-courer.yaml already exists")
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
		fmt.Printf("Error creating config: %v\n", err)
		return
	}

	fmt.Println("Created git-courer.yaml")

	// Add to .gitignore if needed
	addToGitignore()
}

// addToGitignore adds git-courer configs to .gitignore
func addToGitignore() {
	entries := []string{
		"git-courer.yaml",
	}

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
