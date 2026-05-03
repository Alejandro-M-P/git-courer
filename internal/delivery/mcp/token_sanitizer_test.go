package mcp

import (
	"testing"
)

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
