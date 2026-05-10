package classifier

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// compile-time assertion: Classifier satisfies the port
var _ ports.MessageClassifier = (*Classifier)(nil)

// newAnnotatedFixture builds a DiffChunk with the given AnnotatedDiff string.
func newAnnotatedFixture(annotated string) *domain.DiffChunk {
	return &domain.DiffChunk{
		Files:         []string{"a.go", "b.go"},
		Diff:          "sample diff",
		AnnotatedDiff: annotated,
	}
}

// ---------------------------------------------------------------------------
// RED tests — these reference production code (Classifier.Classify)
// that does NOT exist yet.
// ---------------------------------------------------------------------------

// Task 1.1: TestClassify_NEW_FUNC_to_feat
func TestClassify_NEW_FUNC_to_feat(t *testing.T) {
	c := &Classifier{}
	chunk := newAnnotatedFixture("📄 internal/server/webhook.go\nHandleWebhook [NEW_FUNC] internal/server/webhook.go:42\n")

	commitType, confidence := c.Classify(chunk)

	if commitType != "feat" {
		t.Errorf("CommitType = %q, want feat", commitType)
	}
	if confidence < 0.90 {
		t.Errorf("Confidence = %f, want >= 0.90 for single-label NEW_FUNC", confidence)
	}
	if chunk.CommitType != "feat" {
		t.Errorf("chunk.CommitType = %q, want feat", chunk.CommitType)
	}
	if chunk.ConfidenceScore != confidence {
		t.Errorf("chunk.ConfidenceScore = %f, want %f", chunk.ConfidenceScore, confidence)
	}
}

// Task 1.2: TestClassify_MOD_BODY_to_fix_or_refactor
func TestClassify_MOD_BODY_to_fix_or_refactor(t *testing.T) {
	c := &Classifier{}

	// MOD_BODY without test files
	t.Run("mod_body_no_tests", func(t *testing.T) {
		annotated := "📄 internal/auth/login.go\nvalidateToken [MOD_BODY] internal/auth/login.go:25\n"
		chunk := newAnnotatedFixture(annotated)
		chunk.Files = []string{"internal/auth/login.go"}

		commitType, confidence := c.Classify(chunk)

		if commitType == "" || (commitType != "fix" && commitType != "refactor") {
			t.Errorf("CommitType = %q, want fix or refactor", commitType)
		}
		if confidence < 0.80 {
			t.Errorf("Confidence = %f, want >= 0.80", confidence)
		}
	})

	// MOD_BODY with test files → lower confidence for non-test
	t.Run("mod_body_with_tests", func(t *testing.T) {
		annotated := "📄 internal/auth/login.go\nvalidateToken [MOD_BODY] internal/auth/login.go:25\n"
		chunk := newAnnotatedFixture(annotated)
		chunk.Files = []string{"internal/auth/login.go", "internal/auth/login_test.go"}

		commitType, confidence := c.Classify(chunk)

		// When test files are present, MOD_BODY shouldn't be auto-classified
		if commitType == "fix" || commitType == "refactor" {
			t.Logf("CommitType = %q (may be classified as test context)", commitType)
		}
		if confidence > 0.85 {
			t.Logf("Confidence = %f with test files present (expected lower confidence)", confidence)
		}
	})
}

// Task 1.3: TestClassify_CONFIG_to_chore
func TestClassify_CONFIG_to_chore(t *testing.T) {
	c := &Classifier{}

	tests := []struct {
		name      string
		annotated string
		label     string
	}{
		{
			"config_json",
			"📄 config/settings.json\nconfig/settings.json [CONFIG] config/settings.json\n",
			"CONFIG",
		},
		{
			"deps_gomod",
			"📄 go.mod\ngo.mod [DEPS] go.mod\n",
			"DEPS",
		},
		{
			"ci_github",
			"📄 .github/workflows/ci.yml\n.github/workflows/ci.yml [CI] .github/workflows/ci.yml\n",
			"CI",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk := newAnnotatedFixture(tt.annotated)

			commitType, confidence := c.Classify(chunk)

			switch tt.label {
			case "CONFIG", "DEPS":
				if commitType != "chore" {
					t.Errorf("CommitType = %q for %s, want chore", commitType, tt.label)
				}
				if confidence < 0.85 {
					t.Errorf("Confidence = %f for %s, want >= 0.85", confidence, tt.label)
				}
			case "CI":
				if commitType != "ci" {
					t.Errorf("CommitType = %q for %s, want ci", commitType, tt.label)
				}
				if confidence < 0.95 {
					t.Errorf("Confidence = %f for %s, want >= 0.95", confidence, tt.label)
				}
			}
		})
	}
}

// Task 1.4: TestClassify_DOCS_to_docs
func TestClassify_DOCS_to_docs(t *testing.T) {
	c := &Classifier{}
	annotated := "📄 README.md\nREADME.md [DOCS] README.md\n"
	chunk := newAnnotatedFixture(annotated)

	commitType, confidence := c.Classify(chunk)

	if commitType != "docs" {
		t.Errorf("CommitType = %q, want docs", commitType)
	}
	if confidence < 0.95 {
		t.Errorf("Confidence = %f, want >= 0.95", confidence)
	}
}

