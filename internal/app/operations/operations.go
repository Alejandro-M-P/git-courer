// Package operationsusecase provides git operations: merge, rebase, stash,
// reset, cherry-pick, revert, clean, tag, blame, reflog, add.
package operations

import (
	"encoding/json"
	"fmt"

	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// Service handles miscellaneous git operations.
type Service struct {
	git ports.Git
}

// NewService creates a new operations service.
func NewService(git ports.Git) *Service {
	return &Service{git: git}
}

// Result holds the outcome of an operation.
type Result struct {
	Operation string `json:"operation"`
	Created   string `json:"created"`
	Result    string `json:"result"`
	Type      string `json:"type"`
	Tokens    int    `json:"tokens"`
}

// Merge merges a branch into the current branch.
func (s *Service) Merge(branch string) (string, error) {
	if branch == "" {
		return "", fmt.Errorf("branch name is required for merge")
	}

	result, err := s.git.Merge(branch)
	if err != nil {
		return "", fmt.Errorf("merge failed: %w", err)
	}

	resp, _ := json.Marshal(Result{
		Operation: "merge",
		Created:   fmt.Sprintf("merged '%s'", branch),
		Result:    result,
		Type:      "direct",
		Tokens:    0,
	})
	return string(resp), nil
}

// Rebase rebases current branch onto another.
func (s *Service) Rebase(branch string) (string, error) {
	if branch == "" {
		return "", fmt.Errorf("branch name is required for rebase")
	}

	result, err := s.git.Rebase(branch)
	if err != nil {
		return "", fmt.Errorf("rebase failed: %w", err)
	}

	resp, _ := json.Marshal(Result{
		Operation: "rebase",
		Created:   fmt.Sprintf("rebased onto '%s'", branch),
		Result:    result,
		Type:      "direct",
		Tokens:    0,
	})
	return string(resp), nil
}

// Stash saves current changes.
func (s *Service) Stash() (string, error) {
	result, err := s.git.Stash()
	if err != nil {
		return "", fmt.Errorf("stash failed: %w", err)
	}

	resp, _ := json.Marshal(Result{
		Operation: "stash",
		Created:   "changes stashed",
		Result:    result,
		Type:      "direct",
		Tokens:    0,
	})
	return string(resp), nil
}

// StashPop restores stashed changes.
func (s *Service) StashPop() (string, error) {
	result, err := s.git.StashPop()
	if err != nil {
		return "", fmt.Errorf("stash pop failed: %w", err)
	}

	resp, _ := json.Marshal(Result{
		Operation: "stash-pop",
		Created:   "restored stashed changes",
		Result:    result,
		Type:      "direct",
		Tokens:    0,
	})
	return string(resp), nil
}

// Reset resets to a commit with the given mode.
func (s *Service) Reset(mode, target string) (string, error) {
	if target == "" {
		target = "HEAD~1"
	}

	result, err := s.git.Reset(mode, target)
	if err != nil {
		return "", fmt.Errorf("reset failed: %w", err)
	}

	resp, _ := json.Marshal(Result{
		Operation: "reset",
		Created:   fmt.Sprintf("reset to '%s'", target),
		Result:    result,
		Type:      "direct",
		Tokens:    0,
	})
	return string(resp), nil
}

// CherryPick cherry-picks a commit.
func (s *Service) CherryPick(commit string) (string, error) {
	if commit == "" {
		return "", fmt.Errorf("commit hash is required for cherry-pick")
	}

	result, err := s.git.CherryPick(commit)
	if err != nil {
		return "", fmt.Errorf("cherry-pick failed: %w", err)
	}

	resp, _ := json.Marshal(Result{
		Operation: "cherry-pick",
		Created:   fmt.Sprintf("cherry-picked '%s'", commit),
		Result:    result,
		Type:      "direct",
		Tokens:    0,
	})
	return string(resp), nil
}

// Revert reverts a commit.
func (s *Service) Revert(commit string) (string, error) {
	if commit == "" {
		return "", fmt.Errorf("commit hash is required for revert")
	}

	result, err := s.git.Revert(commit)
	if err != nil {
		return "", fmt.Errorf("revert failed: %w", err)
	}

	resp, _ := json.Marshal(Result{
		Operation: "revert",
		Created:   fmt.Sprintf("reverted '%s'", commit),
		Result:    result,
		Type:      "direct",
		Tokens:    0,
	})
	return string(resp), nil
}

// Clean removes untracked files.
func (s *Service) Clean() (string, error) {
	result, err := s.git.Clean(true)
	if err != nil {
		return "", fmt.Errorf("clean failed: %w", err)
	}

	resp, _ := json.Marshal(Result{
		Operation: "clean",
		Created:   "cleaned untracked files",
		Result:    result,
		Type:      "direct",
		Tokens:    0,
	})
	return string(resp), nil
}

// Tag creates a tag.
func (s *Service) Tag(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("tag name is required")
	}

	result, err := s.git.Tag(name)
	if err != nil {
		return "", fmt.Errorf("tag failed: %w", err)
	}

	resp, _ := json.Marshal(Result{
		Operation: "tag",
		Created:   fmt.Sprintf("created tag '%s'", name),
		Result:    result,
		Type:      "direct",
		Tokens:    0,
	})
	return string(resp), nil
}

// Blame returns blame information for a file.
func (s *Service) Blame(file string) (string, error) {
	if file == "" {
		return "", fmt.Errorf("file name is required for blame")
	}

	result, err := s.git.Blame(file)
	if err != nil {
		return "", fmt.Errorf("blame failed: %w", err)
	}

	resp, _ := json.Marshal(Result{
		Operation: "blame",
		Created:   fmt.Sprintf("blame for '%s'", file),
		Result:    result,
		Type:      "direct",
		Tokens:    0,
	})
	return string(resp), nil
}

// Reflog returns the reflog.
func (s *Service) Reflog() (string, error) {
	result, err := s.git.Reflog(20)
	if err != nil {
		return "", fmt.Errorf("reflog failed: %w", err)
	}

	resp, _ := json.Marshal(Result{
		Operation: "reflog",
		Created:   "showed reflog",
		Result:    result,
		Type:      "direct",
		Tokens:    0,
	})
	return string(resp), nil
}

// Add stages files.
func (s *Service) Add(files []string) (string, error) {
	if len(files) == 0 {
		files = []string{"."}
	}

	err := s.git.Add(files)
	if err != nil {
		return "", fmt.Errorf("add failed: %w", err)
	}

	resp, _ := json.Marshal(Result{
		Operation: "add",
		Created:   "staged files",
		Result:    fmt.Sprintf("staged %d file(s)", len(files)),
		Type:      "direct",
		Tokens:    0,
	})
	return string(resp), nil
}
