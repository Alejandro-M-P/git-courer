package domain

import (
	"regexp"
	"time"
)

// Status represents the current state of a git repository
// This is used by the git port interface
type Status struct {
	IsClean    bool         `json:"is_clean"`
	RepoPath   string       `json:"repo_path"`
	Branch     string       `json:"branch"`
	Files      []FileStatus `json:"files"`
	Staged     int          `json:"staged_count"`
	Modified   int          `json:"modified_count"`
	Untracked  int          `json:"untracked_count"`
	Conflicted int          `json:"conflicted_count"`
}

// FileStatus represents a single file in the repository
type FileStatus struct {
	Path      string `json:"path"`
	Status    string `json:"status"` // M, A, D, R, C, U, ?
	Staged    bool   `json:"staged"`
	IsNew     bool   `json:"is_new"`
	IsDeleted bool   `json:"is_deleted"`
	IsRenamed bool   `json:"is_renamed"`
}

// ReleaseIntent represents the user's intent to create a release.
type ReleaseIntent struct {
	TagName              string // e.g., "v1.2.0"
	IsRelease            bool   // true if "sacar version"
	VersionBump          string // "major", "minor", "patch"
	UserSpecifiedVersion bool   // true if user explicitly provided a version
	Changelog            string
	BranchFrom           string   // current branch
	MergePath            []string // e.g., ["feature/xxx->develop", "develop->main"]
}

// Release represents a created release.
type Release struct {
	Tag             string
	Changelog       string
	Version         string
	IsGitHubRelease bool
	CreatedAt       time.Time
}

// IsValidTagName validates tag name is valid semver
func IsValidTagName(tag string) bool {
	matched, err := regexp.MatchString(`^v?\d+\.\d+\.\d+(-[a-zA-Z0-9]+)?$`, tag)
	return err == nil && matched
}

// Backup holds the state saved before a destructive operation.
// It combines a git ref (HEAD snapshot) and an optional stash (working tree snapshot).
type Backup struct {
	Ref       string // refs/git-courer/backup/{timestamp}_{op}
	HasStash  bool   // true if we stashed changes before the operation
	Operation string // "commit", "merge", "release", "branch_delete"
	CreatedAt time.Time
}
