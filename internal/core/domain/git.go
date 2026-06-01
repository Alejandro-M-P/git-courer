package domain

import (
	"regexp"
	"time"
)

// Status represents the current state of a git repository
// This is used by the git port interface
type Status struct {
	IsClean     bool         `json:"is_clean"`
	RepoPath    string       `json:"repo_path"`
	Branch      string       `json:"branch"`
	Ahead       int          `json:"ahead,omitempty"`
	Behind      int          `json:"behind,omitempty"`
	HasUpstream bool         `json:"has_upstream"`
	Files       []FileStatus `json:"files"`
	Staged      int          `json:"staged_count"`
	Modified    int          `json:"modified_count"`
	Untracked   int          `json:"untracked_count"`
	Conflicted  int          `json:"conflicted_count"`
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
	TagName              string `json:"tag_name,omitempty"`
	IsRelease            bool   `json:"is_release"`
	VersionBump          string `json:"version_bump,omitempty"`
	UserSpecifiedVersion bool   `json:"user_specified_version,omitempty"`
	CustomTagMessage     string `json:"custom_tag_message,omitempty"`
}

// Pattern: v?MAJOR.MINOR.PATCH(-[a-zA-Z0-9]+(\.[a-zA-Z0-9-]+)*)?
var validTagNameRe = regexp.MustCompile(`^v?\d+\.\d+\.\d+(-[a-zA-Z0-9]+(\.[a-zA-Z0-9-]+)*)?$`)

// IsValidTagName validates tag name is valid semver
func IsValidTagName(tag string) bool {
	return validTagNameRe.MatchString(tag)
}

// StashMode defines how the working tree should be handled during a backup.
type StashMode int

const (
	StashNone     StashMode = iota // No stash created, only HEAD reference saved
	StashUnstaged                  // Stash tracked unstaged changes, keep untracked files
	StashAll                       // Stash tracked unstaged and untracked files
)

// Backup holds the state saved before a destructive operation.
// It combines a git ref (HEAD snapshot) and an optional stash (working tree snapshot).
type Backup struct {
	Ref       string // refs/git-courer/backup/{timestamp}_{op}
	HasStash  bool   // true if we stashed changes before the operation
	Operation string // "commit", "merge", "release", "branch_delete"
	CreatedAt time.Time
	StashMode StashMode
	Undoable  bool
}

// BlameLine represents a single line from git blame output.
type BlameLine struct {
	Line   int
	Author string
	Hash   string
}

// ShowResult represents a single commit with stats.
type ShowResult struct {
	Hash      string
	Author    string
	Date      string
	Message   string
	Files     int
	Additions int
	Deletions int
	FileList  []string
}

// ReflogEntry represents a single reflog entry.
type ReflogEntry struct {
	Index  int
	Action string
	Hash   string
}

// StashEntry represents a single stash entry.
type StashEntry struct {
	Index   int
	Message string
	Hash    string
}