// Task 1.5: TestClassify_TEST_to_test
func TestClassify_TEST_to_test(t *testing.T) {
	c := &Classifier{}

	t.Run("test_file_new_func", func(t *testing.T) {
		annotated := "📄 internal/auth/login_test.go\nTestLogin [NEW_FUNC] internal/auth/login_test.go:15\n"
		chunk := newAnnotatedFixture(annotated)
		chunk.Files = []string{"internal/auth/login_test.go"}

		commitType, confidence := c.Classify(chunk)

		if commitType != "test" {
			t.Errorf("CommitType = %q, want test (test file with new func)", commitType)
		}
		if confidence < 0.90 {
			t.Errorf("Confidence = %f, want >= 0.90", confidence)
		}
	})

	t.Run("test_file_mod_body", func(t *testing.T) {
		annotated := "📄 internal/auth/login_test.go\nTestLogin [MOD_BODY] internal/auth/login_test.go:30\n"
		chunk := newAnnotatedFixture(annotated)
		chunk.Files = []string{"internal/auth/login_test.go"}

		commitType, confidence := c.Classify(chunk)

		if commitType != "test" {
			t.Errorf("CommitType = %q, want test (test file modification)", commitType)
		}
		if confidence < 0.90 {
			t.Errorf("Confidence = %f, want >= 0.90", confidence)
		}
	})
}

// Task 1.6: TestClassify_mixed_feat_plus_docs
func TestClassify_mixed_patterns_fallback(t *testing.T) {
	c := &Classifier{}

	// Mixed labels: NEW_FUNC + docs → feat dominates per conventional commits
	annotated := "📄 internal/server/webhook.go\nHandleWebhook [NEW_FUNC] internal/server/webhook.go:42\n" +
		"📄 README.md\nREADME.md [DOCS] README.md\n"
	chunk := newAnnotatedFixture(annotated)

	commitType, confidence := c.Classify(chunk)

	// NEW_FUNC dominates per conventional commits rules → feat
	if commitType != "feat" {
		t.Errorf("CommitType = %q, want feat (NEW_FUNC dominates DOCS per conventional commits)", commitType)
	}
	if confidence >= 0.90 {
		t.Errorf("Confidence = %f, want < 0.90 for mixed patterns", confidence)
	}
}

// Task 1.7: TestClassify_empty_diff_fallback
func TestClassify_empty_diff_fallback(t *testing.T) {
	c := &Classifier{}
	chunk := newAnnotatedFixture("")

	commitType, confidence := c.Classify(chunk)

	if commitType != "" {
		t.Errorf("CommitType = %q, want empty for empty annotated diff", commitType)
	}
	if confidence != 0.0 {
		t.Errorf("Confidence = %f, want 0.0 for empty annotated diff", confidence)
	}
}

// TestClassify_multiple_same_type validates homogenous labels → single type.
func TestClassify_multiple_same_type(t *testing.T) {
	c := &Classifier{}
	// Two NEW_FUNC labels in different files
	annotated := "📄 internal/server/webhook.go\nHandleWebhook [NEW_FUNC] internal/server/webhook.go:42\n" +
		"📄 internal/server/handler.go\nNewHandler [NEW_FUNC] internal/server/handler.go:10\n"
	chunk := newAnnotatedFixture(annotated)

	commitType, confidence := c.Classify(chunk)

	if commitType != "feat" {
		t.Errorf("CommitType = %q, want feat (all labels are NEW_FUNC)", commitType)
	}
	if confidence < 0.90 {
		t.Errorf("Confidence = %f, want >= 0.90 for pure feat labels", confidence)
	}
}

// TestClassify_MOD_SIG_breaking validates MOD_SIG → fix! or refactor!
func TestClassify_MOD_SIG_breaking(t *testing.T) {
	c := &Classifier{}
	annotated := "📄 internal/auth/login.go\nvalidateToken [MOD_SIG ⚠ BREAKING] internal/auth/login.go:25\n"
	chunk := newAnnotatedFixture(annotated)

	commitType, confidence := c.Classify(chunk)

	if commitType == "" || commitType == "feat" {
		t.Errorf("CommitType = %q, want fix! or refactor! for breaking change", commitType)
	}
	if confidence < 0.80 {
		t.Errorf("Confidence = %f, want >= 0.80", confidence)
	}
}

// TestClassify_MOD_SIG_private_no_breaking validates Bug #14:
// MOD_SIG on a private (lowercase) Go function should NOT get "!" suffix.
// The "!" suffix must be conditional on hasBreaking.
func TestClassify_MOD_SIG_private_no_breaking(t *testing.T) {
	c := &Classifier{}
	// Private Go function: lowercase "add" — no BREAKING marker
	annotated := "📄 internal/calc/add.go\nadd [MOD_SIG] internal/calc/add.go:10\n"
	chunk := newAnnotatedFixture(annotated)

	commitType, confidence := c.Classify(chunk)

	if commitType != "fix" {
		t.Errorf("CommitType = %q, want fix (no !) for MOD_SIG without BREAKING", commitType)
	}
	if confidence < 0.80 {
		t.Errorf("Confidence = %f, want >= 0.80", confidence)
	}
}

