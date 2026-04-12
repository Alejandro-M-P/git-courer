package workflow

import (
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// mockGitForRelease implements ports.Git interface for testing.
type mockGitForRelease struct {
	latestTagResult    string
	latestTagErr       error
	commitsResult      string
	commitsErr         error
	listTagsResult     []string
	tagExistsResult    bool
	listBranchesResult string
}

// LatestTag implements ports.Git.
func (m *mockGitForRelease) LatestTag() (string, error) {
	return m.latestTagResult, m.latestTagErr
}

// ListTags implements ports.Git.
func (m *mockGitForRelease) ListTags() ([]string, error) {
	return m.listTagsResult, nil
}

// TagExists implements ports.Git.
func (m *mockGitForRelease) TagExists(name string) (bool, error) {
	return m.tagExistsResult, nil
}

// CommitsFromTag implements ports.Git.
func (m *mockGitForRelease) CommitsFromTag(s string) (string, error) {
	return m.commitsResult, m.commitsErr
}

// Tag implements ports.Git - returns (string, error) per interface.
func (m *mockGitForRelease) Tag(name string) (string, error) {
	return "", nil
}

// CurrentBranch implements ports.Git.
func (m *mockGitForRelease) CurrentBranch() (string, error) {
	return "develop", nil
}

// Log implements ports.Git.
func (m *mockGitForRelease) Log(n int) (string, error) {
	return "", nil
}

// ListBranches implements ports.Git.
func (m *mockGitForRelease) ListBranches() (string, error) {
	return m.listBranchesResult, nil
}

// IsRepo implements ports.Git.
func (m *mockGitForRelease) IsRepo() bool {
	return true
}

// Status implements ports.Git.
func (m *mockGitForRelease) Status() (domain.Status, error) {
	return domain.Status{}, nil
}

// Diff implements ports.Git.
func (m *mockGitForRelease) Diff() (string, error) {
	return "", nil
}

// DiffStaged implements ports.Git.
func (m *mockGitForRelease) DiffStaged() (string, error) {
	return "", nil
}

// ListUntracked implements ports.Git.
func (m *mockGitForRelease) ListUntracked() ([]string, error) {
	return nil, nil
}

// Show implements ports.Git.
func (m *mockGitForRelease) Show(commit string) (string, error) {
	return "", nil
}

// Blame implements ports.Git.
func (m *mockGitForRelease) Blame(file string) (string, error) {
	return "", nil
}

// Reflog implements ports.Git.
func (m *mockGitForRelease) Reflog(limit int) (string, error) {
	return "", nil
}

// Add implements ports.Git.
func (m *mockGitForRelease) Add(paths []string) error {
	return nil
}

// Remove implements ports.Git.
func (m *mockGitForRelease) Remove(paths []string) error {
	return nil
}

// Checkout implements ports.Git.
func (m *mockGitForRelease) Checkout(name string) (string, error) {
	return "", nil
}

// Switch implements ports.Git.
func (m *mockGitForRelease) Switch(name string) error {
	return nil
}

// Push implements ports.Git.
func (m *mockGitForRelease) Push() (string, error) {
	return "", nil
}

// PushWithUpstream implements ports.Git.
func (m *mockGitForRelease) PushWithUpstream(branch string) (string, error) {
	return "", nil
}

// Pull implements ports.Git.
func (m *mockGitForRelease) Pull() (string, error) {
	return "", nil
}

// PullRebase implements ports.Git.
func (m *mockGitForRelease) PullRebase() (string, error) {
	return "", nil
}

// Fetch implements ports.Git.
func (m *mockGitForRelease) Fetch() (string, error) {
	return "", nil
}

// Stash implements ports.Git.
func (m *mockGitForRelease) Stash() (string, error) {
	return "", nil
}

// StashPop implements ports.Git.
func (m *mockGitForRelease) StashPop() (string, error) {
	return "", nil
}

// ResetSoft implements ports.Git.
func (m *mockGitForRelease) ResetSoft(commits int) error {
	return nil
}

// Commit implements ports.Git.
func (m *mockGitForRelease) Commit(message string) (string, error) {
	return "", nil
}

// Branch implements ports.Git.
func (m *mockGitForRelease) Branch(name string) (string, error) {
	return "", nil
}

// DeleteBranch implements ports.Git.
func (m *mockGitForRelease) DeleteBranch(name string) (string, error) {
	return "", nil
}

// RenameBranch implements ports.Git.
func (m *mockGitForRelease) RenameBranch(oldName, newName string) (string, error) {
	return "", nil
}

// Merge implements ports.Git.
func (m *mockGitForRelease) Merge(branch string) (string, error) {
	return "", nil
}

// Rebase implements ports.Git.
func (m *mockGitForRelease) Rebase(branch string) (string, error) {
	return "", nil
}

// ResetHard implements ports.Git.
func (m *mockGitForRelease) ResetHard(commits int) error {
	return nil
}

// CherryPick implements ports.Git.
func (m *mockGitForRelease) CherryPick(commit string) (string, error) {
	return "", nil
}

// Revert implements ports.Git.
func (m *mockGitForRelease) Revert(commit string) (string, error) {
	return "", nil
}

// TagDelete implements ports.Git.
func (m *mockGitForRelease) TagDelete(name string) (string, error) {
	return "", nil
}

// DeleteTag implements ports.Git.
func (m *mockGitForRelease) DeleteTag(name string) (string, error) {
	return "", nil
}

// IsGHAuthenticated implements ports.Git.
func (m *mockGitForRelease) IsGHAuthenticated() (bool, error) {
	return true, nil
}

// AddRemote implements ports.Git.
func (m *mockGitForRelease) AddRemote(name, url string) (string, error) {
	return "", nil
}

// RemoveRemote implements ports.Git.
func (m *mockGitForRelease) RemoveRemote(name string) (string, error) {
	return "", nil
}

// Init implements ports.Git.
func (m *mockGitForRelease) Init() (string, error) {
	return "", nil
}

// Clone implements ports.Git.
func (m *mockGitForRelease) Clone(url string) (string, error) {
	return "", nil
}

// RebaseContinue implements ports.Git.
func (m *mockGitForRelease) RebaseContinue() (string, error) {
	return "", nil
}

// RebaseAbort implements ports.Git.
func (m *mockGitForRelease) RebaseAbort() (string, error) {
	return "", nil
}

// Reset implements ports.Git.
func (m *mockGitForRelease) Reset(mode, commit string) (string, error) {
	return "", nil
}

// Clean implements ports.Git.
func (m *mockGitForRelease) Clean(directories bool) (string, error) {
	return "", nil
}

// CreateRelease implements ports.Git.
func (m *mockGitForRelease) CreateRelease(name, changelog string) (string, error) {
	return "", nil
}

// mockLLMForRelease implements ports.LLM interface for testing.
type mockLLMForRelease struct {
	intentResult       *domain.ReleaseIntent
	chunkMessageResult string
	commitIntentResult *domain.CommitIntent
	gitOpResult        map[string]string
	availableResult    bool
	retryContext       string
}

// GenerateChunkMessage implements ports.LLM.
func (m *mockLLMForRelease) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) {
	return m.chunkMessageResult, nil
}

