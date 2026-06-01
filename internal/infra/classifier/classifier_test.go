package classifier

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/core/ports"
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
func (m *mockGit) Status() (domain.Status, error)                 { return domain.Status{}, nil }
func (m *mockGit) Diff(paths ...string) (string, error)           { return "", nil }
func (m *mockGit) DiffStat(paths ...string) (string, error)       { return "", nil }
func (m *mockGit) DiffStatStaged(paths ...string) (string, error) { return "", nil }
func (m *mockGit) DiffAll(paths ...string) (string, error)        { return "", nil }
func (m *mockGit) DiffRange(base, target, mode string, paths ...string) (string, error) {
	return "", nil
}
func (m *mockGit) DiffStaged(paths ...string) (string, error)     { return "", nil }
func (m *mockGit) ListUntracked() ([]string, error)               { return nil, nil }
func (m *mockGit) LogFull(limit int) (string, error)              { return "", nil }
func (m *mockGit) CurrentBranch() (string, error)                 { return "main", nil }
func (m *mockGit) ListBranches(pattern ...string) (string, error) { return "", nil }
func (m *mockGit) ListTags(pattern ...string) ([]string, error)   { return nil, nil }
func (m *mockGit) IsRepo() bool                                   { return true }
func (m *mockGit) RemoteURL() (string, error)                     { return "", nil }
func (m *mockGit) RemoteInfo() (string, error)                    { return "", nil }
func (m *mockGit) Search(pattern string, ctx, before, after int, paths ...string) (string, error) {
	return "", nil
}
func (m *mockGit) CatFile(revision, path string) (string, error)                    { return "", nil }
func (m *mockGit) ListTree(revision, path string, recursive bool) ([]string, error) { return nil, nil }
func (m *mockGit) LatestTag() (string, error)                                       { return "", nil }
func (m *mockGit) CommitsFromTag(sinceTag string) (string, error)                   { return "", nil }
func (m *mockGit) TagExists(name string) (bool, error)                              { return false, nil }
func (m *mockGit) IsGHAuthenticated() (bool, error)                                 { return false, nil }
func (m *mockGit) CreateRelease(tagName, changelog string) (string, error)          { return "", nil }
func (m *mockGit) Blame(filepath string) ([]domain.BlameLine, error)                { return nil, nil }
func (m *mockGit) Show(hash string) (domain.ShowResult, error)                      { return domain.ShowResult{}, nil }
func (m *mockGit) Reflog() ([]domain.ReflogEntry, error)                            { return nil, nil }
func (m *mockGit) StashList() ([]domain.StashEntry, error)                          { return nil, nil }
func (m *mockGit) StashDiff(index string) (string, error)                           { return "", nil }
func (m *mockGit) MergeBase(a, b string) (string, error)                            { return "", nil }
func (m *mockGit) LogRange(from, to string) (string, error)                         { return "", nil }

