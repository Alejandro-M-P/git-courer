// Package remoteusecase provides remote operations: push, pull, fetch.
package remote

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// Service handles remote operations with retry logic.
type Service struct {
	git ports.Git
}

// NewService creates a new remote service.
func NewService(git ports.Git) *Service {
	return &Service{git: git}
}

// Result holds the outcome of a remote operation.
type Result struct {
	Operation string `json:"operation"`
	Created   string `json:"created"`
	Result    string `json:"result"`
	Type      string `json:"type"`
	Tokens    int    `json:"tokens"`
}

// Push pushes to remote with automatic retry on upstream/rejection errors.
func (s *Service) Push() (string, error) {
	result, err := s.git.Push()

	// If push rejected because no upstream, set it up and retry
	if err != nil && (strings.Contains(err.Error(), "no upstream") ||
		strings.Contains(err.Error(), "no tracking information") ||
		strings.Contains(err.Error(), "no hay informaci")) {

		branch, branchErr := s.git.CurrentBranch()
		if branchErr != nil {
			return "", fmt.Errorf("push failed: could not determine branch")
		}
		result, err = s.git.PushWithUpstream(branch)
	}

	// If push rejected (remote has new commits), pull and try again
	if err != nil && strings.Contains(err.Error(), "PUSH_REJECTED") {
		pullResult, pullErr := s.git.PullRebase()
		if pullErr != nil {
			pullResult, pullErr = s.git.Pull()
			if pullErr != nil {
				return "", fmt.Errorf("push rejected and pull failed: %w", pullErr)
			}
		}
		result, err = s.git.Push()
		if err != nil {
			return "", fmt.Errorf("push failed after pull: %w", err)
		}
		result = pullResult + "\n" + result
	}

	if err != nil {
		return "", fmt.Errorf("push failed: %w", err)
	}

	resp, _ := json.Marshal(Result{
		Operation: "push",
		Created:   "pushed to remote",
		Result:    result,
		Type:      "direct",
		Tokens:    0,
	})
	return string(resp), nil
}

// Pull pulls from remote.
func (s *Service) Pull() (string, error) {
	result, err := s.git.Pull()
	if err != nil {
		return "", fmt.Errorf("pull failed: %w", err)
	}

	resp, _ := json.Marshal(Result{
		Operation: "pull",
		Created:   "pulled from remote",
		Result:    result,
		Type:      "direct",
		Tokens:    0,
	})
	return string(resp), nil
}

// Fetch fetches from remote.
func (s *Service) Fetch() (string, error) {
	result, err := s.git.Fetch()
	if err != nil {
		return "", fmt.Errorf("fetch failed: %w", err)
	}

	resp, _ := json.Marshal(Result{
		Operation: "fetch",
		Created:   "fetched from remote",
		Result:    result,
		Type:      "direct",
		Tokens:    0,
	})
	return string(resp), nil
}
