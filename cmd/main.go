package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	gitadapter "github.com/Alejandro-M-P/git-courer/internal/adapters/git"
	ollamaadapter "github.com/Alejandro-M-P/git-courer/internal/adapters/llm"
	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpserver "github.com/Alejandro-M-P/git-courer/internal/delivery/mcp"
	"github.com/Alejandro-M-P/git-courer/internal/infra/logging"
)

func main() {
	setupLogRotation()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "setup":
			runSetup()
			return
		case "install":
			fmt.Println("For global installation, please run the install.sh script:")
			fmt.Println("  curl -fsSL https://raw.githubusercontent.com/Alejandro-M-P/git-courer/main/install.sh | sh")
			return
		case "--version", "-v":
			fmt.Printf("git-courer v%s\n", config.Default().MCP.Version)
			return
		case "version":
			if len(os.Args) > 2 && os.Args[2] == "--predict" {
				runVersionPredict()
				return
			}
			fmt.Printf("git-courer v%s\n", config.Default().MCP.Version)
			return
		}
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	gitAdapter := gitadapter.New(cfg.Git.WorkDir)
	if !gitAdapter.IsRepo() {
		fmt.Fprintf(os.Stderr, "Error: %s is not a git repository\n", cfg.Git.WorkDir)
		os.Exit(1)
	}

	ollamaAdapter := ollamaadapter.New(cfg.Ollama.Host, cfg.Ollama.Model, cfg.Ollama.ModelsDir)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	srv := mcpserver.ServeWithAdapter(cfg, gitAdapter, ollamaAdapter, ollamaAdapter)

	log.Printf("Starting git-courer v%s", cfg.MCP.Version)
	log.Printf("Working directory: %s", cfg.Git.WorkDir)
	log.Printf("Ollama host: %s", cfg.Ollama.Host)
	log.Printf("Ollama model: %s", cfg.Ollama.Model)

	<-stop
	log.Println("Cerrando git-courer...")
	srv.Stop(ollamaAdapter)
	os.Exit(0)
}

func setupLogRotation() {
	writer, err := logging.NewRotatingLogWriter(".gcourer/log/git-courer.log", 20)
	if err != nil {
		return
	}
	log.SetOutput(writer)
}

// runVersionPredict prints what the next version would be based on commits since the last tag.
func runVersionPredict() {
	git := gitadapter.New(".")
	if !git.IsRepo() {
		fmt.Fprintln(os.Stderr, "Error: not a git repository")
		os.Exit(1)
	}
	latestTag, err := git.LatestTag()
	if err != nil || latestTag == "" {
		fmt.Fprintln(os.Stderr, "Error: no tags found in this repository")
		os.Exit(1)
	}
	commits, err := git.CommitsFromTag(latestTag)
	if err != nil || commits == "" {
		fmt.Printf("Current: %s — no new commits since last tag\n", latestTag)
		return
	}
	bump := domain.CalculateBump(strings.Split(commits, "\n"))
	nextTag, err := domain.BumpVersion(latestTag, bump)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error calculating next version: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Current: %s\nBump:    %s\nNext:    %s\n", latestTag, bump, nextTag)
}

func runSetup() {
	// Create .gcourer/config.yaml with defaults
	cfg := config.Default()
	if err := cfg.SaveGlobal(); err != nil {
		fmt.Fprintf(os.Stderr, "Setup failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ git-courer configured")
}
