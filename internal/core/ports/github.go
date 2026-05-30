// Package ports defines the interfaces that the core domain depends on.
package ports

import (
	"context"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// GitHubAPI provides access to GitHub PR-level commit details.
// Implementations use go-github or the gh CLI under the hood.
type GitHubAPI interface {
	// FetchPRCommits fetches the list of commits for each PR.
	// Returns a map where the key is the PR number and the value is the ordered slice of commits.
	FetchPRCommits(ctx context.Context, owner, repo string, prNumbers []int) (map[int][]domain.PRCommit, error)
}
