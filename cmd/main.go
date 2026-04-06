package main

// Test change v5 for git-courer commit message testing
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
	// Setup rotating log file (last 20 lines)
	setupLogRotation()

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
	ollamaAdapter := ollama.NewAdapter(cfg.Ollama.Host, cfg.Ollama.Model, cfg.Ollama.ModelsDir)

	// CRITICAL: Start Ollama BEFORE MCP server accepts requests
	// This prevents timeout when OpenCode immediately tries to use git_do
	log.Println("Ensuring Ollama is running...")
	if _, err := ollamaAdapter.EnsureOllama(); err != nil {
		log.Printf("Warning: Could not start Ollama: %v", err)
	} else {
		log.Println("Ollama is running")
		// Pre-warm model so first request is fast
		if err := ollamaAdapter.PreWarm(); err != nil {
			log.Printf("Warning: Could not pre-warm model: %v", err)
		} else {
			log.Println("Model ready")
		}
	}

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

// createGitCourerConfig creates git-courer/config.yaml if it doesn't exist
func createGitCourerConfig() {
	os.MkdirAll(".gcourer", 0755)
	configPath := ".gcourer/config.yaml"

	if _, err := os.Stat(configPath); err == nil {
		fmt.Println(".gcourer/config.yaml already exists")
		return
	}

	config := `# git-courer configuration
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

	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		fmt.Printf("Error creating config: %v\n", err)
		return
	}

	fmt.Println("Created " + configPath)

	// Add to .gitignore if needed
	addToGitignore()
}

// addToGitignore adds git-courer configs to .gitignore
func addToGitignore() {
	entries := []string{
		".gcourer/",
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

// RotatingLogWriter implements log rotation (max 20 lines)
type RotatingLogWriter struct {
	path     string
	maxLines int
	lines    []string
}

func (w *RotatingLogWriter) Write(p []byte) (n int, err error) {
	w.lines = append(w.lines, string(p))
	if len(w.lines) > w.maxLines {
		w.lines = w.lines[len(w.lines)-w.maxLines:]
	}
	os.WriteFile(w.path, []byte(strings.Join(w.lines, "")), 0644)
	return len(p), nil
}

func setupLogRotation() {
	logDir := ".gcourer/log"
	logFile := logDir + "/git-courer.log"

	// Create log directory
	os.MkdirAll(logDir, 0755)

	// Try to read existing log
	var lines []string
	if data, err := os.ReadFile(logFile); err == nil {
		lines = strings.Split(string(data), "\n")
		// Keep only last maxLines
		if len(lines) > 20 {
			lines = lines[len(lines)-20:]
		}
	}

	writer := &RotatingLogWriter{
		path:     logFile,
		maxLines: 20,
		lines:    lines,
	}

	log.SetOutput(writer)
}
