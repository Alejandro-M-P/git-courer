// Package cli provides CLI delivery adapters for git-courer subcommands.
package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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

// InitBranchScoping scopes the commit store to the given branch/workspace id.
// If branch is empty (detached HEAD), the store uses the legacy global path.
// The caller is responsible for obtaining the current branch name
// (typically via git.CurrentBranch()).
func (c *ReleaseCommand) InitBranchScoping(branch string) {
	if branch == "" {
		// Detached HEAD — use legacy global path
		return
	}
	if err := c.commitStore.SetWorkspace(branch); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to set workspace store: %v\n", err)
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
	if c.cfg != nil && !c.cfg.LLM.Enabled {
		return fmt.Errorf("release command requires AI generation to be enabled. Please enable llm.enabled and configure an OpenAI standard compatible local provider (e.g., Ollama) in your configuration file")
	}

	svc := c.service()
	reader := c.reader()

	for {
		// 1. Prepare
		intent, commits, warnings, err := svc.Prepare("", "")
		if err != nil {
			return fmt.Errorf("release: %w", err)
		}
		for _, w := range warnings {
			fmt.Fprintf(c.writer(), "WARNING: %s\n", w)
		}
		if !intent.IsRelease || commits == "" {
			fmt.Fprintln(c.writer(), "No new commits since last tag")
			return nil
		}

		// 2. Ask tag
		fmt.Fprintf(c.writer(), "Tag? [%s]: ", intent.TagName)
		tagInput := c.readLine(reader)
		if tagInput != "" {
			intent, commits, _, _ = svc.Prepare(tagInput, "")
		}

		// 3. Ask guidance
		fmt.Fprint(c.writer(), "Add guidance for changelog generation? (y/N): ")
		guidanceAction := strings.TrimSpace(strings.ToLower(c.readLine(reader)))
		if guidanceAction == "y" {
			msgPath := filepath.Join(domain.ResolveMetadataDir(c.workDir), "release_guidance.md")
			if mkErr := os.MkdirAll(filepath.Dir(msgPath), 0o755); mkErr != nil {
				fmt.Fprintf(c.writer(), "Failed to create editor: %v\n", mkErr)
			} else {
				_ = os.WriteFile(msgPath, []byte("# Enter guidance for changelog generation\n\n"), 0o644)
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
				editCmd := exec.Command(editor, msgPath)
				editCmd.Stdin = os.Stdin
				editCmd.Stdout = os.Stdout
				editCmd.Stderr = os.Stderr
				_ = editCmd.Run()
				content, readErr := os.ReadFile(msgPath)
				if readErr != nil {
					fmt.Fprintf(c.writer(), "Read guidance failed: %v\n", readErr)
				} else if trimmed := strings.TrimSpace(string(content)); trimmed != "" {
					svc.SetCustomMessage(string(content))
				}
			}
		}

		// 4. Generate changelog
		changelog, _, _, err := svc.Generate(commits)
		if err != nil {
			return fmt.Errorf("release: generate failed: %w", err)
		}

	preview:
		for {
			// 5. Show preview with glamour
			preview := svc.BuildPreview(intent, changelog)
			rendered, renderErr := glamour.Render(preview, "dark")
			if renderErr != nil {
				rendered = preview
			}
			fmt.Fprint(c.writer(), rendered)

			// 6. Ask action
			fmt.Fprint(c.writer(), "Apply? (y/N/r/e): ")
			action := strings.TrimSpace(strings.ToLower(c.readLine(reader)))

			switch action {
			case "y":
				svc.SaveIntent(intent)
				svc.SaveChangelog(changelog)
				result, err := svc.Execute(intent, changelog)
				if err != nil {
					return fmt.Errorf("release: apply failed: %w", err)
				}
				fmt.Fprintln(c.writer(), result)
				return nil
			case "r":
				fmt.Fprint(c.writer(), "Add feedback for regeneration? (y/N): ")
				feedbackAction := strings.TrimSpace(strings.ToLower(c.readLine(reader)))
				var feedback string
				if feedbackAction == "y" {
					fbPath := filepath.Join(domain.ResolveMetadataDir(c.workDir), "release_regenerate_feedback.md")
					if mkErr := os.MkdirAll(filepath.Dir(fbPath), 0o755); mkErr != nil {
						fmt.Fprintf(c.writer(), "Failed to create editor: %v\n", mkErr)
						continue
					}
					_ = os.WriteFile(fbPath, []byte("# Enter feedback for changelog regeneration\n\n"), 0o644)
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
					editCmd := exec.Command(editor, fbPath)
					editCmd.Stdin = os.Stdin
					editCmd.Stdout = os.Stdout
					editCmd.Stderr = os.Stderr
					_ = editCmd.Run()
					content, readErr := os.ReadFile(fbPath)
					if readErr != nil {
						fmt.Fprintf(c.writer(), "Read feedback failed: %v\n", readErr)
						continue
					}
					feedback = string(content)
				}
				regenerated, regErr := c.llm.RegenerateChangelog(changelog, feedback)
				if regErr != nil {
					fmt.Fprintf(c.writer(), "Regenerate failed: %v\n", regErr)
					continue // stay on preview, keep state intact
				}
				changelog = regenerated
				continue preview // skip tag/message re-prompts
			case "e":
				changelogPath := filepath.Join(domain.ResolveMetadataDir(c.workDir), "release_changelog.md")
				// Persist the current changelog to the real on-disk path so the editor
				// opens on a populated file. We write directly rather than relying on
				// svc.SaveChangelog's filesystem side effect — this keeps the edit
				// flow self-contained and testable with a mock service.
				if mkErr := os.MkdirAll(filepath.Dir(changelogPath), 0o755); mkErr != nil {
					fmt.Fprintf(c.writer(), "Edit failed: %v\n", mkErr)
					continue
				}
				if writeErr := os.WriteFile(changelogPath, []byte(changelog), 0o644); writeErr != nil {
					fmt.Fprintf(c.writer(), "Edit failed: %v\n", writeErr)
					continue
				}
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
				editCmd := exec.Command(editor, changelogPath)
				editCmd.Stdin = os.Stdin
				editCmd.Stdout = os.Stdout
				editCmd.Stderr = os.Stderr
				_ = editCmd.Run()
				edited, readErr := os.ReadFile(changelogPath)
				if readErr != nil {
					fmt.Fprintf(c.writer(), "Read edited changelog failed: %v\n", readErr)
					continue // stay on preview, keep state intact
				}
				changelog = string(edited)
				svc.SaveChangelog(changelog)
				continue preview // skip tag/message re-prompts
			default:
				svc.ClearPending()
				fmt.Fprintln(c.writer(), "Release cancelled")
				return nil
			}
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

// writer returns the Stdout writer (or os.Stdout if Stdout is nil).
func (c *ReleaseCommand) writer() io.Writer {
	if c.Stdout != nil {
		return c.Stdout
	}
	return os.Stdout
}

// readLine reads a single line from r, trimming trailing newline characters.
func (c *ReleaseCommand) readLine(r *bufio.Reader) string {
	line, err := r.ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimRight(line, "\r\n")
}