package ports

import "github.com/Alejandro-M-P/git-courer/internal/core/domain"

// LLM defines the interface for AI/LLM operations.
type LLM interface {
	// GenerateChunkMessage generates a conventional commit message for a single diff chunk.
	GenerateChunkMessage(chunk domain.DiffChunk) (string, error)

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

	// InterpretReleaseIntent interprets user's release intent.
	InterpretReleaseIntent(instruction, releases, branches, currentBranch string) (*domain.ReleaseIntent, error)

	// GenerateChangelog generates changelog from commits and returns it.
	GenerateChangelog(commits, previousChangelog, outputFile string) (string, error)

	// PolishChangelog polishes the final changelog.
	PolishChangelog(chunks []string) (string, error)
}