// DecideCommit implements ports.LLM.
func (m *mockLLMForRelease) DecideCommit(instruction, gitStatus, untracked, modified, deleted string) (domain.CommitIntent, error) {
	if m.commitIntentResult != nil {
		return *m.commitIntentResult, nil
	}
	return domain.CommitIntent{}, nil
}

// InterpretGitOp implements ports.LLM.
func (m *mockLLMForRelease) InterpretGitOp(op, instruction string, context map[string]string) (map[string]string, error) {
	return m.gitOpResult, nil
}

// SetRetryContext implements ports.LLM.
func (m *mockLLMForRelease) SetRetryContext(previousMessage string) {
	m.retryContext = previousMessage
}

// ClearRetryContext implements ports.LLM.
func (m *mockLLMForRelease) ClearRetryContext() {
	m.retryContext = ""
}

// IsAvailable implements ports.LLM.
func (m *mockLLMForRelease) IsAvailable() bool {
	return m.availableResult
}

// InterpretReleaseIntent implements ports.LLM.
func (m *mockLLMForRelease) InterpretReleaseIntent(i, r string) (*domain.ReleaseIntent, error) {
	return m.intentResult, nil
}

// GenerateChangelog implements ports.LLM.
func (m *mockLLMForRelease) GenerateChangelog(commits, previousChangelog, outputFile string) error {
	return nil
}

// PolishChangelog implements ports.LLM.
func (m *mockLLMForRelease) PolishChangelog(chunks []string) (string, error) {
	return "", nil
}