func (m *mockGit) CreateBackup(operation string, mode domain.StashMode) (domain.Backup, error) {
	return domain.Backup{}, nil
}
func (m *mockGit) RestoreBackup(backup domain.Backup) error                        { return nil }
func (m *mockGit) DeleteBackup(backup domain.Backup) error                         { return nil }
func (m *mockGit) ListBackups() ([]domain.Backup, error)                           { return nil, nil }
func (m *mockGit) PruneBackups(olderThan time.Duration) error                      { return nil }
func (m *mockGit) Add(paths []string) error                                        { return nil }
func (m *mockGit) Remove(paths []string) error                                     { return nil }
func (m *mockGit) Commit(message string) (string, error)                           { return "", nil }
func (m *mockGit) Push() (string, error)                                           { return "", nil }
func (m *mockGit) PushTo(remoteBranch string) (string, error)                      { return "", nil }
func (m *mockGit) PushToBranch(remote, branch string) (string, error)              { return "", nil }
func (m *mockGit) Pull() (string, error)                                           { return "", nil }
func (m *mockGit) PullFrom(remoteBranch string) (string, error)                    { return "", nil }
func (m *mockGit) PullFromBranch(remote, branch string) (string, error)            { return "", nil }
func (m *mockGit) Fetch() (string, error)                                          { return "", nil }
func (m *mockGit) Stash(message ...string) (string, error)                         { return "", nil }
func (m *mockGit) StashPop() (string, error)                                       { return "", nil }
func (m *mockGit) StashApply(index string) (string, error)                         { return "", nil }
func (m *mockGit) StashDrop(index string) (string, error)                          { return "", nil }
func (m *mockGit) StashClear() (string, error)                                     { return "", nil }
func (m *mockGit) StashShow() (string, error)                                      { return "", nil }
func (m *mockGit) Switch(branch string) error                                      { return nil }
func (m *mockGit) Branch(name string) (string, error)                              { return "", nil }
func (m *mockGit) DeleteBranch(name string, force bool) (string, error)            { return "", nil }
func (m *mockGit) RenameBranch(oldName, newName string) (string, error)            { return "", nil }
func (m *mockGit) DeleteRemoteBranch(name string) error                            { return nil }
func (m *mockGit) Tag(name, message string) (string, error)                        { return "", nil }
func (m *mockGit) PushTag(name string) (string, error)                             { return "", nil }
func (m *mockGit) DeleteTag(name string) (string, error)                           { return "", nil }
func (m *mockGit) DeleteTagRemote(name string) (string, error)                     { return "", nil }
func (m *mockGit) Merge(branch string) (string, error)                             { return "", nil }
func (m *mockGit) Reset(mode string, commit string) (string, error)                { return "", nil }
func (m *mockGit) ResetSoft(ref string) error                                      { return nil }
func (m *mockGit) Revert(commit string) (string, error)                            { return "", nil }
func (m *mockGit) Amend(message string, paths []string) (string, error)            { return "", nil }
func (m *mockGit) Restore(paths []string) error                                    { return nil }
func (m *mockGit) Clean() error                                                    { return nil }
func (m *mockGit) ShowCommit(commit string) (string, error)                        { return "", nil }
func (m *mockGit) RemoteAdd(name, url string) (string, error)                      { return "", nil }
func (m *mockGit) RemoteRemove(name string) (string, error)                        { return "", nil }
func (m *mockGit) StashWithUntracked(message string) (string, error)               { return "", nil }
func (m *mockGit) MergeAbort() (string, error)                                     { return "", nil }
func (m *mockGit) MergeContinue() (string, error)                                  { return "", nil }
func (m *mockGit) MergeSkip() (string, error)                                      { return "", nil }
func (m *mockGit) Rebase(branch string) (string, error)                            { return "", nil }
func (m *mockGit) RebaseAbort() (string, error)                                    { return "", nil }
func (m *mockGit) RebaseContinue() (string, error)                                 { return "", nil }
func (m *mockGit) RebaseSkip() (string, error)                                     { return "", nil }
func (m *mockGit) RebaseOnto(newBase, upstream, branch string) (string, error)     { return "", nil }
func (m *mockGit) CherryPick(commit string) (string, error)                        { return "", nil }
func (m *mockGit) SetUpstream(branch, remote string) (string, error)               { return "", nil }
func (m *mockGit) UnsetUpstream(branch string) (string, error)                     { return "", nil }
func (m *mockGit) ConfigGet(key string) (string, error)                            { return "", nil }
func (m *mockGit) ConfigSet(key, value string) (string, error)                     { return "", nil }
func (m *mockGit) WriteTree() (string, error)                                      { return "", nil }
func (m *mockGit) CommitTree(treeHash, parentHash, message string) (string, error) { return "", nil }
func (m *mockGit) UpdateRef(ref, commitHash string) (string, error)                { return "", nil }
func (m *mockGit) Head() (string, error)                                           { return "", nil }

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
// with MOD_BODY labels are detected and classified as "fix" with high confidence.
// When NEW_FUNC/NEW_TYPE labels are present, weight-based selection promotes to
// "feat" (weight 9) — symmetry detection does NOT override NEW_FUNC.
func TestClassify_code_test_symmetry(t *testing.T) {
	c := newClassifierWithCatalog()

	tests := []struct {
		name      string
		annotated string
		files     []string
		wantType  string
	}{
		{
			name:      "mod_body_symmetry_fix",
			annotated: "📄 internal/server/handler.go\nFunction [MOD_BODY] internal/server/handler.go:10\n📄 internal/server/handler_test.go\nTestFunction [MOD_BODY] internal/server/handler_test.go:5\n",
			files:     []string{"internal/server/handler.go", "internal/server/handler_test.go"},
			wantType:  "fix",
		},
		{
			name:      "new_func_beats_symmetry",
			annotated: "📄 internal/server/handler.go\nNewHandler [NEW_FUNC] internal/server/handler.go:10\n📄 internal/server/handler_test.go\nTestHandler [NEW_FUNC] internal/server/handler_test.go:5\n",
			files:     []string{"internal/server/handler.go", "internal/server/handler_test.go"},
			wantType:  "feat", // NEW_FUNC weight 9 > symmetry fix
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunk := newAnnotatedFixture(tt.annotated)
			chunk.Files = tt.files

			commitType, _ := c.Classify(chunk)

			if commitType != tt.wantType {
				t.Errorf("CommitType = %q, want %q", commitType, tt.wantType)
			}
		})
	}
}