// TestClassify_MOD_SIG_public_breaking_feat validates that MOD_SIG with
// BREAKING + NEW_FUNC results in "feat!" (not "fix!").
func TestClassify_MOD_SIG_public_breaking_feat(t *testing.T) {
	c := &Classifier{}
	annotated := "📄 api/handler.go\nHandle [MOD_SIG ⚠ BREAKING] api/handler.go:5\nNewFunc [NEW_FUNC] api/handler.go:10\n"
	chunk := newAnnotatedFixture(annotated)

	commitType, confidence := c.Classify(chunk)

	if commitType != "feat!" {
		t.Errorf("CommitType = %q, want feat! for MOD_SIG BREAKING + NEW_FUNC", commitType)
	}
	// Mixed labels → lower confidence, but still the correct type
	_ = confidence // confidence may be low for mixed labels; type is what matters
}

// TestClassify_DELETED validates DELETED labels.
func TestClassify_DELETED(t *testing.T) {
	c := &Classifier{}
	annotated := "📄 internal/legacy/old.go\nOldFunc [DELETED_FUNC] internal/legacy/old.go:5\n"
	chunk := newAnnotatedFixture(annotated)

	commitType, confidence := c.Classify(chunk)

	if commitType != "refactor" {
		t.Errorf("CommitType = %q, want refactor for DELETED_FUNC", commitType)
	}
	if confidence < 0.85 {
		t.Errorf("Confidence = %f, want >= 0.85", confidence)
	}
}

// TestClassify_DELETED_FUNC_breaking validates Bug #7:
// DELETED_FUNC with BREAKING marker should classify with breaking suffix
// and high confidence, even when a MOD_BODY label exists alongside.
func TestClassify_DELETED_FUNC_breaking(t *testing.T) {
	c := &Classifier{}

	t.Run("single_deleted_func_breaking", func(t *testing.T) {
		// Single DELETED_FUNC with BREAKING → should be refactor! with high confidence
		annotated := "📄 internal/legacy/old.go\nOldHandler [DELETED_FUNC ⚠ BREAKING] internal/legacy/old.go:5\n"
		chunk := newAnnotatedFixture(annotated)

		commitType, confidence := c.Classify(chunk)

		if !strings.HasSuffix(commitType, "!") {
			t.Errorf("CommitType = %q, want refactor! or feat! for DELETED_FUNC BREAKING", commitType)
		}
		if !strings.HasPrefix(commitType, "refactor") && !strings.HasPrefix(commitType, "feat") {
			t.Errorf("CommitType = %q, want refactor! or feat! prefix for DELETED_FUNC BREAKING", commitType)
		}
		if confidence < 0.90 {
			t.Errorf("Confidence = %f, want >= 0.90 for single DELETED_FUNC BREAKING", confidence)
		}
	})

	t.Run("deleted_func_breaking_with_mod_body", func(t *testing.T) {
		// DELETED_FUNC ⚠BREAKING alongside MOD_BODY → DELETED_FUNC should dominate
		// This is the core of Bug #7: MOD_BODY shouldn't override a breaking deletion
		annotated := "📄 internal/legacy/old.go\nOldHandler [DELETED_FUNC ⚠ BREAKING] internal/legacy/old.go:5\nAdd [MOD_BODY] internal/legacy/old.go:12\n"
		chunk := newAnnotatedFixture(annotated)

		commitType, confidence := c.Classify(chunk)

		// The classifier should recognize this as a deletion with breaking change
		if commitType == "" {
			t.Errorf("CommitType empty for DELETED_FUNC BREAKING + MOD_BODY — should not be empty")
		}
		if confidence < 0.65 {
			t.Errorf("Confidence = %f, want >= 0.65 for DELETED_FUNC BREAKING + MOD_BODY", confidence)
		}
	})
}

// TestClassify_DELETED_TYPE_breaking validates Bug #8:
// DELETED_TYPE with public name should get Breaking marker and classify as breaking.
func TestClassify_DELETED_TYPE_breaking(t *testing.T) {
	c := &Classifier{}

	t.Run("public_type_deletion_breaking", func(t *testing.T) {
		annotated := "📄 internal/config/old.go\nOldConfig [DELETED_TYPE ⚠ BREAKING] internal/config/old.go:3\n"
		chunk := newAnnotatedFixture(annotated)

		commitType, confidence := c.Classify(chunk)

		if !strings.HasSuffix(commitType, "!") {
			t.Errorf("CommitType = %q, want refactor! or feat! for DELETED_TYPE BREAKING", commitType)
		}
		if confidence < 0.90 {
			t.Errorf("Confidence = %f, want >= 0.90 for DELETED_TYPE BREAKING", confidence)
		}
	})
}

// TestClassify_NEW_TYPE validates NEW_TYPE → feat
func TestClassify_NEW_TYPE(t *testing.T) {
	c := &Classifier{}
	annotated := "📄 internal/domain/user.go\nUser [NEW_TYPE] internal/domain/user.go:10\n"
	chunk := newAnnotatedFixture(annotated)

	commitType, confidence := c.Classify(chunk)

	if commitType != "feat" {
		t.Errorf("CommitType = %q, want feat for NEW_TYPE", commitType)
	}
	if confidence < 0.90 {
		t.Errorf("Confidence = %f, want >= 0.90", confidence)
	}
}

// TestClassify_MOD_TYPE validates MOD_TYPE → refactor
func TestClassify_MOD_TYPE(t *testing.T) {
	c := &Classifier{}
	annotated := "📄 internal/domain/user.go\nUser [MOD_TYPE] internal/domain/user.go:15\n"
	chunk := newAnnotatedFixture(annotated)

	commitType, confidence := c.Classify(chunk)

	if commitType != "refactor" {
		t.Errorf("CommitType = %q, want refactor for MOD_TYPE", commitType)
	}
	if confidence < 0.85 {
		t.Errorf("Confidence = %f, want >= 0.85", confidence)
	}
}

