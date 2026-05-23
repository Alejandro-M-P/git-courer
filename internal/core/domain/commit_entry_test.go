package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestNewCommitEntry_ValidConstruction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		sha     string
		message string
		opts    []CommitEntryOption
		wantSHA string
		wantMsg string
	}{
		{
			name:    "valid 40-char hex SHA with message",
			sha:     "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
			message: "feat: add new feature",
			wantSHA: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
			wantMsg: "feat: add new feature",
		},
		{
			name:    "uppercase hex SHA is valid",
			sha:     "A1B2C3D4E5F6A1B2C3D4E5F6A1B2C3D4E5F6A1B2",
			message: "fix: resolve bug",
			wantSHA: "A1B2C3D4E5F6A1B2C3D4E5F6A1B2C3D4E5F6A1B2",
			wantMsg: "fix: resolve bug",
		},
		{
			name:    "mixed case hex SHA is valid",
			sha:     "Ab12Cd34Ef56Ab12Cd34Ef56Ab12Cd34Ef56Ab12",
			message: "chore: update deps",
			wantSHA: "Ab12Cd34Ef56Ab12Cd34Ef56Ab12Cd34Ef56Ab12",
			wantMsg: "chore: update deps",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			entry, err := NewCommitEntry(tt.sha, tt.message, tt.opts...)
			if err != nil {
				t.Fatalf("NewCommitEntry(%q, %q) returned unexpected error: %v", tt.sha, tt.message, err)
			}
			if entry.SHA() != tt.wantSHA {
				t.Errorf("entry.SHA() = %q, want %q", entry.SHA(), tt.wantSHA)
			}
			if entry.Message() != tt.wantMsg {
				t.Errorf("entry.Message() = %q, want %q", entry.Message(), tt.wantMsg)
			}
		})
	}
}

func TestNewCommitEntry_InvalidSHA(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		sha     string
		message string
	}{
		{"empty SHA", "", "feat: some message"},
		{"too short — 7 chars", "abc1234", "feat: some message"},
		{"too long — 41 chars", "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c", "feat: some message"},
		{"non-hex characters", strings.Repeat("g", 40), "feat: some message"},
		{"contains spaces", "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b ", "feat: some message"},
		{"contains special chars", "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6!", "feat: some message"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewCommitEntry(tt.sha, tt.message)
			if err == nil {
				t.Errorf("NewCommitEntry(%q, %q) expected error, got nil", tt.sha, tt.message)
			}
			if !errors.Is(err, ErrInvalidSHA) {
				t.Errorf("NewCommitEntry(%q, %q) error = %v, want ErrInvalidSHA", tt.sha, tt.message, err)
			}
		})
	}
}

func TestNewCommitEntry_EmptyMessage(t *testing.T) {
	t.Parallel()

	validSHA := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"

	tests := []struct {
		name    string
		sha     string
		message string
	}{
		{"empty string message", validSHA, ""},
		{"whitespace-only message", validSHA, "   "},
		{"tab-only message", validSHA, "\t"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewCommitEntry(tt.sha, tt.message)
			if err == nil {
				t.Errorf("NewCommitEntry(%q, %q) expected error, got nil", tt.sha, tt.message)
			}
			if !errors.Is(err, ErrEmptyMessage) {
				t.Errorf("NewCommitEntry(%q, %q) error = %v, want ErrEmptyMessage", tt.sha, tt.message, err)
			}
		})
	}
}

func TestNewCommitEntry_OptionalFields(t *testing.T) {
	t.Parallel()

	validSHA := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	message := "feat: add feature"

	t.Run("WithAuthor sets Author field", func(t *testing.T) {
		t.Parallel()
		entry, err := NewCommitEntry(validSHA, message, WithAuthor("John Doe"))
		if err != nil {
			t.Fatalf("NewCommitEntry with WithAuthor returned unexpected error: %v", err)
		}
		if entry.Author() != "John Doe" {
			t.Errorf("entry.Author() = %q, want %q", entry.Author(), "John Doe")
		}
	})

	t.Run("WithDate sets Date field", func(t *testing.T) {
		t.Parallel()
		entry, err := NewCommitEntry(validSHA, message, WithDate("2026-05-23T12:00:00Z"))
		if err != nil {
			t.Fatalf("NewCommitEntry with WithDate returned unexpected error: %v", err)
		}
		if entry.Date() != "2026-05-23T12:00:00Z" {
			t.Errorf("entry.Date() = %q, want %q", entry.Date(), "2026-05-23T12:00:00Z")
		}
	})

	t.Run("WithAuthor and WithDate together", func(t *testing.T) {
		t.Parallel()
		entry, err := NewCommitEntry(validSHA, message, WithAuthor("Jane"), WithDate("2026-01-01T00:00:00Z"))
		if err != nil {
			t.Fatalf("NewCommitEntry with both options returned unexpected error: %v", err)
		}
		if entry.Author() != "Jane" {
			t.Errorf("entry.Author() = %q, want %q", entry.Author(), "Jane")
		}
		if entry.Date() != "2026-01-01T00:00:00Z" {
			t.Errorf("entry.Date() = %q, want %q", entry.Date(), "2026-01-01T00:00:00Z")
		}
	})

	t.Run("no options yields zero-value Author and Date", func(t *testing.T) {
		t.Parallel()
		entry, err := NewCommitEntry(validSHA, message)
		if err != nil {
			t.Fatalf("NewCommitEntry without options returned unexpected error: %v", err)
		}
		if entry.Author() != "" {
			t.Errorf("entry.Author() = %q, want empty string", entry.Author())
		}
		if entry.Date() != "" {
			t.Errorf("entry.Date() = %q, want empty string", entry.Date())
		}
	})
}

func TestMessages(t *testing.T) {
	t.Parallel()

	t.Run("empty slice returns empty slice", func(t *testing.T) {
		t.Parallel()
		result := Messages([]CommitEntry{})
		if len(result) != 0 {
			t.Errorf("Messages(empty) = %v, want empty slice", result)
		}
	})

	t.Run("multiple entries returns messages in order", func(t *testing.T) {
		t.Parallel()
		validSHA1 := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
		validSHA2 := "b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3"
		validSHA3 := "c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4"

		e1, _ := NewCommitEntry(validSHA1, "feat: first")
		e2, _ := NewCommitEntry(validSHA2, "fix: second")
		e3, _ := NewCommitEntry(validSHA3, "chore: third")

		result := Messages([]CommitEntry{e1, e2, e3})
		if len(result) != 3 {
			t.Fatalf("Messages(3 entries) returned %d messages, want 3", len(result))
		}
		want := []string{"feat: first", "fix: second", "chore: third"}
		for i, msg := range result {
			if msg != want[i] {
				t.Errorf("Messages()[%d] = %q, want %q", i, msg, want[i])
			}
		}
	})

	t.Run("nil slice returns nil", func(t *testing.T) {
		t.Parallel()
		result := Messages(nil)
		if result != nil {
			t.Errorf("Messages(nil) = %v, want nil", result)
		}
	})
}