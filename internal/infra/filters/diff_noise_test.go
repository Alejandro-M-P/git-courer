package filters

import (
	"strings"
	"testing"
)

func TestFilterDiffNoise(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "EmptyDiff",
			input:    "",
			expected: "",
		},
		{
			name:     "OnlyNoise",
			input:    "diff --git a b\nindex 1..2\n@@ -1 +1\n\\",
			expected: "@@ -1 +1",
		},
		{
			name:     "NoNoise",
			input:    "--- a\n+++ b\n+foo\n-bar\n",
			expected: "--- a\n+++ b\n+foo\n-bar\n",
		},
		{
			name:     "MixedNoiseAndContent",
			input:    "diff --git a b\nindex 1..2\n--- a\n+++ b\n@@ -1,1 +1,1 @@\n-old\n+new\n\\ No newline",
			expected: "--- a\n+++ b\n@@ -1,1 +1,1 @@\n-old\n+new",
		},
		{
			name: "MultiFileDiff",
			input: "diff --git a/f1 b/f1\nindex 1..2\n--- a/f1\n+++ b/f1\n@@ -1 +1 @@\n-1a\n+1b\n" +
				"diff --git a/f2 b/f2\nindex 3..4\n--- a/f2\n+++ b/f2\n@@ -1 +1 @@\n-2a\n+2b\n",
			expected: "--- a/f1\n+++ b/f1\n@@ -1 +1 @@\n-1a\n+1b\n--- a/f2\n+++ b/f2\n@@ -1 +1 @@\n-2a\n+2b\n",
		},
		{
			name:     "BackslashNoNewline",
			input:    "--- a\n+++ b\n+line\n\\ No newline",
			expected: "--- a\n+++ b\n+line",
		},
		{
			name:     "KeepsContextLines",
			input:    "--- a\n+++ b\n context\n+add\n-del\n",
			expected: "--- a\n+++ b\n context\n+add\n-del\n",
		},
		{
			name:     "KeepsArbitraryLines",
			input:    "--- a\n+++ b\n arbitrary text\n",
			expected: "--- a\n+++ b\n arbitrary text\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterDiffNoise(tt.input)
			if got != tt.expected {
				t.Errorf("FilterDiffNoise(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFilterDiffNoise_Purity(t *testing.T) {
	input := "--- a\n+++ b\n+foo\n"
	original := input
	_ = FilterDiffNoise(input)
	if input != original {
		t.Error("FilterDiffNoise mutated the input string")
	}
}

func TestFilterDiffNoise_NoBlacklistedPrefixes(t *testing.T) {
	input := "diff --git a b\nindex 1..2\n@@ -1 +1 \n\\ No newline\n--- a\n+++ b\n+foo\n-bar\n"
	got := FilterDiffNoise(input)
	lines := strings.Split(got, "\n")
	hasAtAt := false
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") ||
			strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "\\") {
			t.Errorf("output contains blacklisted prefix: %q", line)
		}
		if strings.HasPrefix(line, "@@") {
			hasAtAt = true
		}
	}
	if !hasAtAt {
		t.Error("expected @@ hunk headers to be preserved, but none found")
	}
}

func TestFilterDiffNoise_PreservesHunkHeaders(t *testing.T) {
	// Triangulation: complex multi-hunk diff — all @@ lines must survive.
	input := "diff --git a/src/main.go b/src/main.go\n" +
		"index 123..456 100644\n" +
		"--- a/src/main.go\n" +
		"+++ b/src/main.go\n" +
		"@@ -1,5 +1,6 @@\n" +
		" package main\n" +
		" \n" +
		"@@ -42,7 +43,7 @@ func main() {\n" +
		"-old\n" +
		"+new\n" +
		"@@ -100,3 +101,4 @@ func helper() {\n" +
		" context\n" +
		"+added\n"
	got := FilterDiffNoise(input)
	if !strings.Contains(got, "@@ -1,5 +1,6 @@") {
		t.Error("missing first hunk header")
	}
	if !strings.Contains(got, "@@ -42,7 +43,7 @@") {
		t.Error("missing second hunk header")
	}
	if !strings.Contains(got, "@@ -100,3 +101,4 @@") {
		t.Error("missing third hunk header")
	}
	// Verify no stale blacklist prefixes
	if strings.Contains(got, "diff --git") {
		t.Error("diff --git should be stripped")
	}
	if strings.Contains(got, "index ") {
		t.Error("index should be stripped")
	}
}