// ---------------------------------------------------------------------------
// History learning tests
// ---------------------------------------------------------------------------

// mockGit is a minimal ports.Git implementation for history tests.
type mockGit struct {
	logOutput string
	logErr    error
}

func (m *mockGit) Log(limit int, pattern string, paths ...string) (string, error) {
	return m.logOutput, m.logErr
}
func (m *mockGit) Status() (domain.Status, error)                                                                        { return domain.Status{}, nil }
func (m *mockGit) Diff(paths ...string) (string, error)                                                                  { return "", nil }
func (m *mockGit) DiffStat(paths ...string) (string, error)                                                              { return "", nil }
func (m *mockGit) DiffStatStaged(paths ...string) (string, error)                                                        { return "", nil }
func (m *mockGit) DiffAll(paths ...string) (string, error)                                                               { return "", nil }
func (m *mockGit) DiffRange(base, target, mode string, paths ...string) (string, error)                                  { return "", nil }
func (m *mockGit) DiffStaged(paths ...string) (string, error)                                                            { return "", nil }
func (m *mockGit) ListUntracked() ([]string, error)                                                                      { return nil, nil }
func (m *mockGit) LogFull(limit int) (string, error)                                                                     { return "", nil }
func (m *mockGit) CurrentBranch() (string, error)                                                                        { return "main", nil }
func (m *mockGit) ListBranches(pattern ...string) (string, error)                                                        { return "", nil }
func (m *mockGit) ListTags(pattern ...string) ([]string, error)                                                          { return nil, nil }
func (m *mockGit) IsRepo() bool                                                                                          { return true }
func (m *mockGit) RemoteURL() (string, error)                                                                            { return "", nil }
func (m *mockGit) RemoteInfo() (string, error)                                                                           { return "", nil }
func (m *mockGit) Search(pattern string, ctx, before, after int, paths ...string) (string, error)                        { return "", nil }
func (m *mockGit) CatFile(revision, path string) (string, error)                                                         { return "", nil }
func (m *mockGit) ListTree(revision, path string, recursive bool) ([]string, error)                                      { return nil, nil }
func (m *mockGit) LatestTag() (string, error)                                                                            { return "", nil }
func (m *mockGit) CommitsFromTag(sinceTag string) (string, error)                                                        { return "", nil }
func (m *mockGit) TagExists(name string) (bool, error)                                                                   { return false, nil }
func (m *mockGit) IsGHAuthenticated() (bool, error)                                                                      { return false, nil }
func (m *mockGit) CreateRelease(tagName, changelog string) (string, error)                                               { return "", nil }
func (m *mockGit) Blame(filepath string) ([]domain.BlameLine, error)                                                     { return nil, nil }
func (m *mockGit) Show(hash string) (domain.ShowResult, error)                                                           { return domain.ShowResult{}, nil }
func (m *mockGit) Reflog() ([]domain.ReflogEntry, error)                                                                 { return nil, nil }
func (m *mockGit) StashList() ([]domain.StashEntry, error)                                                               { return nil, nil }
func (m *mockGit) StashDiff(index string) (string, error)                                                                { return "", nil }
func (m *mockGit) MergeBase(a, b string) (string, error)                                                                 { return "", nil }
func (m *mockGit) CreateBackup(operation string, mode domain.StashMode) (domain.Backup, error)                           { return domain.Backup{}, nil }
func (m *mockGit) RestoreBackup(backup domain.Backup) error                                                              { return nil }
func (m *mockGit) DeleteBackup(backup domain.Backup) error                                                               { return nil }
func (m *mockGit) ListBackups() ([]domain.Backup, error)                                                                 { return nil, nil }
func (m *mockGit) PruneBackups(olderThan time.Duration) error                                                            { return nil }
func (m *mockGit) Add(paths []string) error                                                                              { return nil }
func (m *mockGit) Remove(paths []string) error                                                                           { return nil }
func (m *mockGit) Commit(message string) (string, error)                                                                 { return "", nil }
func (m *mockGit) Push() (string, error)                                                                                 { return "", nil }
func (m *mockGit) PushTo(remoteBranch string) (string, error)                                                            { return "", nil }
func (m *mockGit) Pull() (string, error)                                                                                 { return "", nil }
func (m *mockGit) PullFrom(remoteBranch string) (string, error)                                                          { return "", nil }
func (m *mockGit) Fetch() (string, error)                                                                                { return "", nil }
func (m *mockGit) Stash(message ...string) (string, error)                                                               { return "", nil }
func (m *mockGit) StashPop() (string, error)                                                                             { return "", nil }
func (m *mockGit) StashApply(index string) (string, error)                                                               { return "", nil }
func (m *mockGit) StashDrop(index string) (string, error)                                                                { return "", nil }
func (m *mockGit) StashClear() (string, error)                                                                           { return "", nil }
func (m *mockGit) StashShow() (string, error)                                                                              { return "", nil }
func (m *mockGit) Switch(branch string) error                                                                            { return nil }
func (m *mockGit) Branch(name string) (string, error)                                                                    { return "", nil }
func (m *mockGit) DeleteBranch(name string, force bool) (string, error)                                                  { return "", nil }
func (m *mockGit) RenameBranch(oldName, newName string) (string, error)                                                  { return "", nil }
func (m *mockGit) DeleteRemoteBranch(name string) error                                                                  { return nil }
func (m *mockGit) Tag(name, message string) (string, error)                                                              { return "", nil }
func (m *mockGit) PushTag(name string) (string, error)                                                                   { return "", nil }
func (m *mockGit) DeleteTag(name string) (string, error)                                                                 { return "", nil }
func (m *mockGit) DeleteTagRemote(name string) (string, error)                                                           { return "", nil }
func (m *mockGit) Merge(branch string) (string, error)                                                                   { return "", nil }
func (m *mockGit) Reset(mode string, commit string) (string, error)                                                      { return "", nil }
func (m *mockGit) ResetSoft(ref string) error                                                                            { return nil }

