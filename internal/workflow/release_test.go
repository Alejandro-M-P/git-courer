package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// mockGitForRelease implements ports.Git interface for testing.
type mockGitForRelease struct {
	latestTagResult         string
	latestTagErr            error
	commitsResult           string
	commitsErr              error
	listTagsResult          []string
	tagExistsResult         bool
	isGHAuthenticatedResult bool
	isGHAuthenticatedCalled bool
	createReleaseTag        string
	createReleaseChangelog  string
	createReleaseResult     func(tag, changelog string) (string, error)
	listBranchesResult      string
	tagCreated              bool
	changelogResult         string
}

func (m *mockGitForRelease) Status() (domain.Status, error)           { return domain.Status{}, nil }
func (m *mockGitForRelease) Diff(paths ...string) (string, error)     { return "", nil }
func (m *mockGitForRelease) DiffStaged(paths ...string) (string, error) { return "", nil }
func (m *mockGitForRelease) ListUntracked() ([]string, error)   { return nil, nil }
func (m *mockGitForRelease) Log(limit int, paths ...string) (string, error) { return "", nil }
func (m *mockGitForRelease) LogFull(limit int) (string, error)  { return "", nil }
func (m *mockGitForRelease) CurrentBranch() (string, error)   { return "develop", nil }
func (m *mockGitForRelease) ListBranches(pattern ...string) (string, error) { return m.listBranchesResult, nil }
func (m *mockGitForRelease) ListTags(pattern ...string) ([]string, error) { return m.listTagsResult, nil }
func (m *mockGitForRelease) IsRepo() bool                       { return true }
func (m *mockGitForRelease) LatestTag() (string, error) {
	if m.latestTagResult != "" {
		return m.latestTagResult, m.latestTagErr
	}
	return "v1.0.0", nil
}
func (m *mockGitForRelease) CommitsFromTag(sinceTag string) (string, error) {
	if m.commitsResult != "" {
		return m.commitsResult, m.commitsErr
	}
	return "feat: add two-factor authentication to login page\nfix: user list no longer crashes when projects have no users assigned", m.commitsErr
}
func (m *mockGitForRelease) TagExists(name string) (bool, error) { return m.tagExistsResult, nil }
func (m *mockGitForRelease) DeleteTag(name string) (string, error) { return "", nil }
func (m *mockGitForRelease) DeleteTagRemote(name string) (string, error) { return "", nil }
func (m *mockGitForRelease) PushTag(name string) (string, error) { return "", nil }
func (m *mockGitForRelease) PushTags() (string, error) { return "", nil }
func (m *mockGitForRelease) IsGHAuthenticated() (bool, error) {
	m.isGHAuthenticatedCalled = true
	if m.isGHAuthenticatedResult {
		return true, nil
	}
	return false, nil
}
func (m *mockGitForRelease) CreateRelease(name, changelog string) (string, error) {
	m.createReleaseTag = name
	m.createReleaseChangelog = changelog
	if m.createReleaseResult != nil {
		return m.createReleaseResult(name, changelog)
	}
	m.changelogResult = changelog
	return "", nil
}
func (m *mockGitForRelease) CreateBackup(operation string, stashUntracked bool) (domain.Backup, error) {
	return domain.Backup{}, nil
}
func (m *mockGitForRelease) RestoreBackup(backup domain.Backup) error { return nil }
func (m *mockGitForRelease) DeleteBackup(backup domain.Backup) error { return nil }
func (m *mockGitForRelease) Add(paths []string) error             { return nil }
func (m *mockGitForRelease) Remove(paths []string) error         { return nil }
func (m *mockGitForRelease) Checkout(name string) (string, error)  { return "", nil }
func (m *mockGitForRelease) Switch(name string) error                 { return nil }
func (m *mockGitForRelease) Push() (string, error)                { return "", nil }
func (m *mockGitForRelease) Pull() (string, error)                { return "", nil }
func (m *mockGitForRelease) Fetch() (string, error)                { return "", nil }
func (m *mockGitForRelease) Stash() (string, error)                { return "", nil }
func (m *mockGitForRelease) StashPop() (string, error)          { return "", nil }
func (m *mockGitForRelease) Commit(message string) (string, error) { return "", nil }
func (m *mockGitForRelease) Branch(name string) (string, error)  { m.tagCreated = true; return "", nil }
func (m *mockGitForRelease) RenameBranch(oldName, newName string) (string, error) { return "", nil }
func (m *mockGitForRelease) DeleteBranch(name string) (string, error) { return "", nil }
func (m *mockGitForRelease) Reset(mode string, commit string) (string, error) { return "", nil }
func (m *mockGitForRelease) Merge(branch string) (string, error)  { return "", nil }
func (m *mockGitForRelease) Tag(name string) (string, error) {
	m.tagCreated = true
	return "", nil
}

