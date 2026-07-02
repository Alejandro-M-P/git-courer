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

func TestSanitizeBranchName_Exported(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple branch", "main", "main"},
		{"feature slash", "feature/foo", "feature-foo"},
		{"nested slashes", "feature/auth/login", "feature-auth-login"},
		{"empty returns HEAD", "", "HEAD"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := SanitizeBranchName(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeBranchName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsUUID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid lowercase UUID", "a1b2c3d4-e5f6-4789-8a01-234567890abc", true},
		{"valid uppercase UUID", "A1B2C3D4-E5F6-4789-8A01-234567890ABC", true},
		{"valid mixed case UUID", "a1B2c3D4-e5F6-4789-8a01-234567890AbC", true},
		{"branch name is not UUID", "feature/foo", false},
		{"simple branch is not UUID", "main", false},
		{"empty string is not UUID", "", false},
		{"uuid missing dashes is not UUID", "a1b2c3d4e5f647898a01234567890abc", false},
		{"uuid wrong segment lengths is not UUID", "a1b2c3d4-e5f6-478-8a01-234567890abc", false},
		{"uuid with invalid hex is not UUID", "g1b2c3d4-e5f6-4789-8a01-234567890abc", false},
		{"uuid too many segments is not UUID", "a1b2c3d4-e5f6-4789-8a01-234567890abc-extra", false},
		{"uuid with braces is not UUID", "{a1b2c3d4-e5f6-4789-8a01-234567890abc}", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isUUID(tt.input)
			if got != tt.want {
				t.Errorf("isUUID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