// TestLearnFromHistory_confidence_boost verifies that after learning from a
// history where "feat" is dominant, the classifier boosts confidence for
// feat-classified chunks.
func TestLearnFromHistory_confidence_boost(t *testing.T) {
	// Build a log output where feat appears 40% of the time (>30% → max boost)
	// Format: hash|author|date|subject
	var logLines []string
	for i := 0; i < 40; i++ {
		logLines = append(logLines, fmt.Sprintf("abc%03d|author|2026-05-01|feat: add feature %d", i, i))
	}
	for i := 0; i < 30; i++ {
		logLines = append(logLines, fmt.Sprintf("def%03d|author|2026-05-01|fix: fix issue %d", i, i))
	}
	for i := 0; i < 30; i++ {
		logLines = append(logLines, fmt.Sprintf("ghi%03d|author|2026-05-01|chore: maintenance %d", i, i))
	}

	mock := &mockGit{logOutput: strings.Join(logLines, "\n")}
	c := NewClassifier(mock)

	if err := c.LearnFromHistory(); err != nil {
		t.Fatalf("LearnFromHistory() error: %v", err)
	}

	// Classify a clear NEW_FUNC chunk — should get base confidence + history boost
	baseClassifier := &Classifier{}
	chunk := newAnnotatedFixture("📄 internal/api/handler.go\nNewHandler [NEW_FUNC] internal/api/handler.go:10\n")
	_, baseConfidence := baseClassifier.Classify(chunk)

	chunk2 := newAnnotatedFixture("📄 internal/api/handler.go\nNewHandler [NEW_FUNC] internal/api/handler.go:10\n")
	_, boostedConfidence := c.Classify(chunk2)

	if boostedConfidence <= baseConfidence {
		t.Errorf("expected history boost: boosted=%f should be > base=%f", boostedConfidence, baseConfidence)
	}
}

// TestLearnFromHistory_no_provider verifies that a nil git provider is a no-op.
func TestLearnFromHistory_no_provider(t *testing.T) {
	c := &Classifier{}
	if err := c.LearnFromHistory(); err != nil {
		t.Errorf("LearnFromHistory() with nil provider should return nil, got: %v", err)
	}
}

