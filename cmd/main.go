package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	gitadapter "github.com/Alejandro-M-P/git-courer/internal/adapters/git"
	llm "github.com/Alejandro-M-P/git-courer/internal/adapters/llm"
	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpserver "github.com/Alejandro-M-P/git-courer/internal/delivery/mcp"
	"github.com/Alejandro-M-P/git-courer/internal/infra/logging"
	"github.com/Alejandro-M-P/git-courer/internal/installer"
)

func main() {
	setupLogRotation()

	// Check for post-install hook (after go install)
	if os.Getenv(installer.PostInstallEnv) == "1" {
		if err := installer.RunPostInstall(); err != nil {
			fmt.Fprintf(os.Stderr, "Post-install failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "setup":
			runSetup()
			return
		case "remove":
			runRemove()
			return
		case "uninstall":
			runUninstall()
			return
		case "update":
			runUpdate()
			return
		case "mcp":
			if len(os.Args) > 2 && os.Args[2] == "setup" {
				runMCPSetup()
			} else {
				runMCPServer()
			}
			return
		case "--version", "-v":
			fmt.Printf("git-courer v%s\n", config.ServerVersion)
			return
		case "version":
			if len(os.Args) > 2 && os.Args[2] == "--predict" {
				runVersionPredict()
				return
			}
			fmt.Printf("git-courer v%s\n", config.ServerVersion)
			return
		}
	}

	runMCPServer()
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
	projectDir := "."
	if len(os.Args) > 2 {
		projectDir = os.Args[2]
	}

	if err := installer.RunSetup(projectDir); err != nil {
		fmt.Fprintf(os.Stderr, "Setup failed: %v\n", err)
		os.Exit(1)
	}
}

func runRemove() {
	projectDir := "."
	if len(os.Args) > 2 {
		projectDir = os.Args[2]
	}

	if err := installer.RunRemove(projectDir); err != nil {
		fmt.Fprintf(os.Stderr, "Remove failed: %v\n", err)
		os.Exit(1)
	}
}

func runUninstall() {
	if err := installer.RunUninstall(); err != nil {
		fmt.Fprintf(os.Stderr, "Uninstall failed: %v\n", err)
		os.Exit(1)
	}
}

func runUpdate() {
	force := len(os.Args) > 2 && os.Args[2] == "--force"
	if err := installer.RunUpdate(force); err != nil {
		fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
		os.Exit(1)
	}
}

func runMCPServer() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	gitAdapter := gitadapter.New(cfg.Git.WorkDir)

	// Use the factory to create the LLM adapter based on resolved config.
	resolvedCfg, err := cfg.ResolveLLMConfig()
	if err != nil {
		log.Fatalf("Failed to resolve LLM config: %v", err)
	}
	llmAdapter, lifecycle, err := llm.NewLLMAdapter(llm.FactoryConfig{
		Provider:      resolvedCfg.Provider,
		BaseURL:       resolvedCfg.BaseURL,
		Model:         resolvedCfg.Model,
		APIKey:        resolvedCfg.APIKey,
		ContextWindow: resolvedCfg.ContextWindow,
		NumParallel:   resolvedCfg.NumParallel,
		Operations:    resolvedCfg.Operations,
		Ollama:        resolvedCfg.Ollama,
	})
	if err != nil {
		log.Fatalf("Failed to create LLM adapter: %v", err)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	srv := mcpserver.ServeWithAdapter(cfg, gitAdapter, llmAdapter, lifecycle)

	log.Printf("Starting git-courer v%s", config.ServerVersion)
	log.Printf("Working directory: %s", cfg.Git.WorkDir)
	log.Printf("LLM provider: %s", resolvedCfg.Provider)
	log.Printf("LLM model: %s", resolvedCfg.Model)

	<-stop
	log.Println("Cerrando git-courer...")
	srv.Stop()
	os.Exit(0)
}

func runMCPSetup() {
	// Setup MCP for specific client or all
	clientName := ""
	if len(os.Args) > 3 {
		clientName = os.Args[3]
	}

	binPath, err := installer.FindBinaryPath()
	if err != nil {
		binPath = "git-courer"
	}

	if clientName != "" {
		if err := installer.SetupClient(clientName, binPath); err != nil {
			fmt.Fprintf(os.Stderr, "MCP setup failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ %s configured\n", clientName)
	} else {
		configured, err := installer.ConfigureAllMCP(binPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "MCP setup failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ %d MCP client(s) configured\n", configured)
	}
}
