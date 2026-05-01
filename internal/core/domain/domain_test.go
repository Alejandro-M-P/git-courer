package domain

import (
	"testing"
	"time"
)

// --- IsValidTagName ---

func TestIsValidTagName_Valid(t *testing.T) {
	t.Parallel()
	valid := []string{
		"v1.0.0", "1.0.0", "v0.0.1", "v10.20.30",
		"v1.2.3-beta", "1.2.3-rc1", "v0.9.0-alpha",
	}
	for _, tag := range valid {
		t.Run(tag, func(t *testing.T) {
			t.Parallel()
			if !IsValidTagName(tag) {
				t.Errorf("IsValidTagName(%q) = false, want true", tag)
			}
		})
	}
}

func TestIsValidTagName_Invalid(t *testing.T) {
	t.Parallel()
	invalid := []string{
		"", "latest", "v1.0", "1.0", "v1.0.0.0",
		"v1.0.0-beta.1", "v1.0.0.1", "abc", "v",
		"1.2.x", "v1.2.3.4",
	}
	for _, tag := range invalid {
		t.Run(tag, func(t *testing.T) {
			t.Parallel()
			if IsValidTagName(tag) {
				t.Errorf("IsValidTagName(%q) = true, want false", tag)
			}
		})
	}
}

// --- Status ---

func TestStatus_IsClean(t *testing.T) {
	t.Parallel()
	s := Status{IsClean: true, Branch: "main"}
	if !s.IsClean {
		t.Error("Status.IsClean should be true")
	}
	if s.Branch != "main" {
		t.Errorf("Status.Branch = %q, want %q", s.Branch, "main")
	}
}

func TestStatus_Files(t *testing.T) {
	t.Parallel()
	s := Status{
		IsClean: false,
		Staged:  1, Modified: 2, Untracked: 3,
		Files: []FileStatus{
			{Path: "main.go", Status: "M ", Staged: true},
			{Path: "test.go", Status: "??", IsNew: true},
		},
	}
	if len(s.Files) != 2 {
		t.Errorf("len(Files) = %d, want 2", len(s.Files))
	}
	if s.Files[0].Path != "main.go" {
		t.Errorf("Files[0].Path = %q, want main.go", s.Files[0].Path)
	}
}

// --- Backup ---

func TestBackup_Fields(t *testing.T) {
	t.Parallel()
	now := time.Now()
	b := Backup{
		Ref:       "refs/git-courer/backup/20240101_commit",
		HasStash:  true,
		Operation: "commit",
		CreatedAt: now,
	}
	if b.Ref == "" {
		t.Error("Backup.Ref should not be empty")
	}
	if !b.HasStash {
		t.Error("Backup.HasStash should be true")
	}
	if b.Operation != "commit" {
		t.Errorf("Backup.Operation = %q, want commit", b.Operation)
	}
	if b.CreatedAt != now {
		t.Error("Backup.CreatedAt mismatch")
	}
}

// --- ReleaseIntent ---

func TestReleaseIntent_Fields(t *testing.T) {
	t.Parallel()
	intent := ReleaseIntent{
		TagName:     "v1.2.0",
		IsRelease:   true,
		VersionBump: "minor",
		Changelog:   "## Added\n- feature",
		BranchFrom:  "develop",
		MergePath:   []string{"develop->main"},
	}
	if intent.TagName != "v1.2.0" {
		t.Errorf("TagName = %q, want v1.2.0", intent.TagName)
	}
	if !intent.IsRelease {
		t.Error("IsRelease should be true")
	}
	if len(intent.MergePath) != 1 {
		t.Errorf("len(MergePath) = %d, want 1", len(intent.MergePath))
	}
}

// --- ModelSize ---

func TestModelSizeShouldUseLLMScan(t *testing.T) {
	t.Parallel()
	cases := []struct {
		size ModelSize
		want bool
	}{
		{ModelSizeLarge, true},
		{ModelSizeMedium, false},
		{ModelSizeSmall, false},
	}
	for _, tc := range cases {
		t.Run(string(tc.size), func(t *testing.T) {
			t.Parallel()
			got := tc.size.ShouldUseLLMSecurityScan()
			if got != tc.want {
				t.Errorf("ModelSize(%q).ShouldUseLLMSecurityScan() = %v, want %v", tc.size, got, tc.want)
			}
		})
	}
}

// --- SecurityResult ---

func TestSecurityResult_Fields(t *testing.T) {
	t.Parallel()
	r := SecurityResult{
		Safe:    false,
		Halted:  true,
		Reason:  ReasonSecretDetected,
		File:    "config.go",
		Line:    42,
		Type:    "aws_access_key",
		Message: "[SECURITY] secret found",
	}
	if r.Reason != ReasonSecretDetected {
		t.Errorf("Reason = %q, want %q", r.Reason, ReasonSecretDetected)
	}
	if r.Line != 42 {
		t.Errorf("Line = %d, want 42", r.Line)
	}
}

func TestSecurityReasons_Constants(t *testing.T) {
	t.Parallel()
	reasons := []SecurityReason{
		ReasonBlacklistedFile,
		ReasonBlacklistedFolder,
		ReasonBinaryFile,
		ReasonSecretDetected,
		ReasonSelfBinary,
	}
	seen := make(map[SecurityReason]bool)
	for _, r := range reasons {
		t.Run(string(r), func(t *testing.T) {
			t.Parallel()
			if r == "" {
				t.Error("SecurityReason constant should not be empty string")
			}
			if seen[r] {
				// No podemos usar seen aquí si corremos en paralelo sin mutex,
				// pero t.Run garantiza orden si no llamamos a t.Parallel()
				// dentro del bucle o si controlamos el acceso.
				// Para este test simple de constantes, omitiremos t.Parallel() interno.
			}
		})
	}
	// Re-check without parallel for the duplicate logic
	for _, r := range reasons {
		if seen[r] {
			t.Errorf("duplicate SecurityReason: %q", r)
		}
		seen[r] = true
	}
}
