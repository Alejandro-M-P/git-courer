package git

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/blak0p/git-courer/internal/core/domain"
)

func (a *ExecAdapter) Blame(filepath string) ([]domain.BlameLine, error) {
	out, err := a.runGit("blame", "--line-porcelain", filepath)
	if err != nil {
		return nil, err
	}
	return parseBlamePorcelain(out), nil
}

func parseBlamePorcelain(raw string) []domain.BlameLine {
	var lines []domain.BlameLine
	var current domain.BlameLine
	currentHash := ""
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if len(trimmed) >= 40 && isHashLine(trimmed) {
			if currentHash != "" && current.Line > 0 {
				lines = append(lines, current)
			}
			parts := strings.Fields(trimmed)
			current = domain.BlameLine{}
			current.Hash = parts[0]
			currentHash = parts[0]
			if len(parts) >= 3 {
				if n, err := strconv.Atoi(parts[2]); err == nil {
					current.Line = n
				}
			}
			continue
		}
		if strings.HasPrefix(trimmed, "author ") &&
			!strings.HasPrefix(trimmed, "author-mail") &&
			!strings.HasPrefix(trimmed, "author-time") &&
			!strings.HasPrefix(trimmed, "author-tz") {
			current.Author = strings.TrimPrefix(trimmed, "author ")
			continue
		}
		if strings.HasPrefix(line, "\t") {
			if current.Hash != "" {
				lines = append(lines, current)
				current = domain.BlameLine{}
				currentHash = ""
			}
			continue
		}
	}
	if current.Hash != "" && current.Line > 0 {
		lines = append(lines, current)
	}
	return lines
}

func isHashLine(line string) bool {
	if len(line) < 40 {
		return false
	}
	for i := 0; i < 40; i++ {
		c := line[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func (a *ExecAdapter) Show(hash string) (domain.ShowResult, error) {
	if hash == "" {
		return domain.ShowResult{}, fmt.Errorf("commit hash is required")
	}
	out, err := a.runGit("show", "--stat", "--format=%H%n%an%n%ad%n%s", "--date=short", hash)
	if err != nil {
		return domain.ShowResult{}, err
	}
	return parseShow(out), nil
}

func parseShow(raw string) domain.ShowResult {
	var res domain.ShowResult
	lines := strings.Split(raw, "\n")
	if len(lines) >= 1 {
		res.Hash = strings.TrimSpace(lines[0])
	}
	if len(lines) >= 2 {
		res.Author = strings.TrimSpace(lines[1])
	}
	if len(lines) >= 3 {
		res.Date = strings.TrimSpace(lines[2])
	}
	if len(lines) >= 4 {
		res.Message = strings.TrimSpace(lines[3])
	}
	res.FileList = []string{}
	for _, line := range lines[4:] {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "-") {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) == 2 {
			res.Files++
			fileName := strings.TrimSpace(parts[0])
			res.FileList = append(res.FileList, fileName)
			stats := strings.TrimSpace(parts[1])
			res.Additions += strings.Count(stats, "+")
			res.Deletions += strings.Count(stats, "-")
		}
	}
	return res
}

func (a *ExecAdapter) StashDiff(index string) (string, error) {
	ref := "stash@{0}"
	if index != "" {
		ref = normalizeStashRef(index)
	}
	return a.runGit("stash", "show", "-p", ref)
}

func (a *ExecAdapter) StashList() ([]domain.StashEntry, error) {
	out, err := a.runGit("stash", "list", "--format=%gd|%gs|%H")
	if err != nil {
		return nil, err
	}
	return parseStashList(out), nil
}

func parseStashList(raw string) []domain.StashEntry {
	var entries []domain.StashEntry
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) >= 3 {
			index := 0
			if n, err := strconv.Atoi(strings.TrimPrefix(parts[0], "stash@{")); err == nil {
				index = n
			}
			entries = append(entries, domain.StashEntry{
				Index:   index,
				Message: parts[1],
				Hash:    parts[2],
			})
		}
	}
	return entries
}

func (a *ExecAdapter) MergeBase(aRef, bRef string) (string, error) {
	if aRef == "" || bRef == "" {
		return "", fmt.Errorf("both refs are required")
	}
	out, err := a.runGit("merge-base", aRef, bRef)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (a *ExecAdapter) Search(pattern string, context, before, after int, paths ...string) (string, error) {
	if pattern == "" {
		return "", fmt.Errorf("search pattern is required")
	}
	args := []string{"grep", "--line-number", "-I", "--heading", "--break"}
	if context > 0 {
		args = append(args, fmt.Sprintf("-C%d", context))
	}
	if before > 0 {
		args = append(args, fmt.Sprintf("-B%d", before))
	}
	if after > 0 {
		args = append(args, fmt.Sprintf("-A%d", after))
	}
	args = append(args, pattern)
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	out, err := a.runGit(args...)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "exit status 1") ||
			strings.Contains(errStr, "error: 1") ||
			(strings.Contains(errStr, "git error") && out == "") {
			return "", nil
		}
		return "", err
	}
	return out, nil
}

func (a *ExecAdapter) CatFile(revision, path string) (string, error) {
	if revision == "" {
		revision = "HEAD"
	}
	if path == "" {
		return a.runGit("show", revision)
	}
	ref := revision + ":" + path
	return a.runGit("show", ref)
}

func (a *ExecAdapter) ListTree(revision, path string, recursive bool) ([]string, error) {
	if revision == "" {
		revision = "HEAD"
	}
	args := []string{"ls-tree", "--name-only"}
	if recursive {
		args = append(args, "-r")
	}
	args = append(args, revision)
	if path != "" {
		args = append(args, path)
	}
	out, err := a.runGit(args...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return []string{}, nil
	}
	return strings.Split(strings.TrimSpace(out), "\n"), nil
}
