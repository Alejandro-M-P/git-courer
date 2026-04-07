// Package branchusecase provides branch operations: create, checkout, delete.
package branch

import (
	"encoding/json"
	"fmt"

	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// Service handles branch operations.
type Service struct {
	git ports.Git
}

// NewService creates a new branch service.
func NewService(git ports.Git) *Service {
	return &Service{git: git}
}

// Result holds the outcome of a branch operation.
type Result struct {
	Operation string `json:"operation"`
	Created   string `json:"created"`
	Result    string `json:"result"`
	Type      string `json:"type"`
	Tokens    int    `json:"tokens"`
}

// Create creates a new branch and checks it out.
func (s *Service) Create(branch string) (string, error) {
	if branch == "" {
		return "", fmt.Errorf("branch name is required")
	}

	result, err := s.git.Branch(branch)
	if err != nil {
		return "", fmt.Errorf("failed to create branch '%s': %w", branch, err)
	}

	resp, _ := json.Marshal(Result{
		Operation: "create-branch",
		Created:   fmt.Sprintf("branch '%s'", branch),
		Result:    result,
		Type:      "direct",
		Tokens:    0,
	})
	return string(resp), nil
}

// Checkout switches to an existing branch.
func (s *Service) Checkout(branch string) (string, error) {
	if branch == "" {
		return "", fmt.Errorf("branch name is required")
	}

	result, err := s.git.Checkout(branch)
	if err != nil {
		return "", fmt.Errorf("failed to checkout '%s': %w", branch, err)
	}

	resp, _ := json.Marshal(Result{
		Operation: "checkout",
		Created:   fmt.Sprintf("switched to '%s'", branch),
		Result:    result,
		Type:      "direct",
		Tokens:    0,
	})
	return string(resp), nil
}

// Delete deletes a branch.
func (s *Service) Delete(branch string) (string, error) {
	if branch == "" {
		return "", fmt.Errorf("branch name is required")
	}

	result, err := s.git.DeleteBranch(branch)
	if err != nil {
		return "", fmt.Errorf("failed to delete branch '%s': %w", branch, err)
	}

	resp, _ := json.Marshal(Result{
		Operation: "delete-branch",
		Created:   fmt.Sprintf("deleted branch '%s'", branch),
		Result:    result,
		Type:      "direct",
		Tokens:    0,
	})
	return string(resp), nil
}

// Current returns the current branch name.
func (s *Service) Current() (string, error) {
	branch, err := s.git.CurrentBranch()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	resp, _ := json.Marshal(Result{
		Operation: "current-branch",
		Result:    fmt.Sprintf("Current branch: %s", branch),
		Type:      "read",
		Tokens:    0,
	})
	return string(resp), nil
}
