// Package cli provides CLI delivery adapters for git-courer subcommands.
package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/blak0p/git-courer/internal/config"
	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/core/ports"
	"github.com/blak0p/git-courer/internal/workflow"
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
	SetCustomMessage(msg string)
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
	Stdin       io.Reader  // nil = os.Stdin
	Stdout      io.Writer  // nil = os.Stdout
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
		"",
	)
	releaseCfg.WorkDir = c.workDir
	releaseCfg.ReleaseType = c.cfg.Release.Type
	c.releaseSvc = workflow.NewReleaseService(
		c.git, c.llm, nil, releaseCfg, nil, c.commitStore,
	)
	return c.releaseSvc
}

// Run executes the interactive release loop.
func (c *ReleaseCommand) Run() error {
	svc := c.service()
	reader := c.reader()

	for {
		// 1. Prepare
		intent, commits, warnings, err := svc.Prepare("", "")
		if err != nil {
			return fmt.Errorf("release: %w", err)
		}
		for _, w := range warnings {
			fmt.Fprintf(c.Stdout, "WARNING: %s\n", w)
		}
		if !intent.IsRelease || commits == "" {
			fmt.Fprintln(c.Stdout, "No new commits since last tag")
			return nil
		}

		// 2. Ask tag
		fmt.Fprintf(c.Stdout, "Tag? [%s]: ", intent.TagName)
		tagInput := c.readLine(reader)
		if tagInput != "" {
			intent, commits, _, _ = svc.Prepare(tagInput, "")
		}

		// 3. Ask message
		fmt.Fprint(c.Stdout, "Additional message? (optional): ")
		msgInput := c.readLine(reader)
		if msgInput != "" {
			svc.SetCustomMessage(msgInput)
		}

		// 4. Generate changelog
		changelog, _, _, err := svc.Generate(commits)
		if err != nil {
			return fmt.Errorf("release: generate failed: %w", err)
		}

		// 5. Show preview with glamour
		preview := svc.BuildPreview(intent, changelog)
		rendered, renderErr := glamour.Render(preview, "dark")
		if renderErr != nil {
			rendered = preview
		}
		fmt.Fprint(c.Stdout, rendered)

		// 6. Ask action
		fmt.Fprint(c.Stdout, "Apply? (s/N/r/e): ")
		action := strings.TrimSpace(strings.ToLower(c.readLine(reader)))

		switch action {
		case "s":
			svc.SaveIntent(intent)
			svc.SaveChangelog(changelog)
			result, err := svc.Execute(intent, changelog)
			if err != nil {
				return fmt.Errorf("release: apply failed: %w", err)
			}
			fmt.Fprintln(c.Stdout, result)
			return nil
		case "r":
			fmt.Fprint(c.Stdout, "Feedback to regenerate changelog: ")
			feedback := c.readLine(reader)
			if feedback != "" {
				svc.SetCustomMessage(feedback)
			}
			continue
		case "e":
			svc.SaveChangelog(changelog)
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = os.Getenv("VISUAL")
			}
			if editor == "" {
				if runtime.GOOS == "windows" {
					editor = "notepad"
				} else {
					editor = "vi"
				}
			}
			editCmd := exec.Command(editor, "release_changelog.md")
			editCmd.Stdin = os.Stdin
			editCmd.Stdout = os.Stdout
			editCmd.Stderr = os.Stderr
			_ = editCmd.Run()
			edited, _ := svc.LoadChangelog()
			if edited != "" {
				changelog = edited
			}
			continue
		default:
			svc.ClearPending()
			fmt.Fprintln(c.Stdout, "Release cancelled")
			return nil
		}
	}
}

// reader returns a buffered reader over Stdin (or os.Stdin if Stdin is nil).
func (c *ReleaseCommand) reader() *bufio.Reader {
	r := c.Stdin
	if r == nil {
		r = os.Stdin
	}
	return bufio.NewReader(r)
}

// readLine reads a single line from r, trimming trailing newline characters.
func (c *ReleaseCommand) readLine(r *bufio.Reader) string {
	line, err := r.ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimRight(line, "\r\n")
}