// Package github implements the GitHubAPI port using go-github.
package github

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	gh "github.com/google/go-github/v74/github"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

const maxConcurrentPRFetches = 5

// Client implements ports.GitHubAPI using the go-github library.
type Client struct {
	client *gh.Client
}

// NewClient creates a GitHub API client authenticated with the given token.
func NewClient(token string) *Client {
	c := gh.NewClient(nil).WithAuthToken(token)
	return &Client{client: c}
}

// FetchPRCommits fetches commits for each PR number in parallel (max 5 concurrent).
// Returns a map from PR number to its commits.
// Partial failure is tolerated — if one PR returns 404 (not a PR) or another error,
// that PR is skipped silently. Error is only returned when every single PR fails.
func (c *Client) FetchPRCommits(ctx context.Context, owner, repo string, prNumbers []int) (map[int][]domain.PRCommit, error) {
	result := make(map[int][]domain.PRCommit, len(prNumbers))
	if len(prNumbers) == 0 {
		return result, nil
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentPRFetches)
	var successCount int
	var firstErr error
	var once sync.Once

	for _, prNum := range prNumbers {
		prNum := prNum // capture
		wg.Add(1)
		go func() {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			commits, _, err := c.client.PullRequests.ListCommits(ctx, owner, repo, prNum, nil)
			if err != nil {
				once.Do(func() { firstErr = fmt.Errorf("fetch commits for PR #%d: %w", prNum, err) })
				// Swallow the error — caller will use the fat commit as fallback for this PR
				return
			}

			var prCommits []domain.PRCommit
			for _, rc := range commits {
				prCommits = append(prCommits, domain.PRCommit{
					SHA:     rc.GetSHA(),
					Message: rc.GetCommit().GetMessage(),
					Author:  rc.GetAuthor().GetLogin(),
				})
			}

			mu.Lock()
			result[prNum] = prCommits
			successCount++
			mu.Unlock()
		}()
	}

	wg.Wait()

	// Error only if *no* PR succeeded yet all failed.
	if len(result) == 0 && firstErr != nil {
		return nil, firstErr
	}

	return result, nil
}

// githubNewClientWithHTTP creates a go-github client pointing at a custom base URL
// (used by tests to redirect to httptest.NewServer).
func githubNewClientWithHTTP(httpClient *http.Client, baseURL string) (*gh.Client, error) {
	client, err := gh.NewClient(httpClient).WithEnterpriseURLs(baseURL, baseURL)
	if err != nil {
		return nil, fmt.Errorf("create github client with custom URL: %w", err)
	}
	return client, nil
}