// mockLLMForRelease implements ports.LLM interface for testing.
type mockLLMForRelease struct {
	intentResult       *domain.ReleaseIntent
	changelogResult    string
	commitIntentResult *domain.CommitIntent
	gitOpResult        map[string]string
	availableResult    bool
	retryContext       string
}

// GenerateChunkMessage implements ports.LLM.
func (m *mockLLMForRelease) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) {
	return "", nil
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

// VerifySecrets implements ports.LLM.
func (m *mockLLMForRelease) VerifySecrets(diff string, findings []domain.SecretDetection) (bool, error) {
	return false, nil
}

// AuditBinaryContent implements ports.LLM.
func (m *mockLLMForRelease) AuditBinaryContent(filename, content string) (bool, error) {
	return false, nil
}

// GenerateChangelog implements ports.LLM.
func (m *mockLLMForRelease) GenerateChangelog(commits, previousChangelog, outputFile string) (string, error) {
	if m.changelogResult != "" {
		return m.changelogResult, nil
	}
	return "## Changelog\n- feat: changes", nil
}

// RegenerateMessage implements ports.LLM.
func (m *mockLLMForRelease) RegenerateMessage(previousMessages []string, feedback string, chunks []domain.DiffChunk) ([]string, error) {
	if len(previousMessages) != len(chunks) {
		return nil, fmt.Errorf("mock: count mismatch")
	}
	newMessages := make([]string, len(previousMessages))
	for i, msg := range previousMessages {
		newMessages[i] = msg + " (regenerated)"
	}
	return newMessages, nil
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

// TestExecute_WritesChangelog verifies that Execute writes the changelog to disk when configured.
func TestExecute_WritesChangelog(t *testing.T) {
	tmpDir := t.TempDir()
	changelogPath := filepath.Join(tmpDir, "changelog.md")

	mockGit := &mockGitForRelease{}
	mockLLM := &mockLLMForRelease{}
	mockChunker := &mockLogChunker{}

	cfg := DefaultReleaseServiceConfig(4096, 20, 100, filepath.Join(tmpDir, "release.log"))
	cfg.ChangelogOutputPath = changelogPath
	svc := NewReleaseService(mockGit, mockLLM, mockChunker, cfg)

	intent := &domain.ReleaseIntent{TagName: "v1.2.3"}
	changelog := "## v1.2.3\n- feat: new stuff"

	_, err := svc.Execute(intent, changelog, false)
	if err != nil {
		// writeChangelogFile might succeed but other parts could fail with mock; focus on file presence
	}

	got, readErr := os.ReadFile(changelogPath)
	if readErr != nil {
		t.Fatalf("expected changelog file at %s, got error: %v", changelogPath, readErr)
	}
	if string(got) != changelog {
		t.Errorf("changelog content = %q, want %q", string(got), changelog)
	}
}

// TestWriteChangelogFile_CreatesDirectories verifies that missing directories are created.
func TestWriteChangelogFile_CreatesDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	changelogPath := filepath.Join(tmpDir, "deep", "nested", "changelog.md")

	mockGit := &mockGitForRelease{}
	mockLLM := &mockLLMForRelease{}
	mockChunker := &mockLogChunker{}

	cfg := DefaultReleaseServiceConfig(4096, 20, 100, filepath.Join(tmpDir, "release.log"))
	cfg.ChangelogOutputPath = changelogPath
	svc := NewReleaseService(mockGit, mockLLM, mockChunker, cfg)

	content := "## Changelog\n- fix: bug"
	if err := svc.writeChangelogFile(content); err != nil {
		t.Fatalf("writeChangelogFile() error = %v", err)
	}

	got, err := os.ReadFile(changelogPath)
	if err != nil {
		t.Fatalf("expected file at %s, got error: %v", changelogPath, err)
	}
	if string(got) != content {
		t.Errorf("file content = %q, want %q", string(got), content)
	}
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
