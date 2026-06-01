package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/blak0p/git-courer/internal/core/ports"
)

var errNotFound = errors.New("file not found at ref")

// GitContentProvider reads file contents from git objects.
type GitContentProvider struct {
	repoRoot string
}

// NewGitContentProvider creates a provider for the given repo root.
func NewGitContentProvider(repoRoot string) *GitContentProvider {
	return &GitContentProvider{repoRoot: repoRoot}
}

// GetContents reads before and after contents for changed files.
//
//   - Before: git show HEAD:{path} (empty for new files)
//   - After: git show :{path} (empty for deleted files)
func (g *GitContentProvider) GetContents(files []string) ([]ports.FileContent, error) {
	var result []ports.FileContent

	for _, f := range files {
		f = strings.TrimPrefix(f, g.repoRoot+"/")
		f = strings.TrimPrefix(f, "./")

		before, beforeErr := g.show("HEAD:" + f)
		after, afterErr := g.show(":" + f)

		if beforeErr != nil && beforeErr != errNotFound {
			return nil, beforeErr
		}
		if afterErr != nil && afterErr != errNotFound {
			return nil, afterErr
		}

		if beforeErr == errNotFound && afterErr == errNotFound {
			return nil, fmt.Errorf("read %q: file does not exist in HEAD or index", f)
		}

		fc := ports.FileContent{
			Filename: f,
		}
		if beforeErr == nil {
			fc.Before = before
		}
		if afterErr == nil {
			fc.After = after
		}

		result = append(result, fc)
	}

	return result, nil
}

func (g *GitContentProvider) show(ref string) ([]byte, error) {
	cmd := exec.Command("git", "-C", g.repoRoot, "show", ref)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "does not exist") ||
				strings.Contains(stderr, "Not a valid object") ||
				strings.Contains(stderr, "invalid object name") {
				return nil, errNotFound
			}
		}
		return nil, fmt.Errorf("git show %s: %w", ref, err)
	}
	return out, nil
}