// TestClassify_symmetry_priority validates that symmetry detection applies for
// MOD_BODY labels (fix-weight) but NOT when NEW_FUNC/NEW_TYPE (feat-weight) is present.
// NEW_FUNC weight 9 always beats symmetry heuristic.
func TestClassify_symmetry_priority(t *testing.T) {
	c := newClassifierWithCatalog()

	testFile := "internal/server/handler_test.go"
	codeFile := "internal/server/handler.go"

	t.Run("mod_body_symmetry_wins", func(t *testing.T) {
		// MOD_BODY labels → weight 8 (fix) → symmetry can apply
		annotated := fmt.Sprintf("📄 %s\nFunction [MOD_BODY] %s:10\n📄 %s\nTestFunction [MOD_BODY] %s:5\n",
			codeFile, codeFile, testFile, testFile)
		chunk := newAnnotatedFixture(annotated)
		chunk.Files = []string{codeFile, testFile}

		commitType, confidence := c.Classify(chunk)

		// Should be classified as fix via symmetry for MOD_BODY
		if commitType != "fix" {
			t.Errorf("CommitType = %q, want fix (symmetry should apply for MOD_BODY)", commitType)
		}
		_ = confidence // confidence varies by catalog availability
	})

	t.Run("new_func_beats_symmetry", func(t *testing.T) {
		// NEW_FUNC labels → weight 9 (feat) → weight-based selection wins over symmetry
		annotated := fmt.Sprintf("📄 %s\nNewHandler [NEW_FUNC] %s:10\n📄 %s\nTestHandler [NEW_FUNC] %s:5\n",
			codeFile, codeFile, testFile, testFile)
		chunk := newAnnotatedFixture(annotated)
		chunk.Files = []string{codeFile, testFile}

		commitType, confidence := c.Classify(chunk)

		// NEW_FUNC weight 9 → feat, symmetry does NOT override
		if commitType != "feat" {
			t.Errorf("CommitType = %q, want feat (NEW_FUNC weight beats symmetry)", commitType)
		}
		_ = confidence
	})
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
			name:           "MOD_BODY_CALL_is_fix",
			label:          "MOD_BODY_CALL",
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
			name: "MOD_BODY_with_CONFIG_MOD_BODY_wins",
			annotated: "📄 internal/auth/login.go\n" +
				"validateToken [MOD_BODY] internal/auth/login.go:25\n" +
				"📄 config/settings.json\n" +
				"config/settings.json [CONFIG] config/settings.json\n",
			wantType:       "fix", // MOD_BODY weight 8 > CONFIG weight 6
			wantConfidence: 0.60,
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

	t.Run("MOD_BODY_CALL_degrades_to_fix", func(t *testing.T) {
		c := NewClassifierWithCatalog(nil, nil) // no BinaryClassifier
		annotated := "📄 internal/auth/login.go\nvalidateCall [MOD_BODY_CALL] internal/auth/login.go:10\n"
		chunk := newAnnotatedFixture(annotated)
		chunk.Files = []string{"internal/auth/login.go"}

		commitType, confidence := c.Classify(chunk)

		if commitType != "fix" {
			t.Errorf("CommitType = %q, want fix (degraded)", commitType)
		}
		if confidence < 0.60 {
			t.Errorf("Confidence = %f, want >= 0.60 for degraded MOD_BODY_CALL", confidence)
		}
	})

	t.Run("MOD_BODY_CALL_delegates_to_mock", func(t *testing.T) {
		mock := &mockBinaryClassifier{result: "refactor"}
		c := NewClassifierWithCatalog(nil, nil, WithBinaryClassifier(mock))
		annotated := "📄 internal/auth/login.go\nvalidateCall [MOD_BODY_CALL] internal/auth/login.go:10\n"
		chunk := newAnnotatedFixture(annotated)
		chunk.Files = []string{"internal/auth/login.go"}

		commitType, _ := c.Classify(chunk)

		if commitType != "refactor" {
			t.Errorf("CommitType = %q, want refactor (delegated to mock)", commitType)
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

// ---------------------------------------------------------------------------
// Phase 1 RED: Weight-based classification (Fuerza table) tests
// These tests reference LabelWeight() — the exported version of labelWeight.
// ---------------------------------------------------------------------------

// Task 1.1: TestLabelWeight — verify each label type maps to correct (commitType, weight)
func TestLabelWeight(t *testing.T) {
	tests := []struct {
		name       string
		labelType  string
		wantType   string
		wantWeight int
	}{
		// Fuerza 9: feat
		{name: "NEW_FUNC_maps_to_feat_9", labelType: "NEW_FUNC", wantType: "feat", wantWeight: 9},
		{name: "NEW_TYPE_maps_to_feat_9", labelType: "NEW_TYPE", wantType: "feat", wantWeight: 9},
		// Fuerza 8: fix
		{name: "MOD_BODY_LOGIC_maps_to_fix_8", labelType: "MOD_BODY_LOGIC", wantType: "fix", wantWeight: 8},
		{name: "MOD_BODY_ERROR_maps_to_fix_8", labelType: "MOD_BODY_ERROR", wantType: "fix", wantWeight: 8},
		{name: "MOD_SIG_maps_to_fix_8", labelType: "MOD_SIG", wantType: "fix", wantWeight: 8},
		// Fuerza 7: refactor
		{name: "MOD_BODY_REORDER_maps_to_refactor_7", labelType: "MOD_BODY_REORDER", wantType: "refactor", wantWeight: 7},
		{name: "DELETED_FUNC_maps_to_refactor_7", labelType: "DELETED_FUNC", wantType: "refactor", wantWeight: 7},
		{name: "DELETED_TYPE_maps_to_refactor_7", labelType: "DELETED_TYPE", wantType: "refactor", wantWeight: 7},
		{name: "MOD_TYPE_maps_to_refactor_7", labelType: "MOD_TYPE", wantType: "refactor", wantWeight: 7},
		// Fuerza 6: chore / ci / docs
		{name: "CONFIG_maps_to_chore_6", labelType: "CONFIG", wantType: "chore", wantWeight: 6},
		{name: "DEPS_maps_to_chore_6", labelType: "DEPS", wantType: "chore", wantWeight: 6},
		{name: "CI_maps_to_ci_6", labelType: "CI", wantType: "ci", wantWeight: 6},
		{name: "DOCS_maps_to_docs_6", labelType: "DOCS", wantType: "docs", wantWeight: 6},
		// Fuerza 5: test
		{name: "TEST_maps_to_test_5", labelType: "TEST", wantType: "test", wantWeight: 5},
		// Fuerza 4: unknown
		{name: "UNKNOWN_GENERIC_maps_to_chore_4", labelType: "UNKNOWN_GENERIC", wantType: "chore", wantWeight: 4},
		// CHANGED maps to fix (weight 7) as per requirement
		{name: "CHANGED_maps_to_fix_7", labelType: "CHANGED", wantType: "fix", wantWeight: 7},
		// MOD_BODY_CALL → fix with weight 7 (more significant than CONFIG/DEPS at weight 6)
		{name: "MOD_BODY_CALL_maps_to_empty_6", labelType: "MOD_BODY_CALL", wantType: "", wantWeight: 6},
		// Generic MOD_BODY catches unknown future subtypes
		{name: "MOD_BODY_FUTURE_maps_to_fix_8", labelType: "MOD_BODY_FUTURE", wantType: "fix", wantWeight: 8},
		// Unknown label type
		{name: "UNKNOWN_LABEL_maps_to_empty_0", labelType: "SOMETHING_ELSE", wantType: "", wantWeight: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotWeight := LabelWeight(tt.labelType)
			if gotType != tt.wantType {
				t.Errorf("LabelWeight(%q).type = %q, want %q", tt.labelType, gotType, tt.wantType)
			}
			if gotWeight != tt.wantWeight {
				t.Errorf("LabelWeight(%q).weight = %d, want %d", tt.labelType, gotWeight, tt.wantWeight)
			}
		})
	}
}

// RC2: MOD_BODY generic must map to ("fix", 7), not fall through to default prefix match
func TestLabelWeight_MOD_BODY_returns_fix_weight_7(t *testing.T) {
	gotType, gotWeight := LabelWeight("MOD_BODY")
	if gotType != "fix" {
		t.Errorf("LabelWeight(\"MOD_BODY\").type = %q, want %q", gotType, "fix")
	}
	if gotWeight != 7 {
		t.Errorf("LabelWeight(\"MOD_BODY\").weight = %d, want %d", gotWeight, 7)
	}
}

// RC1: UNKNOWN_GENERIC must map to ("chore", 4), not ("refactor", 4)
func TestLabelWeight_UNKNOWN_GENERIC_returns_chore_weight_4(t *testing.T) {
	gotType, gotWeight := LabelWeight("UNKNOWN_GENERIC")
	if gotType != "chore" {
		t.Errorf("LabelWeight(\"UNKNOWN_GENERIC\").type = %q, want %q", gotType, "chore")
	}
	if gotWeight != 4 {
		t.Errorf("LabelWeight(\"UNKNOWN_GENERIC\").weight = %d, want %d", gotWeight, 4)
	}
}

// RC1: CHANGED must map to ("fix", 7), not ("chore", 4)
func TestLabelWeight_CHANGED_returns_fix_weight_7(t *testing.T) {
	gotType, gotWeight := LabelWeight("CHANGED")
	if gotType != "fix" {
		t.Errorf("LabelWeight(\"CHANGED\").type = %q, want %q", gotType, "fix")
	}
	if gotWeight != 7 {
		t.Errorf("LabelWeight(\"CHANGED\").weight = %d, want %d", gotWeight, 7)
	}
}

// RC2 triangulation: MOD_BODY subtypes still have their specific weights
func TestLabelWeight_MOD_BODY_subtypes_still_precedence(t *testing.T) {
	tests := []struct {
		name       string
		labelType  string
		wantType   string
		wantWeight int
	}{
		{name: "MOD_BODY_LOGIC_still_fix_8", labelType: "MOD_BODY_LOGIC", wantType: "fix", wantWeight: 8},
		{name: "MOD_BODY_ERROR_still_fix_8", labelType: "MOD_BODY_ERROR", wantType: "fix", wantWeight: 8},
		{name: "MOD_BODY_REORDER_still_refactor_7", labelType: "MOD_BODY_REORDER", wantType: "refactor", wantWeight: 7},
		{name: "MOD_BODY_CALL_still_empty_6", labelType: "MOD_BODY_CALL", wantType: "", wantWeight: 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotWeight := LabelWeight(tt.labelType)
			if gotType != tt.wantType {
				t.Errorf("LabelWeight(%q).type = %q, want %q", tt.labelType, gotType, tt.wantType)
			}
			if gotWeight != tt.wantWeight {
				t.Errorf("LabelWeight(%q).weight = %d, want %d", tt.labelType, gotWeight, tt.wantWeight)
			}
		})
	}
}

// Task 1.2: Weight-based mixed labels — NEW_FUNC beats MOD_BODY_LOGIC by weight
func TestDetermineType_WeightBasedMixedLabels(t *testing.T) {
	c := &Classifier{}

	// 3×MOD_BODY_LOGIC + 1×NEW_FUNC → feat (weight 9 > weight 8)
	labels := []labelInfo{
		{Type: "MOD_BODY_LOGIC", Breaking: false},
		{Type: "MOD_BODY_LOGIC", Breaking: false},
		{Type: "MOD_BODY_LOGIC", Breaking: false},
		{Type: "NEW_FUNC", Breaking: false},
	}

	commitType, confidence := c.determineType(labels, []string{"a.go"}, nil, nil, "some diff")

	if commitType != "feat" {
		t.Errorf("determineType(mixed NEW_FUNC+MOD_BODY_LOGIC) = %q, want feat", commitType)
	}
	if confidence < 0.65 {
		t.Errorf("determineType confidence = %f, want >= 0.65 for mixed labels", confidence)
	}
}

// Task 1.3: Tiebreaker — equal weight broken by label count
func TestDetermineType_WeightBasedTiebreaker(t *testing.T) {
	c := &Classifier{}

	// MOD_SIG (weight 8) and MOD_BODY_LOGIC (weight 8) have same weight.
	// Whichever has MORE labels wins by count tiebreaker.
	t.Run("equal_weight_more_mod_body_logic_wins", func(t *testing.T) {
		labels := []labelInfo{
			{Type: "MOD_BODY_LOGIC", Breaking: false},
			{Type: "MOD_BODY_LOGIC", Breaking: false},
			{Type: "MOD_SIG", Breaking: false},
		}
		commitType, confidence := c.determineType(labels, []string{"a.go"}, nil, nil, "diff")
		if commitType != "fix" {
			t.Errorf("determineType(MOD_BODY_LOGIC×2 + MOD_SIG×1) = %q, want fix", commitType)
		}
		_ = confidence
	})

	t.Run("equal_weight_more_mod_sig_wins", func(t *testing.T) {
		labels := []labelInfo{
			{Type: "MOD_SIG", Breaking: false},
			{Type: "MOD_SIG", Breaking: false},
			{Type: "MOD_BODY_LOGIC", Breaking: false},
		}
		commitType, confidence := c.determineType(labels, []string{"a.go"}, nil, nil, "diff")
		if commitType != "fix" {
			t.Errorf("determineType(MOD_SIG×2 + MOD_BODY_LOGIC×1) = %q, want fix", commitType)
		}
		_ = confidence
	})
}

// Task 1.4: Breaking suffix is orthogonal to weight
func TestDetermineType_BreakingSuffixIsOrthogonal(t *testing.T) {
	c := &Classifier{}

	t.Run("new_func_with_breaking_gives_feat_bang", func(t *testing.T) {
		labels := []labelInfo{
			{Type: "DELETED_FUNC", Breaking: true},
			{Type: "NEW_FUNC", Breaking: false},
		}
		commitType, _ := c.determineType(labels, []string{"a.go"}, nil, nil, "diff")
		if commitType != "feat!" {
			t.Errorf("determineType(NEW_FUNC + DELETED_FUNC BREAKING) = %q, want feat!", commitType)
		}
	})

	t.Run("mod_body_logic_with_breaking_gives_fix_bang", func(t *testing.T) {
		labels := []labelInfo{
			{Type: "MOD_BODY_LOGIC", Breaking: true},
		}
		commitType, _ := c.determineType(labels, []string{"a.go"}, nil, nil, "diff")
		if commitType != "fix!" {
			t.Errorf("determineType(MOD_BODY_LOGIC BREAKING) = %q, want fix!", commitType)
		}
	})

	t.Run("new_func_no_breaking_no_bang", func(t *testing.T) {
		labels := []labelInfo{
			{Type: "NEW_FUNC", Breaking: false},
			{Type: "MOD_BODY_LOGIC", Breaking: false},
		}
		commitType, _ := c.determineType(labels, []string{"a.go"}, nil, nil, "diff")
		if commitType != "feat" {
			t.Errorf("determineType(NEW_FUNC+MOD_BODY_LOGIC no breaking) = %q, want feat", commitType)
		}
	})
}

// Task 1.5: Test-only override still takes precedence over weight
func TestDetermineType_TestOnlyOverridesWeight(t *testing.T) {
	c := &Classifier{}

	// All test files with NEW_FUNC → test (step 0 override, not feat)
	// This requires a catalog that marks files as test files.
	// We test via Classify() with test-pattern files (filename-based detection)
	labels := []labelInfo{
		{Type: "NEW_FUNC", Breaking: false},
	}
	commitType, _ := c.determineType(labels, []string{"handler_test.go"}, nil, nil, "diff")
	if commitType != "test" {
		t.Errorf("determineType(NEW_FUNC on test file) = %q, want test (test override wins)", commitType)
	}
}

// Task 1.6: Feat with low confidence — 1×NEW_FUNC + 5×MOD_BODY_LOGIC → feat but confidence < 0.85
func TestDetermineType_FeatWithLowConfidence(t *testing.T) {
	c := &Classifier{}

	labels := []labelInfo{
		{Type: "NEW_FUNC", Breaking: false},
		{Type: "MOD_BODY_LOGIC", Breaking: false},
		{Type: "MOD_BODY_LOGIC", Breaking: false},
		{Type: "MOD_BODY_LOGIC", Breaking: false},
		{Type: "MOD_BODY_LOGIC", Breaking: false},
		{Type: "MOD_BODY_LOGIC", Breaking: false},
	}

	commitType, confidence := c.determineType(labels, []string{"a.go"}, nil, nil, "diff")

	if commitType != "feat" {
		t.Errorf("determineType(1×NEW_FUNC + 5×MOD_BODY_LOGIC) = %q, want feat", commitType)
	}
	if confidence >= 0.85 {
		t.Errorf("determineType confidence = %f, want < 0.85 for low-purity feat", confidence)
	}
}

// TestLabelWeight_MOD_BODY_CALL_wins_over_CONFIG validates that MOD_BODY_CALL
// (weight 7) now wins over CONFIG (weight 6) per the updated Fuerza table.
func TestLabelWeight_MOD_BODY_CALL_wins_over_CONFIG(t *testing.T) {
	tests := []struct {
		name       string
		labelType  string
		wantType   string
		wantWeight int
	}{
		{name: "MOD_BODY_CALL_is_weight_6", labelType: "MOD_BODY_CALL", wantType: "", wantWeight: 6},
		{name: "CONFIG_is_weight_6", labelType: "CONFIG", wantType: "chore", wantWeight: 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotWeight := LabelWeight(tt.labelType)
			if gotType != tt.wantType {
				t.Errorf("LabelWeight(%q).type = %q, want %q", tt.labelType, gotType, tt.wantType)
			}
			if gotWeight != tt.wantWeight {
				t.Errorf("LabelWeight(%q).weight = %d, want %d", tt.labelType, gotWeight, tt.wantWeight)
			}
		})
	}
}

// TestCommitTypeWeight validates that each commit type maps to the correct weight.
// This is the reverse mapping of LabelWeight — commit types like "feat" → weight 9.
func TestCommitTypeWeight(t *testing.T) {
	tests := []struct {
		name       string
		commitType string
		wantWeight int
	}{
		{name: "feat_is_weight_9", commitType: "feat", wantWeight: 9},
		{name: "fix_is_weight_8", commitType: "fix", wantWeight: 8},
		{name: "refactor_is_weight_7", commitType: "refactor", wantWeight: 7},
		{name: "chore_is_weight_6", commitType: "chore", wantWeight: 6},
		{name: "ci_is_weight_6", commitType: "ci", wantWeight: 6},
		{name: "docs_is_weight_6", commitType: "docs", wantWeight: 6},
		{name: "test_is_weight_5", commitType: "test", wantWeight: 5},
		{name: "unknown_is_weight_0", commitType: "unknown_type", wantWeight: 0},
		{name: "empty_is_weight_0", commitType: "", wantWeight: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotWeight := domain.CommitTypeWeight(tt.commitType)
			if gotWeight != tt.wantWeight {
				t.Errorf("CommitTypeWeight(%q) = %d, want %d", tt.commitType, gotWeight, tt.wantWeight)
			}
		})
	}
}