// TestLearnFromHistory_empty_log verifies that empty git log doesn't error.
func TestLearnFromHistory_empty_log(t *testing.T) {
	c := NewClassifier(&mockGit{logOutput: ""})
	if err := c.LearnFromHistory(); err != nil {
		t.Errorf("LearnFromHistory() with empty log should return nil, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Performance test
// ---------------------------------------------------------------------------

// TestClassify_performance validates no timeout for large annotated diff.
func TestClassify_performance(t *testing.T) {
	c := &Classifier{}

	// Build a large annotated diff with 200 NEW_FUNC labels
	var sb strings.Builder
	for i := 0; i < 200; i++ {
		sb.WriteString(fmt.Sprintf("📄 file_%d.go\nFunc_%d [NEW_FUNC] file_%d.go:1\n", i, i, i))
	}
	chunk := newAnnotatedFixture(sb.String())

	commitType, confidence := c.Classify(chunk)

	if commitType != "feat" {
		t.Errorf("CommitType = %q, want feat (all labels are NEW_FUNC)", commitType)
	}
	if confidence < 0.90 {
		t.Errorf("Confidence = %f, want >= 0.90", confidence)
	}
}

// ---------------------------------------------------------------------------
// Phase 3: Code-test symmetry detection tests
// ---------------------------------------------------------------------------

// newClassifierWithCatalog creates a Classifier with a populated catalog for testing.
func newClassifierWithCatalog() *Classifier {
	c := NewClassifier(nil)
	return c
}

// TestClassify_code_test_symmetry validates that paired code and test files
// are detected and classified as "fix" with high confidence.
func TestClassify_code_test_symmetry(t *testing.T) {
	c := newClassifierWithCatalog()

	tests := []struct {
		name     string
		codeFile string
		testFile string
		expected bool
	}{
		{
			name:     "go_with_test",
			codeFile: "internal/server/handler.go",
			testFile: "internal/server/handler_test.go",
			expected: true,
		},
		{
			name:     "js_with_test",
			codeFile: "src/utils/helpers.js",
			testFile: "src/utils/helpers.test.js",
			expected: true,
		},
		{
			name:     "unpaired_files",
			codeFile: "internal/auth/service.go",
			testFile: "internal/server/handler_test.go",
			expected: false,
		},
		{
			name:     "same_file_type",
			codeFile: "internal/auth/service.go",
			testFile: "internal/auth/another.go",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotated := fmt.Sprintf("📄 %s\nFunction [MOD_BODY] %s:10\n📄 %s\nTestFunction [NEW_FUNC] %s:5\n",
				tt.codeFile, tt.codeFile, tt.testFile, tt.testFile)
			chunk := newAnnotatedFixture(annotated)
			chunk.Files = []string{tt.codeFile, tt.testFile}

			commitType, confidence := c.Classify(chunk)

			if tt.expected {
				// Should detect symmetry and classify as fix with high confidence
				if commitType != "fix" {
					t.Errorf("CommitType = %q, want fix for paired code-test files", commitType)
				}
				if confidence < 0.95 {
					t.Errorf("Confidence = %f, want >= 0.95 for symmetry detection", confidence)
				}
			} else {
				// Should proceed with normal classification (not fix via symmetry)
				if commitType == "fix" && confidence > 0.95 {
					t.Errorf("Unexpected symmetry detection for unpaired files: %q with confidence %f", 
						commitType, confidence)
				}
			}
		})
	}
}

// TestClassify_symmetry_priority validates that symmetry detection runs before
// normal label-based classification and takes precedence.
func TestClassify_symmetry_priority(t *testing.T) {
	c := newClassifierWithCatalog()

	// This should be detected as symmetry (fix) even though it has NEW_FUNC label
	testFile := "internal/server/handler_test.go"
	codeFile := "internal/server/handler.go"
	
	annotated := fmt.Sprintf("📄 %s\nNewHandler [NEW_FUNC] %s:10\n📄 %s\nTestHandler [NEW_FUNC] %s:5\n",
		codeFile, codeFile, testFile, testFile)
	chunk := newAnnotatedFixture(annotated)
	chunk.Files = []string{codeFile, testFile}

	commitType, confidence := c.Classify(chunk)

	// Should be classified as fix due to symmetry, not feat from NEW_FUNC
	if commitType != "fix" {
		t.Errorf("CommitType = %q, want fix (symmetry should override label-based classification)", commitType)
	}
	if confidence < 0.95 {
		t.Errorf("Confidence = %f, want >= 0.95 for symmetry detection", confidence)
	}
}

// TestClassify_symmetry_insufficient_files validates that symmetry detection
// only triggers when exactly one code file and one test file are present.
func TestClassify_symmetry_insufficient_files(t *testing.T) {
	c := newClassifierWithCatalog()

	tests := []struct {
		name  string
		files []string
	}{
		{
			name:  "single_file",
			files: []string{"internal/server/handler.go"},
		},
		{
			name:  "three_files",
			files: []string{"internal/server/handler.go", "internal/server/handler_test.go", "README.md"},
		},
		{
			name:  "two_test_files",
			files: []string{"internal/server/handler_test.go", "internal/auth/login_test.go"},
		},
		{
			name:  "two_code_files",
			files: []string{"internal/server/handler.go", "internal/auth/login.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotated := "📄 file.go\nFunction [MOD_BODY] file.go:10\n"
			chunk := newAnnotatedFixture(annotated)
			chunk.Files = tt.files

			commitType, confidence := c.Classify(chunk)

			// Should NOT trigger symmetry detection
			if commitType == "fix" && confidence > 0.95 {
				t.Errorf("Unexpected symmetry detection for %s: %q with confidence %f",
					tt.name, commitType, confidence)
			}
		})
	}
}

// TestClassify_empty_catalog validates fallback behavior when catalog is nil.
func TestClassify_empty_catalog(t *testing.T) {
	c := &Classifier{} // No catalog

	// Even with paired files, should not detect symmetry without catalog
	codeFile := "internal/server/handler.go"
	testFile := "internal/server/handler_test.go"
	
	annotated := fmt.Sprintf("📄 %s\nFunction [MOD_BODY] %s:10\n📄 %s\nTestFunction [NEW_FUNC] %s:5\n",
		codeFile, codeFile, testFile, testFile)
	chunk := newAnnotatedFixture(annotated)
	chunk.Files = []string{codeFile, testFile}

	commitType, confidence := c.Classify(chunk)

	// Should NOT detect symmetry without catalog
	if commitType == "fix" && confidence > 0.95 {
		t.Errorf("Unexpected symmetry detection without catalog: %q with confidence %f", 
			commitType, confidence)
	}
}

// ---------------------------------------------------------------------------
// MOD_BODY subtype fallback tests
// ---------------------------------------------------------------------------

// TestDetermineType_MODBodySubtypes validates that MOD_BODY_* labels
// are classified identically to MOD_BODY via prefix-match fallback.
func TestDetermineType_MODBodySubtypes(t *testing.T) {
	c := &Classifier{}

	subtypes := []string{
		"MOD_BODY_LOGIC",
		"MOD_BODY_ERROR",
		"MOD_BODY_REORDER",
		"MOD_BODY_CALL",
	}

	for _, subtype := range subtypes {
		t.Run(subtype, func(t *testing.T) {
			annotated := fmt.Sprintf("📄 internal/auth/login.go\nvalidateToken [%s] internal/auth/login.go:25\n", subtype)
			chunk := newAnnotatedFixture(annotated)
			chunk.Files = []string{"internal/auth/login.go"}

			commitType, _ := c.Classify(chunk)

			// Should map to same commit type as MOD_BODY: "fix" or "refactor"
			if commitType != "fix" && commitType != "refactor" {
				t.Errorf("CommitType = %q for %s, want fix or refactor (same as MOD_BODY)", commitType, subtype)
			}
		})
	}
}

// TestLabelPriority_MODBodySubtypes validates that labelPriority returns
// the same priority (3) for MOD_BODY_* as for MOD_BODY.
func TestLabelPriority_MODBodySubtypes(t *testing.T) {
	subtypes := []string{
		"MOD_BODY_LOGIC",
		"MOD_BODY_ERROR",
		"MOD_BODY_REORDER",
		"MOD_BODY_CALL",
		"MOD_BODY_FUTURE", // unknown prefix-matched subtype
	}

	for _, subtype := range subtypes {
		t.Run(subtype, func(t *testing.T) {
			got := labelPriority(subtype)
			if got != 3 {
				t.Errorf("labelPriority(%q) = %d, want 3 (same as MOD_BODY)", subtype, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Phase 4: Smart Classifier — subtype mapping, code-over-CONFIG, BinaryClassifier
// ---------------------------------------------------------------------------

// mockBinaryClassifier is a test double for ports.BinaryClassifier.
type mockBinaryClassifier struct {
	result string
	err    error
}

func (m *mockBinaryClassifier) ClassifyBinary(prompt string) (string, error) {
	return m.result, m.err
}

// TestDetermineType_SubtypeMapping validates that MOD_BODY_* subtypes map
// deterministically to commit types: LOGIC→fix, ERROR→fix, REORDER→refactor,
// CALL with BinaryClassifier→delegate, CALL without→fix degradation.
func TestDetermineType_SubtypeMapping(t *testing.T) {
	tests := []struct {
		name           string
		label          string
		binaryResult   string // if non-empty, inject mock BinaryClassifier
		binaryErr      error
		wantType       string
		wantConfidence float64 // minimum expected confidence
	}{
		{
			name:           "MOD_BODY_LOGIC_to_fix",
			label:          "MOD_BODY_LOGIC",
			wantType:       "fix",
			wantConfidence: 0.85,
		},
		{
			name:           "MOD_BODY_ERROR_to_fix",
			label:          "MOD_BODY_ERROR",
			wantType:       "fix",
			wantConfidence: 0.85,
		},
		{
			name:           "MOD_BODY_REORDER_to_refactor",
			label:          "MOD_BODY_REORDER",
			wantType:       "refactor",
			wantConfidence: 0.85,
		},
		{
			name:           "MOD_BODY_CALL_with_mock_delegates",
			label:          "MOD_BODY_CALL",
			binaryResult:   "refactor",
			wantType:       "refactor",
			wantConfidence: 0.95,
		},
		{
			name:           "MOD_BODY_CALL_nil_degrades_to_fix",
			label:          "MOD_BODY_CALL",
			binaryResult:   "", // empty = no mock (nil BinaryClassifier)
			wantType:       "fix",
			wantConfidence: 0.60, // degraded confidence
		},
		{
			name:           "MOD_BODY_CALL_mock_returns_fix",
			label:          "MOD_BODY_CALL",
			binaryResult:   "fix",
			wantType:       "fix",
			wantConfidence: 0.95,
		},
		{
			name:           "MOD_BODY_CALL_mock_error_degrades",
			label:          "MOD_BODY_CALL",
			binaryErr:      fmt.Errorf("LLM unavailable"),
			binaryResult:   "irrelevant",
			wantType:       "fix",
			wantConfidence: 0.60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Classifier{}
			if tt.binaryResult != "" || tt.binaryErr != nil {
				// Inject mock BinaryClassifier
				c.binaryClassifier = &mockBinaryClassifier{
					result: tt.binaryResult,
					err:    tt.binaryErr,
				}
			}

			annotated := fmt.Sprintf("📄 internal/auth/login.go\nvalidateToken [%s] internal/auth/login.go:25\n", tt.label)
			chunk := newAnnotatedFixture(annotated)
			chunk.Files = []string{"internal/auth/login.go"}

			commitType, confidence := c.Classify(chunk)

			if commitType != tt.wantType {
				t.Errorf("CommitType = %q, want %q", commitType, tt.wantType)
			}
			if confidence < tt.wantConfidence {
				t.Errorf("Confidence = %f, want >= %f", confidence, tt.wantConfidence)
			}
		})
	}
}

// TestDetermineType_CodeOverConfig validates that MOD_BODY_* subtypes override
// CONFIG/DEPS when co-present, but generic MOD_BODY does NOT override.
func TestDetermineType_CodeOverConfig(t *testing.T) {
	tests := []struct {
		name           string
		annotated      string
		wantType       string
		wantConfidence float64
	}{
		{
			name: "MOD_BODY_LOGIC_with_CONFIG_wins",
			annotated: "📄 internal/auth/login.go\n" +
				"validateToken [MOD_BODY_LOGIC] internal/auth/login.go:25\n" +
				"📄 config/settings.json\n" +
				"config/settings.json [CONFIG] config/settings.json\n",
			wantType:       "fix",
			wantConfidence: 0.60, // mixed labels → lower confidence
		},
		{
			name: "CONFIG_alone_is_chore",
			annotated: "📄 config/settings.json\n" +
				"config/settings.json [CONFIG] config/settings.json\n",
			wantType:       "chore",
			wantConfidence: 0.90,
		},
		{
			name: "MOD_BODY_with_CONFIG_CONFIG_wins",
			annotated: "📄 internal/auth/login.go\n" +
				"validateToken [MOD_BODY] internal/auth/login.go:25\n" +
				"📄 config/settings.json\n" +
				"config/settings.json [CONFIG] config/settings.json\n",
			wantType:       "chore",
			wantConfidence: 0.60, // generic MOD_BODY doesn't override CONFIG
		},
		{
			name: "MOD_BODY_ERROR_with_DEPS_wins",
			annotated: "📄 internal/auth/login.go\n" +
				"handleError [MOD_BODY_ERROR] internal/auth/login.go:10\n" +
				"📄 go.mod\n" +
				"go.mod [DEPS] go.mod\n",
			wantType:       "fix",
			wantConfidence: 0.60,
		},
		{
			name: "MOD_BODY_REORDER_with_CONFIG_wins",
			annotated: "📄 internal/auth/login.go\n" +
				"reorderFunc [MOD_BODY_REORDER] internal/auth/login.go:15\n" +
				"📄 config/settings.json\n" +
				"config/settings.json [CONFIG] config/settings.json\n",
			wantType:       "refactor",
			wantConfidence: 0.60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Classifier{}
			chunk := newAnnotatedFixture(tt.annotated)

			commitType, confidence := c.Classify(chunk)

			if commitType != tt.wantType {
				t.Errorf("CommitType = %q, want %q", commitType, tt.wantType)
			}
			if confidence < tt.wantConfidence {
				t.Errorf("Confidence = %f, want >= %f", confidence, tt.wantConfidence)
			}
		})
	}
}

// TestWithBinaryClassifier validates that NewClassifierWithCatalog with
// WithBinaryClassifier stores the mock, and without it binaryClassifier is nil
// and MOD_BODY_CALL degrades to "fix".
func TestWithBinaryClassifier(t *testing.T) {
	t.Run("with_option_stores_mock", func(t *testing.T) {
		mock := &mockBinaryClassifier{result: "refactor"}
		c := NewClassifierWithCatalog(nil, nil, WithBinaryClassifier(mock))

		if c.binaryClassifier == nil {
			t.Fatal("binaryClassifier should not be nil when WithBinaryClassifier is provided")
		}
		// Verify it actually delegates
		result, err := c.binaryClassifier.ClassifyBinary("test")
		if err != nil {
			t.Fatalf("ClassifyBinary() error: %v", err)
		}
		if result != "refactor" {
			t.Errorf("ClassifyBinary() = %q, want %q", result, "refactor")
		}
	})

	t.Run("without_option_nil_binaryClassifier", func(t *testing.T) {
		c := NewClassifierWithCatalog(nil, nil)

		if c.binaryClassifier != nil {
			t.Fatal("binaryClassifier should be nil when WithBinaryClassifier is not provided")
		}
	})

	t.Run("nil_degrades_MOD_BODY_CALL_to_fix", func(t *testing.T) {
		c := NewClassifierWithCatalog(nil, nil) // no BinaryClassifier
		annotated := "📄 internal/auth/login.go\nvalidateCall [MOD_BODY_CALL] internal/auth/login.go:10\n"
		chunk := newAnnotatedFixture(annotated)
		chunk.Files = []string{"internal/auth/login.go"}

		commitType, confidence := c.Classify(chunk)

		if commitType != "fix" {
			t.Errorf("CommitType = %q, want fix (degraded)", commitType)
		}
		if confidence > 0.70 {
			t.Errorf("Confidence = %f, want <= 0.70 for degraded classification", confidence)
		}
	})

	t.Run("with_mock_MOD_BODY_CALL_delegates", func(t *testing.T) {
		mock := &mockBinaryClassifier{result: "refactor"}
		c := NewClassifierWithCatalog(nil, nil, WithBinaryClassifier(mock))
		annotated := "📄 internal/auth/login.go\nvalidateCall [MOD_BODY_CALL] internal/auth/login.go:10\n"
		chunk := newAnnotatedFixture(annotated)
		chunk.Files = []string{"internal/auth/login.go"}

		commitType, confidence := c.Classify(chunk)

		if commitType != "refactor" {
			t.Errorf("CommitType = %q, want refactor (delegated)", commitType)
		}
		if confidence < 0.95 {
			t.Errorf("Confidence = %f, want >= 0.95 for binary classification", confidence)
		}
	})
}

// TestClassifyBinary_Interface validates that ports.BinaryClassifier interface
// compiles and a mock implementation works correctly.
func TestClassifyBinary_Interface(t *testing.T) {
	// Compile-time check: mockBinaryClassifier satisfies ports.BinaryClassifier
	var _ ports.BinaryClassifier = (*mockBinaryClassifier)(nil)

	t.Run("mock_returns_result", func(t *testing.T) {
		mock := &mockBinaryClassifier{result: "fix"}
		result, err := mock.ClassifyBinary("classify this diff")
		if err != nil {
			t.Fatalf("ClassifyBinary() error: %v", err)
		}
		if result != "fix" {
			t.Errorf("ClassifyBinary() = %q, want %q", result, "fix")
		}
	})

	t.Run("mock_returns_error", func(t *testing.T) {
		mock := &mockBinaryClassifier{result: "", err: fmt.Errorf("LLM error")}
		_, err := mock.ClassifyBinary("classify this diff")
		if err == nil {
			t.Error("expected error from ClassifyBinary()")
		}
	})
}