// mockLogChunker implements LogChunker interface for testing.
type mockLogChunker struct {
	chunksResult []string
	err          error
}

// Chunk implements LogChunker.
func (m *mockLogChunker) Chunk(commits string, maxPerChunk int) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	// Simple implementation: split by newlines
	lines := splitLines(commits)
	var result []string
	for i := 0; i < len(lines); i += maxPerChunk {
		end := i + maxPerChunk
		if end > len(lines) {
			end = len(lines)
		}
		result = append(result, joinLines(lines[i:end]))
	}
	if len(result) == 0 {
		result = []string{commits}
	}
	return result, nil
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var lines []string
	for _, line := range splitString(s, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func splitString(s, sep string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}

// TestDetectBranchFlow tests the branch flow detection.
func TestDetectBranchFlow(t *testing.T) {
	tests := []struct {
		name     string
		branches string
		want     string
		wantErr  bool
	}{
		{
			name:     "gitflow with develop and main",
			branches: "main\ndevelop\nfeature/foo",
			want:     "gitflow",
			wantErr:  false,
		},
		{
			name:     "gitflow with dev and master",
			branches: "master\ndev\nfeature/bar",
			want:     "gitflow",
			wantErr:  false,
		},
		{
			name:     "trunk with main only",
			branches: "main\nfeature/baz",
			want:     "trunk",
			wantErr:  false,
		},
		{
			name:     "trunk with master only",
			branches: "master",
			want:     "trunk",
			wantErr:  false,
		},
		{
			name:     "unknown - no main or master",
			branches: "develop\nfeature/test",
			want:     "unknown",
			wantErr:  false,
		},
		{
			name:     "empty branches",
			branches: "",
			want:     "unknown",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGit := &mockGitForRelease{
				listBranchesResult: tt.branches,
			}
			mockLLM := &mockLLMForRelease{}
			mockChunker := &mockLogChunker{}

			cfg := DefaultReleaseServiceConfig(4096, 20, 100, "/tmp/release_test.log")
			svc := NewReleaseService(mockGit, mockLLM, mockChunker, cfg)

			got, err := svc.DetectBranchFlow()
			if (err != nil) != tt.wantErr {
				t.Errorf("DetectBranchFlow() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("DetectBranchFlow() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestBuildPreview tests the release preview formatting.
func TestBuildPreview(t *testing.T) {
	tests := []struct {
		name         string
		intent       *domain.ReleaseIntent
		changelog    string
		wantContains []string
	}{
		{
			name: "basic preview with tag",
			intent: &domain.ReleaseIntent{
				TagName:     "v1.0.0",
				VersionBump: "minor",
			},
			changelog:    "## Added\n- Feature A\n\n## Fixed\n- Bug B",
			wantContains: []string{"📦 Release Preview", "Tag: v1.0.0", "Version Bump: minor", "## Added", "Feature A"},
		},
		{
			name: "preview without version bump",
			intent: &domain.ReleaseIntent{
				TagName: "v2.1.3",
			},
			changelog:    "No changes",
			wantContains: []string{"Tag: v2.1.3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGit := &mockGitForRelease{}
			mockLLM := &mockLLMForRelease{}
			mockChunker := &mockLogChunker{}

			cfg := DefaultReleaseServiceConfig(4096, 20, 100, "/tmp/release_test.log")
			svc := NewReleaseService(mockGit, mockLLM, mockChunker, cfg)

			got := svc.BuildPreview(tt.intent, tt.changelog)

			for _, want := range tt.wantContains {
				if !contains(got, want) {
					t.Errorf("BuildPreview() does not contain %q\nGot: %s", want, got)
				}
			}
		})
	}
}

// TestIsValidTagName tests the tag name validation.
func TestIsValidTagName(t *testing.T) {
	tests := []struct {
		tag  string
		want bool
	}{
		{"v1.0.0", true},
		{"1.0.0", true},
		{"v1.2.3-beta", true},
		{"1.2.3-rc1", true},
		{"v1.0", false},
		{"1.0.0.0", false},
		{"v1.0.0-beta.1", false},
		{"v1.0.0.1", false},
		{"", false},
		{"latest", false},
		{"v1.0.0-beta.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			got := domain.IsValidTagName(tt.tag)
			if got != tt.want {
				t.Errorf("IsValidTagName(%q) = %v, want %v", tt.tag, got, tt.want)
			}
		})
	}
}

// Helper function to check if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (len(s) == len(substr) || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
