package ports

import "github.com/Alejandro-M-P/git-courer/internal/core/domain"

// LLM defines the interface for AI/LLM operations.
type LLM interface {
	// GenerateChunkMessage generates a conventional commit message for a single diff chunk.
	GenerateChunkMessage(chunk domain.DiffChunk) (string, error)

	// GenerateCommitSynthesis synthesizes multiple file-by-file commit messages into a single conventional commit message.
	GenerateCommitSynthesis(combinedChunk domain.DiffChunk, fileMessages []string) (string, error)

	// DecideCommit determines what files to stage based on user instruction and git status.
	DecideCommit(instruction, gitStatus, untracked, modified, deleted string) (domain.CommitIntent, error)

	// InterpretGitOp interprets a natural language instruction for a given git operation.
	// Returns a map of concrete args (e.g. {"branch": "feat/login"}).
	// context provides extra git state (branches, log, tags) for better accuracy.
	InterpretGitOp(op, instruction string, context map[string]string) (map[string]string, error)

	// SetRetryContext stores a previously rejected message so the LLM generates a different one.
	SetRetryContext(previousMessage string)

	// ClearRetryContext clears the retry context after commit or abort.
	ClearRetryContext()

	// IsAvailable returns true if the LLM backend is reachable.
	IsAvailable() bool

	// VerifySecrets uses the LLM to verify if a diff contains sensitive information.
	VerifySecrets(diff string, findings []domain.SecretDetection) (bool, error)

	// AuditBinaryContent uses the LLM to determine if content is binary noise or legitimate text.
	AuditBinaryContent(filename, content string) (bool, error)

	// GenerateChangelog generates changelog from commits and returns structured data.
	GenerateChangelog(commits, previousChangelog, outputFile string) (*domain.Changelog, error)

	// GenerateChangelogByArea translates pre-filtered, area-grouped commits into user-facing release notes.
	// formattedGroups is the output of FormatGroupedCommits — areas with commit lists using group_N keys.
	// nameMap maps group_N keys back to area names (e.g. group_1 → "core").
	// The LLM never sees area names — only group_N. The adapter remaps group_N to area names in the response.
	GenerateChangelogByArea(formattedGroups string, nameMap map[string]string) (domain.ChangelogByArea, error)

	// GenerateChangelogGeneric generates a generic changelog without area grouping.
	// Uses changelog_generate prompt, returns Features/Fixes/Breaking format.
	GenerateChangelogGeneric(commits, previousChangelog, outputFile string) (*domain.Changelog, error)

	// RegenerateMessage generates new commit messages based on feedback.
	// Used when the user requests regeneration of commit messages in preview mode.
	RegenerateMessage(previousMessages []string, feedback string, chunks []domain.DiffChunk) ([]string, error)

	// ProjectInit analyzes the codebase and returns a suggested project description
	// and area-scope mappings for project initialization.
	ProjectInit(repoRoot string) (*domain.ProjectConfig, error)

	// ClassifyBinary categorizes a diff into exactly one of two categories with high precision.
	// Used for binary classification tasks like fix vs refactor determination.
	ClassifyBinary(prompt string) (string, error)
}

// Lifecycle manages provider-specific startup/shutdown.
// All providers implement this interface — for remote/local servers managed
// externally, EnsureRunning checks availability and Stop is a no-op.
type Lifecycle interface {
	// EnsureRunning checks if the provider is available and starts it if needed.
	// Returns true if the provider was started by this call.
	EnsureRunning() (bool, error)

	// PreWarm loads the model into memory before the first request.
	PreWarm() error

	// IsWarmed returns true if the model has been loaded into memory.
	IsWarmed() bool

	// Stop gracefully shuts down the provider if it was started by EnsureRunning.
	Stop()
}
