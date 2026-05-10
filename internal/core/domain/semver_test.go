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

// --- REQ-1: BumpVersion Preserves Prerelease ---

func TestBumpVersion_PreservesPrerelease(t *testing.T) {
	cases := []struct {
		tag, bump, want string
	}{
		// REQ-1 scenarios
		{"v1.0.0-alpha", "patch", "v1.0.1-alpha"},
		{"v1.0.0-alpha.b.1", "minor", "v1.1.0-alpha.b.1"},
		{"v2.0.0-rc.1", "patch", "v2.0.1-rc.1"},
		// No prerelease — unchanged behavior
		{"v1.0.0", "major", "v2.0.0"},
		// Without v prefix, with prerelease
		{"1.0.0-alpha", "patch", "1.0.1-alpha"},
		// Major bump with prerelease preserves suffix
		{"v1.0.0-rc.1", "major", "v2.0.0-rc.1"},
	}
	for _, tc := range cases {
		t.Run(tc.tag+"_"+tc.bump, func(t *testing.T) {
			got, err := BumpVersion(tc.tag, tc.bump)
			if err != nil {
				t.Fatalf("BumpVersion(%q, %q) error: %v", tc.tag, tc.bump, err)
			}
			if got != tc.want {
				t.Errorf("BumpVersion(%q, %q) = %q, want %q", tc.tag, tc.bump, got, tc.want)
			}
		})
	}
}

func TestBumpVersion_PreReleaseSuffix(t *testing.T) {
	// Tags with pre-release suffix should preserve the suffix on bump
	got, err := BumpVersion("v1.4.2-beta", "patch")
	if err != nil {
		t.Fatalf("BumpVersion(v1.4.2-beta, patch) error: %v", err)
	}
	if got != "v1.4.3-beta" {
		t.Errorf("BumpVersion(v1.4.2-beta, patch) = %q, want %q", got, "v1.4.3-beta")
	}
}

func TestBumpVersion_InvalidTag(t *testing.T) {
	cases := []string{
		"v1.0",     // only 2 parts
		"vfoo.1.0", // non-numeric major
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

// --- Edge cases adicionales ---

func TestCalculateBump_EdgeCases(t *testing.T) {
	cases := []struct {
		name     string
		messages []string
		want     string
	}{
		// Commas with !
		{"feat! with comma", []string{"feat!: add", "fix:"}, "major"},
		// Multiple : in message
		{"multiple colons", []string{"feat: add: something"}, "minor"},
		// Only whitespace changes
		{"style only", []string{"style: format code"}, "patch"},
		{"perf only", []string{"perf: optimize"}, "minor"}, // perf is like feat
		// Revert commits
		{"revert commit", []string{"revert: abc123"}, "patch"},
		// Merge commits
		{"merge commit", []string{"merge branch"}, "patch"},
		{"merge feat into main", []string{"merge feat/login into main"}, "patch"},
		// CI/CD
		{"ci commit", []string{"ci: update pipeline"}, "patch"},
		{"build commit", []string{"build: compile"}, "patch"},
		// Version tags in commit message (should not confuse)
		{"version in body", []string{"chore: bump to v2.0.0"}, "patch"},
		// Long messages
		{"long message", []string{"fix: this is a very long commit message describing what was fixed in detail for some file"}, "patch"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CalculateBump(tc.messages)
			if got != tc.want {
				t.Errorf("CalculateBump(%q) = %q, want %q", tc.messages, got, tc.want)
			}
		})
	}
}

func TestBumpVersion_PreReleaseVariants(t *testing.T) {
	cases := []struct {
		tag, bump, want string
	}{
		// Pre-release preserved on bump (REQ-1)
		{"v1.0.0-alpha", "major", "v2.0.0-alpha"},
		{"v1.0.0-alpha", "minor", "v1.1.0-alpha"},
		{"v1.0.0-beta", "patch", "v1.0.1-beta"},
		{"v1.0.0-rc", "minor", "v1.1.0-rc"},
		// Pre-release with dotted identifiers preserved (REQ-1)
		{"v2.0.0-rc.1", "patch", "v2.0.1-rc.1"},
		{"v1.0.0-alpha.1", "minor", "v1.1.0-alpha.1"},
	}
	for _, tc := range cases {
		t.Run(tc.tag+"_"+tc.bump, func(t *testing.T) {
			got, err := BumpVersion(tc.tag, tc.bump)
			if err != nil {
				t.Fatalf("BumpVersion(%q, %q) error: %v", tc.tag, tc.bump, err)
			}
			if got != tc.want {
				t.Errorf("BumpVersion(%q, %q) = %q, want %q", tc.tag, tc.bump, got, tc.want)
			}
		})
	}
}

func TestBumpVersion_WithVPrefixVariants(t *testing.T) {
	cases := []struct {
		tag, bump, want string
	}{
		// With v prefix
		{"v1.0.0", "major", "v2.0.0"},
		{"v1.0.0", "minor", "v1.1.0"},
		{"v1.0.0", "patch", "v1.0.1"},
		// Without v prefix
		{"1.0.0", "major", "2.0.0"},
		{"1.0.0", "minor", "1.1.0"},
		{"1.0.0", "patch", "1.0.1"},
	}
	for _, tc := range cases {
		t.Run(tc.tag+"_"+tc.bump, func(t *testing.T) {
			got, err := BumpVersion(tc.tag, tc.bump)
			if err != nil {
				t.Fatalf("BumpVersion(%q, %q) error: %v", tc.tag, tc.bump, err)
			}
			if got != tc.want {
				t.Errorf("BumpVersion(%q, %q) = %q, want %q", tc.tag, tc.bump, got, tc.want)
			}
		})
	}
}