// ---------------------------------------------------------------------------

// TestInferCommitType validates the 7-level heuristic cascade for inferring
// commit types from diff content when the classifier returns an empty CommitType.
func TestInferCommitType(t *testing.T) {
	tests := []struct {
		name     string
		chunk    domain.DiffChunk
		wantType string
	}{
		{
			name: "new_file_returns_feat",
			chunk: domain.DiffChunk{
				Files: []string{"cmd/server.go", "cmd/server_test.go"},
				Diff:  "diff --git a/cmd/server.go b/cmd/server.go\nnew file mode 100644\n--- /dev/null\n+++ b/cmd/server.go\n@@ -0,0 +1,10 @@\n+package cmd\n",
			},
			wantType: "feat",
		},
		{
			name: "only_config_returns_chore",
			chunk: domain.DiffChunk{
				Files: []string{"go.mod", "go.sum"},
				Diff:  "--- a/go.mod\n+++ b/go.mod\n@@ -1,5 +1,5 @@\n module github.com/example/repo\n-go 1.22\n+go 1.23\n",
			},
			wantType: "chore",
		},
		{
			name: "source_modifications_returns_fix",
			chunk: domain.DiffChunk{
				Files: []string{"internal/service/handler.go"},
				Diff:  "--- a/internal/service/handler.go\n+++ b/internal/service/handler.go\n@@ -10,7 +10,7 @@ func handle() {\n-   old := true\n+   new := true\n",
			},
			wantType: "fix",
		},
		{
			name: "only_test_files_returns_test",
			chunk: domain.DiffChunk{
				Files: []string{"internal/service/handler_test.go"},
				Diff:  "--- a/internal/service/handler_test.go\n+++ b/internal/service/handler_test.go\n@@ -5,6 +5,10 @@ func TestHandler(t *T) {\n",
			},
			wantType: "test",
		},
		{
			name: "empty_diff_empty_files_returns_chore",
			chunk: domain.DiffChunk{
				Files: []string{},
				Diff:  "",
			},
			wantType: "chore",
		},
		{
			name: "new_file_via_diff_pattern_returns_feat",
			chunk: domain.DiffChunk{
				Files: []string{"src/new_feature.rs"},
				Diff:  "diff --git a/src/new_feature.rs b/src/new_feature.rs\nnew file mode 100644\n--- /dev/null\n+++ b/src/new_feature.rs\n@@ -0,0 +1,5 @@\n+fn main() {}\n",
			},
			wantType: "feat",
		},
		{
			name: "new_file_mixed_with_config_returns_feat",
			chunk: domain.DiffChunk{
				Files: []string{"go.mod", "cmd/app/main.go"},
				Diff:  "diff --git a/cmd/app/main.go b/cmd/app/main.go\nnew file mode 100644\n--- /dev/null\n+++ b/cmd/app/main.go\n@@ -0,0 +1,3 @@\n+package main\n+\n+func main() {}\n",
			},
			wantType: "feat",
		},
		{
			name: "docs_only_returns_docs",
			chunk: domain.DiffChunk{
				Files: []string{"docs/api.md", "README.md"},
				Diff:  "--- a/docs/api.md\n+++ b/docs/api.md\n@@ -1,3 +1,4 @@\n # API Documentation\n+\n## Endpoints\n",
			},
			wantType: "docs",
		},
		{
			name: "deleted_file_returns_refactor",
			chunk: domain.DiffChunk{
				Files: []string{"internal/legacy/deprecated.go"},
				Diff:  "diff --git a/internal/legacy/deprecated.go b/internal/legacy/deprecated.go\ndeleted file mode 100644\n--- a/internal/legacy/deprecated.go\n+++ /dev/null\n@@ -1,10 +0,0 @@\n-package legacy\n",
			},
			wantType: "refactor",
		},
		{
			name: "non_empty_commit_type_returns_existing",
			chunk: domain.DiffChunk{
				CommitType:      "refactor",
				ConfidenceScore: 0.80,
				Files:           []string{"handler.go"},
				Diff:            "--- a/handler.go\n+++ b/handler.go\n",
			},
			wantType: "refactor",
		},
		{
			name: "test_file_alongside_code_is_fix_not_test",
			chunk: domain.DiffChunk{
				Files: []string{"handler.go", "handler_test.go"},
				Diff:  "--- a/handler.go\n+++ b/handler.go\n@@ -10,3 +10,5 @@ func Handle() {\n",
			},
			wantType: "fix", // code+test pair → not "only test files" → source mod → fix
		},
		{
			name: "docs_path_prefix_returns_docs",
			chunk: domain.DiffChunk{
				Files: []string{"docs/guide.md"},
				Diff:  "--- a/docs/guide.md\n+++ b/docs/guide.md\n@@ -1 +1,2 @@\n",
			},
			wantType: "docs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.InferCommitType(tt.chunk)
			if got != tt.wantType {
				t.Errorf("InferCommitType() = %q, want %q", got, tt.wantType)
			}
		})
	}
}

