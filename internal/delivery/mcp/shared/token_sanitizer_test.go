package shared

import (
	"strings"
	"testing"
)

func TestSanitizeDiffForProvider_Local_StripsNoise(t *testing.T) {
	raw := "diff --git a/file.go b/file.go\nindex 123..456\n--- a/file.go\n+++ b/file.go\n@@ -1,3 +1,4 @@\n-old\n+new\n\\ No newline"
	result := SanitizeDiffForProvider(raw, 0, 100, "ollama")

	if result.Diff == "" {
		t.Fatal("SanitizeDiffForProvider returned empty diff for local mode")
	}
	if strings.Contains(result.Diff, "diff --git") {
		t.Error("local mode should strip 'diff --git' lines")
	}
	if strings.Contains(result.Diff, "index ") {
		t.Error("local mode should strip 'index' lines")
	}
	if strings.Contains(result.Diff, "\\ No newline") {
		t.Error("local mode should strip '\\ No newline' lines")
	}
	if !strings.Contains(result.Diff, "@@ -1,3 +1,4 @@") {
		t.Error("local mode must preserve @@ hunk headers")
	}
	if !strings.Contains(result.Diff, "--- a/file.go") {
		t.Error("local mode must preserve file headers")
	}
}

func TestSanitizeDiffForProvider_Cloud_PreservesContext(t *testing.T) {
	raw := "diff --git a/file.go b/file.go\nindex 123..456\n--- a/file.go\n+++ b/file.go\n@@ -1,3 +1,4 @@\n-old\n+new\n\\ No newline"
	result := SanitizeDiffForProvider(raw, 0, 100, "kimi")

	if result.Diff == "" {
		t.Fatal("SanitizeDiffForProvider returned empty diff for cloud mode")
	}
	// Cloud mode preserves more context — diff --git should survive
	if !strings.Contains(result.Diff, "diff --git") {
		t.Error("cloud mode should preserve 'diff --git' lines")
	}
	if !strings.Contains(result.Diff, "@@ -1,3 +1,4 @@") {
		t.Error("cloud mode should preserve @@ hunk headers")
	}
	if !strings.Contains(result.Diff, "-old") {
		t.Error("cloud mode should preserve diff content lines")
	}
}

func TestSanitizeDiffForProvider_Cloud_FallbackToRaw(t *testing.T) {
	// A diff that would be stripped completely even in cloud mode.
	// Actually cloud mode keeps everything, so fallback is hard to trigger
	// unless we pass empty. Fallback mainly protects local→raw transitions.
	raw := "old-content-line"
	result := SanitizeDiffForProvider(raw, 0, 100, "kimi")
	// Cloud keeps everything, so old-content-line should survive
	if !strings.Contains(result.Diff, "old-content-line") {
		t.Error("cloud mode should keep all content lines")
	}
}

func TestSanitizeDiffForProvider_NeutralEmptyProvider(t *testing.T) {
	raw := "diff --git a/file.go b/file.go\nindex 123..456\n--- a/file.go\n+++ b/file.go\n@@ -1 +1 @@\n-old\n+new"
	result := SanitizeDiffForProvider(raw, 0, 100, "")
	// Empty provider = neutral: NO assumptions, raw passthrough.
	// Nothing is stripped because we don't know what the consumer needs.
	if !strings.Contains(result.Diff, "diff --git") {
		t.Error("neutral mode (empty provider) must preserve 'diff --git' — no assumptions")
	}
	if !strings.Contains(result.Diff, "index ") {
		t.Error("neutral mode must preserve 'index' lines")
	}
	if !strings.Contains(result.Diff, "@@ -1 +1 @@") {
		t.Error("neutral mode must preserve @@ hunk headers")
	}
	if !strings.Contains(result.Diff, "-old") {
		t.Error("neutral mode must preserve content lines")
	}
}

