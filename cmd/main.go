package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Alejandro-M-P/git-courer/internal/infra/git"
	ollama "github.com/Alejandro-M-P/git-courer/internal/infra/llm"
	"github.com/Alejandro-M-P/git-courer/internal/infra/mcp"
	"github.com/Alejandro-M-P/git-courer/internal/infra/config"
	"github.com/Alejandro-M-P/git-courer/internal/infra/logging"
	"github.com/Alejandro-M-P/git-courer/internal/app/setup"
)

func main() {
	// Setup rotating log file (last 20 lines)
	setupLogRotation()

	// Check for setup command (per project)
	if len(os.Args) > 1 && os.Args[1] == "setup" {
		setup.Run()
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

	// Create adapters (Composition Root)
	gitAdapter := git.NewExecAdapter(cfg.Git.WorkDir)
	if !gitAdapter.IsRepo() {
		fmt.Fprintf(os.Stderr, "Error: %s is not a git repository\n", cfg.Git.WorkDir)
		os.Exit(1)
	}

	ollamaAdapter := ollama.NewAdapter(cfg.Ollama.Host, cfg.Ollama.Model, cfg.Ollama.ModelsDir)

	// Setup graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// Start MCP server — full DI: server knows nothing about concrete adapters
	mcpServer := mcp.ServeWithAdapter(cfg, gitAdapter, ollamaAdapter, ollamaAdapter)

	log.Printf("Starting git-courer v%s", cfg.MCP.Version)
	log.Printf("Working directory: %s", cfg.Git.WorkDir)
	log.Printf("Ollama host: %s", cfg.Ollama.Host)
	log.Printf("Ollama model: %s", cfg.Ollama.Model)

	// Wait for shutdown signal
	<-stop
	log.Println("Cerrando git-courer...")

	// Stop Ollama if we started it
	mcpServer.Stop(ollamaAdapter)

	os.Exit(0)
}

func setupLogRotation() {
	logDir := ".gcourer/log"
	logFile := logDir + "/git-courer.log"

	writer, err := logging.NewRotatingLogWriter(logFile, 20)
	if err != nil {
		// If rotation setup fails, log to stderr only
		return
	}

	log.SetOutput(writer)
}