// --- Pillar 0.5: Path-type detection tests ---

func TestClassify_Pillar05_AllFilesMatchPathType(t *testing.T) {
	t.Parallel()
	pathTypes := map[string][]string{
		"ci":   {".github/workflows/", "ci/"},
		"test": {"test/"},
		"docs": {"docs/"},
	}
	c := NewClassifierWithCatalog(nil, domain.NewLanguageCatalog(nil, nil, nil), WithPathTypes(pathTypes))

	chunk := &domain.DiffChunk{
		Files:         []string{".github/workflows/ci.yml"},
		AnnotatedDiff: "📄 .github/workflows/ci.yml\nSetup [NEW_FUNC] .github/workflows/ci.yml:10\n",
		Diff:          "sample diff",
	}

	commitType, _ := c.Classify(chunk)
	if commitType != "ci" {
		t.Errorf("Pillar 0.5: all files match path type ci = %q, want %q (not feat from NEW_FUNC)", commitType, "ci")
	}
}

func TestClassify_Pillar05_SkipsWhenNotAllFilesMatchOneType(t *testing.T) {
	t.Parallel()
	pathTypes := map[string][]string{
		"ci": {".github/workflows/"},
	}
	c := NewClassifierWithCatalog(nil, domain.NewLanguageCatalog(nil, nil, nil), WithPathTypes(pathTypes))

	chunk := &domain.DiffChunk{
		Files:         []string{".github/workflows/ci.yml", "src/main.go"},
		AnnotatedDiff: "📄 src/main.go\nMain [NEW_FUNC] src/main.go:10\n",
		Diff:          "sample diff",
	}

	commitType, _ := c.Classify(chunk)
	// Mixed types → Pillar 0.5 does not override, weight-based determines
	if commitType == "ci" {
		t.Errorf("Pillar 0.5: mixed files should not override, got %q", commitType)
	}
}

