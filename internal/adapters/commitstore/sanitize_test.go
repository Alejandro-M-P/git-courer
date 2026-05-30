package commitstore

import "testing"

func TestSanitizeBranchName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "slash replaced with dash",
			input:    "feat/auth",
			expected: "feat-auth",
		},
		{
			name:     "slash and hyphen preserved",
			input:    "fix/bug-42",
			expected: "fix-bug-42",
		},
		{
			name:     "version slash replaced",
			input:    "release/v2.0",
			expected: "release-v2.0",
		},
		{
			name:     "HEAD unchanged",
			input:    "HEAD",
			expected: "HEAD",
		},
		{
			name:     "tilde removed with leading dash trimmed",
			input:    "~/.config",
			expected: ".config",
		},
		{
			name:     "all special chars removed results in HEAD",
			input:    "^^",
			expected: "HEAD",
		},
		{
			name:     "double slash collapsed",
			input:    "feat//auth",
			expected: "feat-auth",
		},
		{
			name:     "colon removed",
			input:    "fix:login",
			expected: "fixlogin",
		},
		{
			name:     "backslash removed",
			input:    `feat\auth`,
			expected: "featauth",
		},
		{
			name:     "space removed",
			input:    "feat auth",
			expected: "featauth",
		},
		{
			name:     "caret removed",
			input:    "feat^auth",
			expected: "featauth",
		},
		{
			name:     "trailing dash trimmed",
			input:    "feat/auth-",
			expected: "feat-auth",
		},
		{
			name:     "leading dash trimmed",
			input:    "-feat/auth",
			expected: "feat-auth",
		},
		{
			name:     "consecutive dashes collapsed",
			input:    "feat---auth",
			expected: "feat-auth",
		},
		{
			name:     "simple branch name unchanged",
			input:    "main",
			expected: "main",
		},
		{
			name:     "nested slashes converted",
			input:    "feature/auth/login",
			expected: "feature-auth-login",
		},
		{
			name:     "empty string returns HEAD",
			input:    "",
			expected: "HEAD",
		},
		{
			name:     "only dashes after sanitization returns HEAD",
			input:    "---",
			expected: "HEAD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sanitizeBranchName(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeBranchName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}