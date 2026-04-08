// Package queryusecase provides read-only git operations: status, log, diff, branches.
package query

import (
	"encoding/json"
	"fmt"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// Service handles read-only git queries.
type Service struct {
	git ports.Git
}

// NewService creates a new query service.
func NewService(git ports.Git) *Service {
	return &Service{git: git}
}

// Result holds the outcome of a query operation.
type Result struct {
	Operation string `json:"operation"`
	Result    string `json:"result"`
	Type      string `json:"type"`
	Tokens    int    `json:"tokens"`
}

// Status returns the current repository status.
func (s *Service) Status() (string, error) {
	status, err := s.git.Status()
	if err != nil {
		return "", fmt.Errorf("failed to get status: %w", err)
	}

	output := formatStatus(status)
	resp, _ := json.Marshal(Result{
		Operation: "status",
		Result:    output,
		Type:      "read",
		Tokens:    0,
	})
	return string(resp), nil
}

// Log returns recent commit history.
func (s *Service) Log(limit int) (string, error) {
	result, err := s.git.Log(limit)
	if err != nil {
		return "", fmt.Errorf("failed to get log: %w", err)
	}

	resp, _ := json.Marshal(Result{
		Operation: "log",
		Result:    result,
		Type:      "read",
		Tokens:    0,
	})
	return string(resp), nil
}

// Diff returns the diff for all changes.
func (s *Service) Diff() (string, error) {
	result, err := s.git.Diff()
	if err != nil {
		return "", fmt.Errorf("failed to get diff: %w", err)
	}

	if result == "" {
		result = "No unstaged changes."
	}

	resp, _ := json.Marshal(Result{
		Operation: "diff",
		Result:    result,
		Type:      "read",
		Tokens:    0,
	})
	return string(resp), nil
}

// CurrentBranch returns the current branch name.
func (s *Service) CurrentBranch() (string, error) {
	branch, err := s.git.CurrentBranch()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	resp, _ := json.Marshal(Result{
		Operation: "branches",
		Result:    fmt.Sprintf("Current branch: %s", branch),
		Type:      "read",
		Tokens:    0,
	})
	return string(resp), nil
}

func formatStatus(status domain.Status) string {
	output := fmt.Sprintf("Branch: %s\n", status.Branch)

	if status.IsClean {
		output += "Status: clean\n"
	} else {
		output += fmt.Sprintf("Status: %d files changed\n", len(status.Files))
		output += fmt.Sprintf("  Staged: %d\n", status.Staged)
		output += fmt.Sprintf("  Modified: %d\n", status.Modified)
		output += fmt.Sprintf("  Untracked: %d\n", status.Untracked)
	}

	if len(status.Files) > 0 {
		output += "\nFiles:\n"
		for _, f := range status.Files {
			output += fmt.Sprintf("  %s %s\n", f.Status, f.Path)
		}
	}

	return output
}