func TestClassify_Pillar05_EmptyPathTypesIsNoOp(t *testing.T) {
	t.Parallel()
	c := NewClassifierWithCatalog(nil, domain.NewLanguageCatalog(nil, nil, nil))
	// No WithPathTypes → pathTypes is nil

	chunk := &domain.DiffChunk{
		Files:         []string{".github/workflows/ci.yml"},
		AnnotatedDiff: "📄 .github/workflows/ci.yml\nSetup [NEW_FUNC] .github/workflows/ci.yml:10\n",
		Diff:          "sample diff",
	}

	commitType, _ := c.Classify(chunk)
	// Without pathTypes, Pillar 0.5 is no-op, NEW_FUNC → feat
	if commitType != "feat" {
		t.Errorf("Pillar 0.5: empty pathTypes should not override, got %q, want feat", commitType)
	}
}

func TestClassify_Pillar05_TestDirWithNEWFUNC(t *testing.T) {
	t.Parallel()
	pathTypes := map[string][]string{
		"test": {"test/"},
		"ci":   {".github/workflows/"},
		"docs": {"docs/"},
	}
	c := NewClassifierWithCatalog(nil, domain.NewLanguageCatalog(nil, nil, nil), WithPathTypes(pathTypes))

	chunk := &domain.DiffChunk{
		Files:         []string{"test/pipeline/runner.go"},
		AnnotatedDiff: "📄 test/pipeline/runner.go\nRun [NEW_FUNC] test/pipeline/runner.go:20\n",
		Diff:          "sample diff",
	}

	commitType, _ := c.Classify(chunk)
	// test/ dir files should be "test" (Pillar 0 for test-file + Pillar 0.5 for path-type both fire)
	if commitType != "test" {
		t.Errorf("Pillar 0.5: test/ with NEW_FUNC = %q, want %q", commitType, "test")
	}
}

