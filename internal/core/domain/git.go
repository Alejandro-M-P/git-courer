package domain

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

// Diff represents git diff output
type Diff struct {
	Files []DiffFile `json:"files"`
	Stats DiffStats  `json:"stats"`
	Raw   string     `json:"raw"`
}

// DiffFile represents diff for a single file
type DiffFile struct {
	Path      string `json:"path"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Raw       string `json:"raw"`
}

// DiffStats represents summary of changes
type DiffStats struct {
	FilesChanged int `json:"files_changed"`
	Additions    int `json:"additions"`
	Deletions    int `json:"deletions"`
}
