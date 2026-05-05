package git

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

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

		fc := ports.FileContent{
			Filename: f,
			Before:   before,
			After:    after,
		}

		if beforeErr != nil && afterErr != nil {
			return nil, fmt.Errorf("read %q: before=%v, after=%v", f, beforeErr, afterErr)
		}

		result = append(result, fc)
	}

	return result, nil
}

func (g *GitContentProvider) show(ref string) ([]byte, error) {
	cmd := exec.Command("git", "-C", g.repoRoot, "show", ref)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "does not exist") || strings.Contains(stderr, "Not a valid object") {
				return nil, nil // File does not exist at this ref
			}
		}
		return nil, fmt.Errorf("git show %s: %w", ref, err)
	}
	return out, nil
}
