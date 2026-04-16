package domain

import "testing"

// --- CalculateBump ---

func TestCalculateBump_MajorBreakingExclamation(t *testing.T) {
	cases := []struct {
		name     string
		messages []string
	}{
		{"feat! before colon", []string{"feat!: redesign DB schema"}},
		{"fix! before colon", []string{"fix!: remove old API"}},
		{"type with scope feat!(scope)", []string{"feat!(auth)!: breaking change"}},
		{"mixed with minor — major wins", []string{"feat: add button", "fix!: remove endpoint"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CalculateBump(tc.messages); got != "major" {
				t.Errorf("CalculateBump(%v) = %q, want %q", tc.messages, got, "major")
			}
		})
	}
}

func TestCalculateBump_MajorBreakingChange(t *testing.T) {
	messages := []string{
		"feat: add feature\n\nBREAKING CHANGE: old behavior removed",
		"BREAKING CHANGE: complete rewrite",
	}
	for _, msg := range messages {
		if got := CalculateBump([]string{msg}); got != "major" {
			t.Errorf("CalculateBump([%q]) = %q, want %q", msg, got, "major")
		}
	}
}

func TestCalculateBump_Minor(t *testing.T) {
	cases := []struct {
		name     string
		messages []string
	}{
		{"feat: prefix", []string{"feat: add dark mode"}},
		{"feat( prefix with scope", []string{"feat(ui): new button component"}},
		{"minor beats patch", []string{"fix: typo", "feat: new endpoint", "chore: cleanup"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CalculateBump(tc.messages); got != "minor" {
				t.Errorf("CalculateBump(%v) = %q, want %q", tc.messages, got, "minor")
			}
		})
	}
}

func TestCalculateBump_Patch(t *testing.T) {
	cases := []struct {
		name     string
		messages []string
	}{
		{"fix commit", []string{"fix: null pointer in auth"}},
		{"chore commit", []string{"chore: update deps"}},
		{"docs commit", []string{"docs: update readme"}},
		{"test commit", []string{"test: add unit tests"}},
		{"refactor commit", []string{"refactor: extract helper"}},
		{"multiple patches", []string{"fix: a", "fix: b", "chore: c"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CalculateBump(tc.messages); got != "patch" {
				t.Errorf("CalculateBump(%v) = %q, want %q", tc.messages, got, "patch")
			}
		})
	}
}

func TestCalculateBump_EmptyMessages(t *testing.T) {
	cases := [][]string{
		{},
		{"", "   ", "\t"},
	}
	for _, msgs := range cases {
		got := CalculateBump(msgs)
		if got != "patch" {
			t.Errorf("CalculateBump(%v) = %q, want %q", msgs, got, "patch")
		}
	}
}

func TestCalculateBump_MajorTrumpsAll(t *testing.T) {
	messages := []string{
		"feat: new dashboard",
		"fix: alignment bug",
		"BREAKING CHANGE: removed legacy API",
		"chore: cleanup",
	}
	if got := CalculateBump(messages); got != "major" {
		t.Errorf("CalculateBump() = %q, want %q", got, "major")
	}
}

// --- BumpVersion ---

func TestBumpVersion_Major(t *testing.T) {
	cases := []struct{ tag, want string }{
		{"v1.4.2", "v2.0.0"},
		{"v0.9.3", "v1.0.0"},
		{"v10.5.1", "v11.0.0"},
		{"1.4.2", "2.0.0"}, // without v prefix
	}
	for _, tc := range cases {
		t.Run(tc.tag, func(t *testing.T) {
			got, err := BumpVersion(tc.tag, "major")
			if err != nil {
				t.Fatalf("BumpVersion(%q, major) error: %v", tc.tag, err)
			}
			if got != tc.want {
				t.Errorf("BumpVersion(%q, major) = %q, want %q", tc.tag, got, tc.want)
			}
		})
	}
}

func TestBumpVersion_Minor(t *testing.T) {
	cases := []struct{ tag, want string }{
		{"v1.4.2", "v1.5.0"},
		{"v0.0.0", "v0.1.0"},
		{"v2.9.8", "v2.10.0"},
	}
	for _, tc := range cases {
		t.Run(tc.tag, func(t *testing.T) {
			got, err := BumpVersion(tc.tag, "minor")
			if err != nil {
				t.Fatalf("BumpVersion(%q, minor) error: %v", tc.tag, err)
			}
			if got != tc.want {
				t.Errorf("BumpVersion(%q, minor) = %q, want %q", tc.tag, got, tc.want)
			}
		})
	}
}

func TestBumpVersion_Patch(t *testing.T) {
	cases := []struct{ tag, want string }{
		{"v1.4.2", "v1.4.3"},
		{"v0.0.0", "v0.0.1"},
		{"v3.2.9", "v3.2.10"},
	}
	for _, tc := range cases {
		t.Run(tc.tag, func(t *testing.T) {
			got, err := BumpVersion(tc.tag, "patch")
			if err != nil {
				t.Fatalf("BumpVersion(%q, patch) error: %v", tc.tag, err)
			}
			if got != tc.want {
				t.Errorf("BumpVersion(%q, patch) = %q, want %q", tc.tag, got, tc.want)
			}
		})
	}
}

func TestBumpVersion_PreReleaseSuffix(t *testing.T) {
	// Tags with pre-release suffix should parse the numeric part
	got, err := BumpVersion("v1.4.2-beta", "patch")
	if err != nil {
		t.Fatalf("BumpVersion(v1.4.2-beta, patch) error: %v", err)
	}
	if got != "v1.4.3" {
		t.Errorf("BumpVersion(v1.4.2-beta, patch) = %q, want %q", got, "v1.4.3")
	}
}

func TestBumpVersion_InvalidTag(t *testing.T) {
	cases := []string{
		"v1.0",       // only 2 parts
		"vfoo.1.0",   // non-numeric major
		"not-semver",
	}
	for _, tag := range cases {
		_, err := BumpVersion(tag, "patch")
		if err == nil {
			t.Errorf("BumpVersion(%q, patch) expected error, got nil", tag)
		}
	}
}

func TestBumpVersion_InvalidBump(t *testing.T) {
	_, err := BumpVersion("v1.0.0", "mega")
	if err == nil {
		t.Error("BumpVersion with unknown bump type expected error, got nil")
	}
}