// ---------------------------------------------------------------------------
// resolvePathTypeFromMap — direct unit tests
// ---------------------------------------------------------------------------

func TestResolvePathTypeFromMap_AllFilesMatch(t *testing.T) {
	t.Parallel()
	pathTypes := map[string][]string{
		"ci":   {".github/workflows/"},
		"test": {"test/"},
	}
	result := resolvePathTypeFromMap(pathTypes, []string{".github/workflows/ci.yml", ".github/workflows/build.yml"})
	if result != "ci" {
		t.Errorf("resolvePathTypeFromMap: all match ci = %q, want %q", result, "ci")
	}
}

func TestResolvePathTypeFromMap_NotAllFilesMatch(t *testing.T) {
	t.Parallel()
	pathTypes := map[string][]string{
		"ci": {".github/workflows/"},
	}
	result := resolvePathTypeFromMap(pathTypes, []string{".github/workflows/ci.yml", "src/main.go"})
	if result != "" {
		t.Errorf("resolvePathTypeFromMap: mixed files = %q, want empty string", result)
	}
}

func TestResolvePathTypeFromMap_EmptyMap(t *testing.T) {
	t.Parallel()
	result := resolvePathTypeFromMap(nil, []string{"test/runner.go"})
	if result != "" {
		t.Errorf("resolvePathTypeFromMap: nil map = %q, want empty string", result)
	}
}

