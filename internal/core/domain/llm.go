package domain

// CommitMessage represents an AI-generated commit message
type CommitMessage struct {
	Type    string // feat, fix, chore, docs, etc.
	Subject string // Short description
	Full    string // Full conventional commit message
}

// SecretDetection represents a detected secret
type SecretDetection struct {
	File    string
	Line    int
	Type    string // api_key, password, token, etc.
	Content string // The detected secret (redacted)
}

// CommitPlan - individual commit plan
type CommitPlan struct {
	Files    []string `json:"files"`    // files for this commit
	Commit   string   `json:"commit"`   // commit message
	Commands []string `json:"commands"` // git commands to execute
}

// CommitAnalysis represents the AI's analysis of files for commit planning
type CommitAnalysis struct {
	Strategy string         `json:"strategy"` // single | split
	Commits  []CommitGroup  `json:"commits"`
	Excluded []ExcludedFile `json:"excluded"`
	Warnings []string       `json:"warnings"`
}

// CommitGroup represents a logical group of files to commit together
type CommitGroup struct {
	Files   []string `json:"files"`
	Message string   `json:"message"`
	Type    string   `json:"type"`
}

// ExcludedFile represents a file that shouldn't be committed
type ExcludedFile struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
}

// CommitIntent represents the user's intent for what to commit.
type CommitIntent struct {
	// IncludeUntracked indicates whether to include new/untracked files.
	IncludeUntracked bool
	// Filter is a glob pattern to filter files (e.g., "internal/*", "src/**").
	// Empty means no filter.
	Filter string
	// Reasoning explains why these files were chosen.
	Reasoning string
	// FilesSelected lists files that will be included.
	FilesSelected []string
	// FilesExcluded lists files that will NOT be included and why.
	FilesExcluded []string
}
