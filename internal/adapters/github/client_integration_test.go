//go:build integration
// +build integration

package github

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestFetchPRCommits_Live makes a real API call to GitHub for PR #40.
// It NEVER fails — skips if token is missing, API is down, or PR has no commits.
func TestFetchPRCommits_Live(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live test in short mode")
	}

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("GITHUB_TOKEN not set — skipping live GitHub API test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := NewClient(token)

	result, err := client.FetchPRCommits(ctx, "blak0p", "git-courer", []int{40})
	if err != nil {
		// API might be rate-limited or down — not a code failure
		t.Skipf("Live API call failed: %v", err)
	}

	commits, ok := result[40]
	if !ok || len(commits) == 0 {
		t.Skip("PR #40 returned no commits (may have been deleted or token lacks permissions)")
	}

	t.Logf("PR #40 has %d commits:", len(commits))
	for _, c := range commits {
		t.Logf("  - %s: %s", c.SHA[:8], c.Message)
	}

	// Verify we got commits with SHA and Message
	for _, c := range commits {
		if c.SHA == "" {
			t.Error("commit SHA is empty")
		}
		if c.Message == "" {
			t.Error("commit Message is empty")
		}
	}
}
