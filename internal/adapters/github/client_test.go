package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

func TestFetchPRCommits(t *testing.T) {
	mockCommits := []map[string]interface{}{
		{
			"sha": "abc123",
			"commit": map[string]interface{}{
				"message": "feat: add login",
			},
			"author": map[string]interface{}{
				"login": "alice",
			},
		},
	}

	cases := []struct {
		name        string
		prNumbers   []int
		statusCode  int
		wantCounts  map[int]int // PR number -> expected commit count
		wantErr     bool
	}{
		{
			name:       "single PR success",
			prNumbers:  []int{42},
			statusCode: http.StatusOK,
			wantCounts: map[int]int{42: 1},
			wantErr:    false,
		},
		{
			name:       "multiple PRs parallel",
			prNumbers:  []int{10, 20},
			statusCode: http.StatusOK,
			wantCounts: map[int]int{10: 1, 20: 1},
			wantErr:    false,
		},
		{
			name:       "API error returns error",
			prNumbers:  []int{42},
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
		{
			name:       "404 returns error",
			prNumbers:  []int{99},
			statusCode: http.StatusNotFound,
			wantErr:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var requestCount atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestCount.Add(1)
				// Verify path format: /repos/owner/repo/pulls/{prNumber}/commits
				if tc.statusCode != http.StatusOK {
					w.WriteHeader(tc.statusCode)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(mockCommits)
			}))
			defer srv.Close()

			client := NewClient("test-token")
			// Override base URL for testing
			client.client, _ = githubNewClientWithHTTP(srv.Client(), srv.URL)

			result, err := client.FetchPRCommits(context.Background(), "owner", "repo", tc.prNumbers)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for _, prNum := range tc.prNumbers {
				commits, ok := result[prNum]
				if !ok {
					t.Errorf("expected PR #%d in result", prNum)
					continue
				}
				expectedCount := tc.wantCounts[prNum]
				if len(commits) != expectedCount {
					t.Errorf("PR #%d: got %d commits, want %d", prNum, len(commits), expectedCount)
				}
				if expectedCount > 0 {
					c := commits[0]
					if c.SHA != "abc123" {
						t.Errorf("PR #%d commit SHA = %q, want %q", prNum, c.SHA, "abc123")
					}
					if c.Message != "feat: add login" {
						t.Errorf("PR #%d commit Message = %q, want %q", prNum, c.Message, "feat: add login")
					}
					if c.Author != "alice" {
						t.Errorf("PR #%d commit Author = %q, want %q", prNum, c.Author, "alice")
					}
				}
			}
		})
	}
}

func TestFetchPRCommitsMapsToPRCommit(t *testing.T) {
	// Verify domain.PRCommit fields are correctly mapped from go-github types
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		commits := []map[string]interface{}{
			{
				"sha": "deadbeef",
				"commit": map[string]interface{}{
					"message": "fix: security issue",
				},
				"author": map[string]interface{}{
					"login": "bob",
				},
			},
		}
		json.NewEncoder(w).Encode(commits)
	}))
	defer srv.Close()

	client := NewClient("test-token")
	client.client, _ = githubNewClientWithHTTP(srv.Client(), srv.URL)

	result, err := client.FetchPRCommits(context.Background(), "owner", "repo", []int{7})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	prCommits := result[7]
	if len(prCommits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(prCommits))
	}

	got := prCommits[0]
	want := domain.PRCommit{SHA: "deadbeef", Message: "fix: security issue", Author: "bob"}
	if got != want {
		t.Errorf("PRCommit = %+v, want %+v", got, want)
	}
}

func TestFetchPRCommitsEmptyPRList(t *testing.T) {
	client := NewClient("test-token")

	result, err := client.FetchPRCommits(context.Background(), "owner", "repo", []int{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

// Suppress unused import for fmt — it's used in URL formatting
var _ = fmt.Sprintf