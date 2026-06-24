package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/blak0p/git-courer/internal/adapters/commitstore"
	gitadapter "github.com/blak0p/git-courer/internal/adapters/git"
	llm "github.com/blak0p/git-courer/internal/adapters/llm"
	"github.com/blak0p/git-courer/internal/config"
	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/delivery/cli"
	mcpserver "github.com/blak0p/git-courer/internal/delivery/mcp"
	"github.com/blak0p/git-courer/internal/infra/chunkers"
	"github.com/blak0p/git-courer/internal/installer"
	"github.com/blak0p/git-courer/tui"
	"github.com/blak0p/git-courer/tui/screens"
)

// isTTY checks if running in an interactive terminal
func isTTY() bool {
	_, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	return err == nil
}

func main() {
	// Configure grammar cache early
	home, _ := os.UserHomeDir()
	cacheDir := filepath.Join(home, ".cache", "git-courer", "grammars")
	_ = chunkers.ConfigureGrammarCache(cacheDir)

	// Check for post-install hook (after go install)
	if os.Getenv(installer.PostInstallEnv) == "1" {
		if err := installer.RunPostInstall(); err != nil {
			fmt.Fprintf(os.Stderr, "Post-install failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// No args → launch TUI (default behavior)
	if len(os.Args) == 1 {
		runTUI()
		return
	}

	// Handle subcommands
	switch os.Args[1] {
	case "mcp":
		if len(os.Args) > 2 && os.Args[2] == "setup" {
			runMCPSetup()
		} else {
			runMCPServer()
		}
		return
	case "remove":
		runRemove()
		return
	case "uninstall":
		runUninstall()
		return
	case "restore":
		runRestore()
		return
	case "update":
		runUpdate()
		return
	case "init":
		runTUIInit()
		return
	case "release":
		runRelease(os.Args[2:])
		return
	case "hook-check":
		runHookCheck(os.Args[2:])
		return
	case "doctor":
		runDoctor()
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
	default:
		// Unknown command → show help
		fmt.Printf("Unknown command: %s\n\n", os.Args[1])
		showHelp()
		os.Exit(1)
	}
}

func showHelp() {
	fmt.Println("git-courer - Git assistant with MCP")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  git-courer                  # Launch interactive TUI")
	fmt.Println("  git-courer init             # Initialize project configuration")
	fmt.Println("  git-courer mcp              # Run MCP server")
	fmt.Println("  git-courer mcp setup        # Configure MCP clients")
	fmt.Println("  git-courer release          # Manage releases")
	fmt.Println("    start [flags]             # Preview version bump and changelog")
	fmt.Println("      --instruction <text>    Release instruction")
	fmt.Println("      --bump <type>           Force bump type (major, minor, patch)")
	fmt.Println("      --message <text>        Custom tag annotation message")
	fmt.Println("      --dry-run               Preview only, don't save")
	fmt.Println("    apply                     # Create and push release tag")
	fmt.Println("    abort                     # Discard pending release")
	fmt.Println("    regenerate [--feedback]   # Revise changelog with feedback")
	fmt.Println("  git-courer hook-check <cmd> # Classify a command (agent hook)")
	fmt.Println("  git-courer doctor           # Diagnose MCP client health")
	fmt.Println("  git-courer update           # Check for binary updates")
	fmt.Println("  git-courer uninstall        # Remove git-courer")
	fmt.Println("  git-courer restore          # Restore MCP configs from backups")
	fmt.Println("  git-courer version          # Show version")
}

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

func runRemove() {
	fmt.Println("Removing git-courer...")
	fmt.Println("  ✓ Project cleaned")
	fmt.Println("\n✓ git-courer removed from project!")
}

func runUninstall() {
	if err := installer.RunUninstall(); err != nil {
		fmt.Fprintf(os.Stderr, "Uninstall failed: %v\n", err)
		os.Exit(1)
	}
}

// runRestore handles the restore subcommand for CLI usage.
// It restores MCP client config backups and removes hooks/GIT_COURER.md
// without removing the binary or global config.
func runRestore() {
	cmd := cli.RestoreCommand{}
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func runUpdate() {
	force := len(os.Args) > 2 && os.Args[2] == "--force"

	fmt.Println("Checking for updates...")
	if err := installer.RunUpdate(force); err != nil {
		fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Update complete!")

	// Post-update: Reconfigure MCP
	binPath, _ := installer.FindBinaryPath()
	if configured, err := installer.ConfigureAllMCP(binPath); err == nil && configured > 0 {
		fmt.Printf("✓ %d MCP client(s) reconfigured\n", configured)
	}
}

func runMCPServer() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	gitAdapter := gitadapter.New(".")

	// Use the factory to create the LLM adapter based on config.
	llmAdapter, lifecycle, err := llm.NewLLMAdapter(llm.FactoryConfig{
		Provider:    cfg.LLM.Provider,
		BaseURL:     cfg.LLM.BaseURL,
		Model:       cfg.LLM.Model,
		NumParallel: cfg.LLM.NumParallel,
	})
	if err != nil {
		log.Fatalf("Failed to create LLM adapter: %v", err)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	srv := mcpserver.ServeWithAdapter(cfg, gitAdapter, llmAdapter, lifecycle)

	log.Printf("Starting git-courer v%s", config.ServerVersion)
	log.Printf("Working directory: .")
	log.Printf("LLM provider: %s", cfg.LLM.Provider)
	log.Printf("LLM model: %s", cfg.LLM.Model)

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

// runTUIInit launches the TUI init wizard for project configuration.
func runTUIInit() {
	gitAdapter := gitadapter.New(".")
	if err := screens.RunInitScreen(".", gitAdapter); err != nil {
		fmt.Fprintf(os.Stderr, "Init error: %v\n", err)
		os.Exit(1)
	}
}

func runTUI() {
	// Check if running in interactive terminal
	if !isTTY() {
		// No TTY available - run in non-interactive mode (show help)
		fmt.Println("git-courer - Interactive TUI")
		fmt.Println("")
		fmt.Println("Usage:")
		fmt.Println("  git-courer           # Launch TUI (requires terminal)")
		fmt.Println("  git-courer mcp      # Run MCP server")
		fmt.Println("  git-courer mcp setup # Configure MCP clients")
		fmt.Println("  git-courer update    # Check for updates")
		fmt.Println("  git-courer uninstall # Remove git-courer")
		fmt.Println("  git-courer version  # Show version")
		return
	}

	if err := tui.Run(80, 24); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}
}

// runRelease handles the release subcommand for CLI usage.
// It creates a branch-scoped commit store and dispatches to ReleaseCommand.
func runRelease(args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	gitAdapter := gitadapter.New(".")

	// Create branch-scoped commit store
	commitStore := commitstore.NewFilesystemCommitStore(".", gitAdapter)
	currentBranch, branchErr := gitAdapter.CurrentBranch()
	if branchErr == nil && currentBranch != "" {
		if err := commitStore.SetBranch(currentBranch); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to set branch store: %v\n", err)
		}
	}

	// Create LLM adapter
	llmAdapter, lifecycle, err := llm.NewLLMAdapter(llm.FactoryConfig{
		Provider:    cfg.LLM.Provider,
		BaseURL:     cfg.LLM.BaseURL,
		Model:       cfg.LLM.Model,
		NumParallel: cfg.LLM.NumParallel,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create LLM adapter: %v\n", err)
		os.Exit(1)
	}

	// Ensure LLM provider is available
	if lifecycle != nil {
		started, startupErr := lifecycle.EnsureRunning()
		if startupErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: LLM provider not available: %v\n", startupErr)
		} else if started {
			if preWarmErr := lifecycle.PreWarm(); preWarmErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to pre-warm model: %v\n", preWarmErr)
			}
		}
	}

	cmd := cli.NewReleaseCommand(gitAdapter, llmAdapter, cfg, commitStore, ".")
	if err := cmd.Run(args); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

// runHookCheck handles the hook-check subcommand for CLI usage.
// It classifies a shell command and emits the result as JSON on stdout.
func runHookCheck(args []string) {
	cmd := cli.HookCheckCommand{}
	if err := cmd.Run(args); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

// runDoctor handles the doctor subcommand for CLI usage.
// It runs installer diagnostics and emits them as JSON on stdout.
func runDoctor() {
	cmd := cli.DoctorCommand{}
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
