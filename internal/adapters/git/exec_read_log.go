package git

import (
	"fmt"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

func (a *ExecAdapter) Log(limit int, pattern string, paths ...string) (string, error) {
	args := []string{"log", fmt.Sprintf("-%d", limit), "--pretty=format:%H|%an|%ad|%s", "--date=short"}
	
	// Separate flags from paths
	var actualPaths []string
	for _, p := range paths {
		if strings.HasPrefix(p, "-") {
			args = append(args, p)
		} else {
			actualPaths = append(actualPaths, p)
		}
	}

	if pattern != "" {
		args = append(args, "-i", "--grep="+pattern)
	}
	
	if len(actualPaths) > 0 {
		args = append(args, "--")
		args = append(args, actualPaths...)
	}
	out, err := a.runGit(args...)
	if err != nil {
		// git log returns exit status 1 if no matches are found with --grep
		if strings.Contains(err.Error(), "exit status 1") {
			return "", nil
		}
		return "", err
	}
	return out, nil
}

func (a *ExecAdapter) LogFull(limit int) (string, error) {
	return a.runGit("log", fmt.Sprintf("-%d", limit))
}

func (a *ExecAdapter) Reflog() ([]domain.ReflogEntry, error) {
	out, err := a.runGit("reflog")
	if err != nil {
		return nil, err
	}
	return parseReflog(out), nil
}

func parseReflog(raw string) []domain.ReflogEntry {
	var entries []domain.ReflogEntry
	for i, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			entries = append(entries, domain.ReflogEntry{
				Index:  i,
				Hash:   parts[0],
				Action: parts[2],
			})
		}
	}
	return entries
}
