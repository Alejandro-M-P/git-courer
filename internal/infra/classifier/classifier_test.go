package classifier

import (
	"fmt"
	"strings"
	"testing"

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
				if confidence < 0.95 {
					t.Errorf("Confidence = %f for %s, want >= 0.95", confidence, tt.label)
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

// Task 1.6: TestClassify_mixed_patterns_fallback
func TestClassify_mixed_patterns_fallback(t *testing.T) {
	c := &Classifier{}

	// Mixed labels: feat + docs → should NOT classify confidently
	annotated := "📄 internal/server/webhook.go\nHandleWebhook [NEW_FUNC] internal/server/webhook.go:42\n" +
		"📄 README.md\nREADME.md [DOCS] README.md\n"
	chunk := newAnnotatedFixture(annotated)

	commitType, confidence := c.Classify(chunk)

	if commitType != "" {
		t.Errorf("CommitType = %q, want empty (mixed patterns → fallback to LLM)", commitType)
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
