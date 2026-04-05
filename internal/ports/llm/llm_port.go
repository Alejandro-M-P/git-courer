package llm

// Port defines what AI operations the core can do
// The core uses this interface, the adapter implements it
type Port interface {
	// GenerateCommitMessage creates a commit message from diff
	GenerateCommitMessage(diff string) (CommitMessage, error)

	// GenerateSummary creates a human-readable summary of changes
	GenerateSummary(diff string) (string, error)

	// GenerateBranchName suggests a branch name based on task
	GenerateBranchName(task string) (string, error)

	// DetectSecrets checks if files contain secrets
	DetectSecrets(files []string) ([]SecretDetection, error)

	// IsAvailable checks if the AI service is running
	IsAvailable() bool
}

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
