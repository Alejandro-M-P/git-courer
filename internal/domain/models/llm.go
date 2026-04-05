package models

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

// LLMPort defines what AI operations the core can do
type LLMPort interface {
	GenerateCommitMessage(diff string) (CommitMessage, error)
	GenerateSummary(diff string) (string, error)
	GenerateBranchName(task string) (string, error)
	DetectSecrets(files []string) ([]SecretDetection, error)
	IsAvailable() bool
}