func TestSanitizeDiffForProvider_NeutralKeepsEverything(t *testing.T) {
	// Triangulation: with \\ No newline marker — neutral still keeps all content
	raw := "diff --git a/x b/x\nindex abc..def\n--- a/x\n+++ b/x\n@@ -10,5 +10,6 @@\n context\n-added\n\\ No newline"
	result := SanitizeDiffForProvider(raw, 0, 100, "")
	// Neutral mode preserves everything — only pagination applies
	if !strings.Contains(result.Diff, "diff --git") {
		t.Error("neutral should keep diff --git")
	}
	if !strings.Contains(result.Diff, "\\ No newline") {
		t.Error("neutral should keep \\ No newline — we don't know if caller needs it")
	}
	if !strings.Contains(result.Diff, "added") {
		t.Error("neutral should keep content changes")
	}
	if result.TotalLines != 8 {
		t.Errorf("TotalLines = %d, want 8 (all lines preserved)", result.TotalLines)
	}
	if result.Filtered {
		t.Error("neutral mode should not report Filtered=true")
	}
	if result.NoiseLinesRemoved != 0 {
		t.Errorf("neutral NoiseLinesRemoved = %d, want 0", result.NoiseLinesRemoved)
	}
}

func TestSanitizeDiffForProvider_Pagination(t *testing.T) {
	raw := "--- a\n+++ b\n@@ -1,3 +1,3 @@\nline1\nline2\nline3"
	result := SanitizeDiffForProvider(raw, 1, 2, "ollama")
	if result.TotalLines != 6 {
		t.Errorf("TotalLines = %d, want 6", result.TotalLines)
	}
	if result.LinesShown != 2 {
		t.Errorf("LinesShown = %d, want 2 (offset 1, limit 2 = lines 2-3)", result.LinesShown)
	}
	if !result.Truncated {
		t.Error("Truncated should be true when limit < total lines")
	}
	if !strings.Contains(result.Diff, "+++ b") {
		t.Error("offset 1 should skip --- a, include +++ b")
	}
}

// TestSanitizeDiffForProvider_OpenAICompatibleAsCloud verifies that
// non-ollama providers (like openai-compatible, kimi, deepseek) get cloud treatment.
func TestSanitizeDiffForProvider_OpenAICompatibleAsCloud(t *testing.T) {
	raw := "diff --git a x\nindex 1..2\n--- a\n+++ b\n@@ -1 +1 @@\n-old\n+new"
	result := SanitizeDiffForProvider(raw, 0, 100, "openai-compatible")
	if !strings.Contains(result.Diff, "diff --git") {
		t.Error("openai-compatible is cloud, should preserve diff --git")
	}
	if !strings.Contains(result.Diff, "@@ -1 +1 @@") {
		t.Error("cloud mode should preserve @@ hunk headers")
	}
}

// TestSanitizeLogPipeFormat verifies parsing of pipe-delimited log output.
func TestSanitizeLogPipeFormat(t *testing.T) {
	raw := "abc1234|John Doe|2024-01-15|Initial commit\ndef5678|Jane Smith|2024-01-16|Fix bug\nghi9012|Bob Builder|2024-01-17|Add feature"

	result := SanitizeLog(raw, 0, 10)

	if result.TotalCommits != 3 {
		t.Errorf("TotalCommits = %d, want 3", result.TotalCommits)
	}
	if result.Returned != 3 {
		t.Errorf("Returned = %d, want 3", result.Returned)
	}

	want := []CommitEntry{
		{Hash: "abc1234", Author: "John Doe", Date: "2024-01-15", Message: "Initial commit"},
		{Hash: "def5678", Author: "Jane Smith", Date: "2024-01-16", Message: "Fix bug"},
		{Hash: "ghi9012", Author: "Bob Builder", Date: "2024-01-17", Message: "Add feature"},
	}

	for i, wantEntry := range want {
		if i >= len(result.Commits) {
			t.Fatalf("missing commit at index %d", i)
		}
		got := result.Commits[i]
		if got.Hash != wantEntry.Hash {
			t.Errorf("commit[%d].Hash = %q, want %q", i, got.Hash, wantEntry.Hash)
		}
		if got.Author != wantEntry.Author {
			t.Errorf("commit[%d].Author = %q, want %q", i, got.Author, wantEntry.Author)
		}
		if got.Date != wantEntry.Date {
			t.Errorf("commit[%d].Date = %q, want %q", i, got.Date, wantEntry.Date)
		}
		if got.Message != wantEntry.Message {
			t.Errorf("commit[%d].Message = %q, want %q", i, got.Message, wantEntry.Message)
		}
	}
}