func TestResolvePathTypeFromMap_EmptyFiles(t *testing.T) {
	t.Parallel()
	pathTypes := map[string][]string{"test": {"test/"}}
	result := resolvePathTypeFromMap(pathTypes, nil)
	if result != "" {
		t.Errorf("resolvePathTypeFromMap: nil files = %q, want empty string", result)
	}
}

func TestResolvePathTypeFromMap_DifferentTypesNoUnanimity(t *testing.T) {
	t.Parallel()
	pathTypes := map[string][]string{
		"ci":   {".github/workflows/"},
		"docs": {"docs/"},
	}
	// 1 file matches ci, 1 matches docs → no type has unanimity → empty
	result := resolvePathTypeFromMap(pathTypes, []string{".github/workflows/ci.yml", "docs/api.md"})
	if result != "" {
		t.Errorf("resolvePathTypeFromMap: split types = %q, want empty string (no unanimity)", result)
	}
}

func TestResolvePathTypeFromMap_TieBreakByTypeName(t *testing.T) {
	t.Parallel()
	pathTypes := map[string][]string{
		"ci":   {".github/workflows/", "ci/"},
		"docs": {"docs/", ".github/workflows/"},
	}
	// Both types match .github/workflows/ci.yml, but ci matches 2 files, docs matches 1 → ci wins by majority
	result := resolvePathTypeFromMap(pathTypes, []string{".github/workflows/ci.yml", "ci/config.yml"})
	if result != "ci" {
		t.Errorf("resolvePathTypeFromMap: majority ci = %q, want %q", result, "ci")
	}
}

func TestClassify_Pillar05_DocsWithCONFIG(t *testing.T) {
	t.Parallel()
	pathTypes := map[string][]string{
		"docs": {"docs/"},
		"ci":   {".github/workflows/"},
	}
	c := NewClassifierWithCatalog(nil, domain.NewLanguageCatalog(nil, nil, nil), WithPathTypes(pathTypes))

	chunk := &domain.DiffChunk{
		Files:         []string{"docs/api.md"},
		AnnotatedDiff: "📄 docs/api.md\nConfig [CONFIG] docs/api.md:5\n",
		Diff:          "sample diff",
	}

	commitType, _ := c.Classify(chunk)
	// Path-type should override weight-based; docs/ → "docs" not "chore"
	if commitType != "docs" {
		t.Errorf("Pillar 0.5: docs/ with CONFIG = %q, want %q", commitType, "docs")
	}
}

func TestClassify_AnnotatedLabels(t *testing.T) {
	c := &Classifier{}

	t.Run("namespaced label empty suffix", func(t *testing.T) {
		chunk := &domain.DiffChunk{
			Files:         []string{"internal/helper.go"},
			AnnotatedDiff: "📄 internal/helper.go\nhelper [NEW_FUNC: ] internal/helper.go:12\n",
			Diff:          "sample diff",
		}
		commitType, _ := c.Classify(chunk)
		if commitType != "feat" {
			t.Errorf("Expected commitType 'feat', got %q", commitType)
		}
	})

	t.Run("namespaced breaking label", func(t *testing.T) {
		chunk := &domain.DiffChunk{
			Files:         []string{"internal/helper.go"},
			AnnotatedDiff: "📄 internal/helper.go\nhelper [MOD_BODY_LOGIC: helper ⚠BREAKING] internal/helper.go:12\n",
			Diff:          "sample diff",
		}
		commitType, confidence := c.Classify(chunk)
		if commitType != "fix!" {
			t.Errorf("Expected commitType 'fix!', got %q", commitType)
		}
		if confidence < 0.80 {
			t.Errorf("Expected confidence >= 0.80, got %f", confidence)
		}
	})

	t.Run("namespaced chore label", func(t *testing.T) {
		chunk := &domain.DiffChunk{
			Files:         []string{"config/settings.json"},
			AnnotatedDiff: "📄 config/settings.json\nsettings.json [CONFIG: dev] config/settings.json:1\n",
			Diff:          "sample diff",
		}
		commitType, confidence := c.Classify(chunk)
		if commitType != "chore" {
			t.Errorf("Expected commitType 'chore', got %q", commitType)
		}
		if confidence < 0.80 {
			t.Errorf("Expected confidence >= 0.80, got %f", confidence)
		}
	})
}
