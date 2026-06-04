package domain

import (
	"strings"
)

// BaseBranchDetector provides the git operations needed for base branch detection.
// This is a narrow interface extracted from ports.Git to keep DetectBaseBranch testable
// without requiring a full Git mock.
type BaseBranchDetector interface {
	SymbolicRef(ref string) (string, error)
	ConfigGet(key string) (string, error)
}

// DetectBaseBranch attempts to detect the base branch for the repository.
// It tries, in order:
//  1. git symbolic-ref refs/remotes/origin/HEAD — returns the branch that
//     origin/HEAD points to (e.g., "refs/remotes/origin/main" → "main").
//  2. git config init.defaultBranch — returns the user's configured default
//     branch (typically "main" or "master").
//  3. If both fail, returns an empty string.
//
// The caller is responsible for deciding what to do with an empty result
// (typically falling back to "main" or leaving BaseBranch empty for
// hardcoded-list reconciliation).
func DetectBaseBranch(git BaseBranchDetector) string {
	// Strategy 1: symbolic-ref refs/remotes/origin/HEAD
	ref, err := git.SymbolicRef("refs/remotes/origin/HEAD")
	if err == nil && ref != "" {
		// Strip "refs/remotes/origin/" prefix to get bare branch name
		branch := strings.TrimPrefix(ref, "refs/remotes/origin/")
		if branch != "" && branch != ref {
			// Prefix was successfully stripped (branch differs from ref)
			return branch
		}
	}

	// Strategy 2: init.defaultBranch from git config
	branch, err := git.ConfigGet("init.defaultBranch")
	if err == nil && branch != "" {
		return branch
	}

	// Strategy 3: both failed
	return ""
}