// TestSanitizeLogPartialPipeFormat verifies handling of partial pipe-delimited lines (2-3 parts).
func TestSanitizeLogPartialPipeFormat(t *testing.T) {
	raw := "abc1234|partial message"

	result := SanitizeLog(raw, 0, 10)
	if result.TotalCommits != 1 {
		t.Errorf("TotalCommits = %d, want 1", result.TotalCommits)
	}
	if result.Commits[0].Hash != "abc1234" {
		t.Errorf("Hash = %q, want abc1234", result.Commits[0].Hash)
	}
	if result.Commits[0].Message != "partial message" {
		t.Errorf("Message = %q, want 'partial message'", result.Commits[0].Message)
	}
}

// TestSanitizeLogNoPipeFormat verifies handling of lines without any pipes.
func TestSanitizeLogNoPipeFormat(t *testing.T) {
	raw := "abc1234"

	result := SanitizeLog(raw, 0, 10)
	if result.TotalCommits != 1 {
		t.Errorf("TotalCommits = %d, want 1", result.TotalCommits)
	}
	if result.Commits[0].Hash != "abc1234" {
		t.Errorf("Hash = %q, want abc1234", result.Commits[0].Hash)
	}
	if result.Commits[0].Message != "" {
		t.Errorf("Message = %q, want empty", result.Commits[0].Message)
	}
}

// TestSanitizeLogEmptyInput verifies empty string handling.
func TestSanitizeLogEmptyInput(t *testing.T) {
	result := SanitizeLog("", 0, 10)
	if result.TotalCommits != 0 {
		t.Errorf("TotalCommits = %d, want 0", result.TotalCommits)
	}
	if len(result.Commits) != 0 {
		t.Errorf("len(Commits) = %d, want 0", len(result.Commits))
	}
}

// TestSanitizeLogPagination verifies offset and limit pagination.
func TestSanitizeLogPagination(t *testing.T) {
	raw := "a|A|2024-01-01|msg1\nb|B|2024-01-02|msg2\nc|C|2024-01-03|msg3\nd|D|2024-01-04|msg4"

	result := SanitizeLog(raw, 1, 2)
	if result.TotalCommits != 4 {
		t.Errorf("TotalCommits = %d, want 4", result.TotalCommits)
	}
	if result.Returned != 2 {
		t.Errorf("Returned = %d, want 2", result.Returned)
	}
	if result.Commits[0].Hash != "b" {
		t.Errorf("first returned hash = %q, want b", result.Commits[0].Hash)
	}
	if result.Commits[1].Hash != "c" {
		t.Errorf("second returned hash = %q, want c", result.Commits[1].Hash)
	}
	if !result.Truncated {
		t.Error("Truncated should be true")
	}
	if result.NextOffset != 3 {
		t.Errorf("NextOffset = %d, want 3", result.NextOffset)
	}
}

// TestSanitizeLogPartialParts verifies handling of partial pipe-delimited lines.
func TestSanitizeLogPartialParts(t *testing.T) {
	raw := "abc1234|partial"

	result := SanitizeLog(raw, 0, 10)
	if result.TotalCommits != 1 {
		t.Errorf("TotalCommits = %d, want 1", result.TotalCommits)
	}
	if result.Commits[0].Hash != "abc1234" {
		t.Errorf("Hash = %q, want abc1234", result.Commits[0].Hash)
	}
	if result.Commits[0].Message != "partial" {
		t.Errorf("Message = %q, want 'partial'", result.Commits[0].Message)
	}
}
