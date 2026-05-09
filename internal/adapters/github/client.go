// Package github implements the GitHubAPI port using raw HTTP calls to avoid heavy SDK dependencies.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

const maxConcurrentPRFetches = 5

// Client implements ports.GitHubAPI using raw HTTP calls.
type Client struct {
	token      string
	httpClient *http.Client
	baseURL    string // Used for testing
}

// NewClient creates a GitHub API client authenticated with the given token.
func NewClient(token string) *Client {
	return &Client{
		token: token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://api.github.com",
	}
}

// githubCommit matches the structure of the GitHub API response for commits.
type githubCommit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
	} `json:"commit"`
	Author *struct {
		Login string `json:"login"`
	} `json:"author"`
}

// FetchPRCommits fetches commits for each PR number in parallel (max 5 concurrent).
func (c *Client) FetchPRCommits(ctx context.Context, owner, repo string, prNumbers []int) (map[int][]domain.PRCommit, error) {
	result := make(map[int][]domain.PRCommit, len(prNumbers))
	if len(prNumbers) == 0 {
		return result, nil
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentPRFetches)
	var firstErr error
	var once sync.Once

	for _, prNum := range prNumbers {
		prNum := prNum
		wg.Add(1)
		go func() {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			commits, err := c.fetchCommits(ctx, owner, repo, prNum)
			if err != nil {
				once.Do(func() { firstErr = err })
				return
			}

			mu.Lock()
			result[prNum] = commits
			mu.Unlock()
		}()
	}

	wg.Wait()

	if len(result) == 0 && firstErr != nil {
		return nil, firstErr
	}

	return result, nil
}

func (c *Client) fetchCommits(ctx context.Context, owner, repo string, prNum int) ([]domain.PRCommit, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/commits", c.baseURL, owner, repo, prNum)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api error: %s", resp.Status)
	}

	var ghCommits []githubCommit
	if err := json.NewDecoder(resp.Body).Decode(&ghCommits); err != nil {
		return nil, err
	}

	commits := make([]domain.PRCommit, 0, len(ghCommits))
	for _, gc := range ghCommits {
		author := ""
		if gc.Author != nil {
			author = gc.Author.Login
		}
		commits = append(commits, domain.PRCommit{
			SHA:     gc.SHA,
			Message: gc.Commit.Message,
			Author:  author,
		})
	}

	return commits, nil
}

// githubNewClientWithHTTP is used by tests to override the client and base URL.
func githubNewClientWithHTTP(httpClient *http.Client, baseURL string) (*Client, error) {
	return &Client{
		httpClient: httpClient,
		baseURL:    baseURL,
	}, nil
}
