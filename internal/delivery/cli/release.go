// Package cli provides CLI delivery adapters for git-courer subcommands.
package cli

import (
	"fmt"
	"os"

	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/workflow"
)

// ReleaseSvc abstracts the release service methods used by ReleaseCommand.
// This keeps the command testable without requiring a full workflow.ReleaseService.
type ReleaseSvc interface {
	Prepare(instruction, userBump string) (*domain.ReleaseIntent, string, []string, error)
	Generate(commits string) (string, []string, bool, error)
	Execute(intent *domain.ReleaseIntent, changelog string) (string, error)
	SaveIntent(intent *domain.ReleaseIntent)
	LoadIntent() (*domain.ReleaseIntent, error)
	SaveChangelog(changelog string)
	LoadChangelog() (string, error)
	ClearPending()
	LoadState() string
	BuildPreview(intent *domain.ReleaseIntent, changelog string) string
}

// ReleaseCommand implements the CLI release subcommand.
// This is a thin CLI adapter — all logic lives in workflow.ReleaseService.
type ReleaseCommand struct {
	git         ports.Git
	llm         ports.LLM
	cfg         *config.Config
	commitStore ports.CommitStore
	workDir     string
	releaseSvc  ReleaseSvc
}

// NewReleaseCommand creates a ReleaseCommand with the given dependencies.
func NewReleaseCommand(git ports.Git, llm ports.LLM, cfg *config.Config, commitStore ports.CommitStore, workDir string) *ReleaseCommand {
	return &ReleaseCommand{
		git:         git,
		llm:         llm,
		cfg:         cfg,
		commitStore: commitStore,
		workDir:     workDir,
	}
}

// InitBranchScoping scopes the commit store to the given branch.
// If branch is empty (detached HEAD), the store uses the legacy global path.
// The caller is responsible for obtaining the current branch name
// (typically via git.CurrentBranch()).
func (c *ReleaseCommand) InitBranchScoping(branch string) {
	if branch == "" {
		// Detached HEAD — use legacy global path
		return
	}
	if err := c.commitStore.SetBranch(branch); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to set branch store: %v\n", err)
	}
}

// Run dispatches to the appropriate release subcommand.
func (c *ReleaseCommand) Run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gcourer release <start|apply|abort|regenerate>")
	}
	switch args[0] {
	case "start":
		return c.start(args[1:])
	case "apply":
		return c.apply()
	case "abort":
		return c.abort()
	case "regenerate":
		return c.regenerate(args[1:])
	default:
		return fmt.Errorf("unknown release subcommand: %s (use start|apply|abort|regenerate)", args[0])
	}
}

// SetReleaseService allows injecting a ReleaseSvc for testing.
// If not called, the command creates a default service on first use.
func (c *ReleaseCommand) SetReleaseService(svc ReleaseSvc) {
	c.releaseSvc = svc
}

// service returns the ReleaseService, creating it lazily if needed.
func (c *ReleaseCommand) service() ReleaseSvc {
	if c.releaseSvc != nil {
		return c.releaseSvc
	}
	contextWindow := c.cfg.LLM.ContextWindow
	if contextWindow == 0 {
		contextWindow = 8192
	}
	releaseCfg := workflow.DefaultReleaseServiceConfigWithPaths(
		contextWindow, 20, 500,
		c.workDir+"/.gcourer/release.log",
	)
	c.releaseSvc = workflow.NewReleaseService(
		c.git, c.llm, nil, releaseCfg, nil, c.commitStore,
	)
	return c.releaseSvc
}

func (c *ReleaseCommand) start(args []string) error {
	instruction := ""
	dryRun := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--instruction":
			i++
			if i < len(args) {
				instruction = args[i]
			}
		case "--dry-run":
			dryRun = true
		}
	}

	svc := c.service()
	intent, commits, warnings, err := svc.Prepare(instruction, "")
	if err != nil {
		return fmt.Errorf("release start: %w", err)
	}

	// Print warnings to stderr
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "WARNING: %s\n", w)
	}

	if !intent.IsRelease || commits == "" {
		fmt.Println("No releaseable commits found")
		return nil
	}

	// Generate changelog
	changelog, _, _, err := svc.Generate(commits)
	if err != nil {
		return fmt.Errorf("release start: generate failed: %w", err)
	}

	svc.SaveIntent(intent)
	svc.SaveChangelog(changelog)

	// Print preview
	fmt.Println(svc.BuildPreview(intent, changelog))

	if dryRun {
		fmt.Println("(dry run — no changes made)")
	}
	return nil
}

func (c *ReleaseCommand) apply() error {
	svc := c.service()

	intent, err := svc.LoadIntent()
	if err != nil || intent == nil {
		return fmt.Errorf("no pending release. Run 'gcourer release start' first")
	}

	changelog, _ := svc.LoadChangelog()

	result, err := svc.Execute(intent, changelog)
	if err != nil {
		return fmt.Errorf("release apply: %w", err)
	}

	fmt.Println(result)
	return nil
}

func (c *ReleaseCommand) abort() error {
	svc := c.service()
	svc.ClearPending()
	fmt.Println("Release aborted")
	return nil
}

func (c *ReleaseCommand) regenerate(args []string) error {
	feedback := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--feedback":
			i++
			if i < len(args) {
				feedback = args[i]
			}
		}
	}

	svc := c.service()

	intent, err := svc.LoadIntent()
	if err != nil || intent == nil {
		return fmt.Errorf("no pending release. Run 'gcourer release start' first")
	}

	state := svc.LoadState()
	if state == "processing" {
		return fmt.Errorf("release generation in progress")
	}

	// Re-prepare with feedback
	_, commits, warnings, err := svc.Prepare(feedback, "")
	if err != nil {
		return fmt.Errorf("release regenerate: %w", err)
	}

	// Print warnings to stderr
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "WARNING: %s\n", w)
	}

	changelog, _, _, err := svc.Generate(commits)
	if err != nil {
		return fmt.Errorf("release regenerate: generate failed: %w", err)
	}

	svc.SaveIntent(intent)
	svc.SaveChangelog(changelog)

	fmt.Println(svc.BuildPreview(intent, changelog))
	return nil
}