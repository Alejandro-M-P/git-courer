package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetect_SensitiveExtensions(t *testing.T) {
	// Approval test: Detect must flag sensitive extensions via sensitiveExts
	tests := []struct {
		name      string
		ext       string
		wantType  string
		wantFound bool
	}{
		{".env file", ".env", "env_file", true},
		{".pem file", ".pem", "private_key", true},
		{".key file", ".key", "private_key", true},
		{".p12 file", ".p12", "keystore", true},
		{".pkcs8 file", ".pkcs8", "private_key", true},
		{".keystore file", ".keystore", "keystore", true},
		{".go file not sensitive", ".go", "", false},
	}

	tmpDir := t.TempDir()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := filepath.Join(tmpDir, "test"+tt.ext)
			if err := os.WriteFile(filename, []byte("content"), 0644); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}

			detections, err := Detect([]string{filename})
			if err != nil {
				t.Fatalf("Detect() error = %v", err)
			}
			if tt.wantFound {
				if len(detections) == 0 {
					t.Errorf("Detect(%q) expected detection of type %q, got none", filename, tt.wantType)
				} else if detections[0].Type != tt.wantType {
					t.Errorf("Detect(%q) type = %q, want %q", filename, detections[0].Type, tt.wantType)
				}
			} else {
				if len(detections) > 0 {
					t.Errorf("Detect(%q) expected no detection, got %v", filename, detections)
				}
			}
		})
	}
}

func TestDetect_FilenameSubstringChecks(t *testing.T) {
	// Approval test: After delegating to IsBlacklistedName, Detect flags
	// blacklisted filenames correctly — including id_rsa, credentials, secrets, password
	tests := []struct {
		name      string
		filename  string
		wantFound bool
	}{
		{"credentials.json", "credentials.json", true},
		{"secrets.yml", "secrets.yml", true},
		{"password.txt", "password.txt", true},
		{".env via extension", ".env", true},
		{".env.local via extension", ".env.local", true},
		{"credentials in subdir", "config/credentials.json", true},
		{"id_rsa via IsBlacklistedName", "id_rsa", true},
		{"id_ed25519 via IsBlacklistedName", "id_ed25519", true},
		{"git-courer binary via IsBlacklistedName", "git-courer", true},
		{".env.example not blacklisted", ".env.example", false},
		{".env.template not blacklisted", ".env.template", false},
	}

	tmpDir := t.TempDir()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := filepath.Join(tmpDir, tt.filename)
			dir := filepath.Dir(filename)
			if dir != tmpDir {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatalf("failed to create dir: %v", err)
				}
			}
			if err := os.WriteFile(filename, []byte("content"), 0644); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}

			detections, err := Detect([]string{filename})
			if err != nil {
				t.Fatalf("Detect() error = %v", err)
			}
			if tt.wantFound {
				if len(detections) == 0 {
					t.Errorf("Detect(%q) expected detection, got none", filename)
				}
			} else {
				if len(detections) > 0 {
					t.Errorf("Detect(%q) expected no detection, got %v", filename, detections)
				}
			}
		})
	}
}

func TestDetect_EmptyInput(t *testing.T) {
	detections, err := Detect(nil)
	if err != nil {
		t.Fatalf("Detect(nil) error = %v", err)
	}
	if detections != nil {
		t.Errorf("Detect(nil) = %v, want nil", detections)
	}
}

func TestDetectInContent(t *testing.T) {
	// Test that DetectInContent scans a diff string for patterns
	content := `+ AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
+ some normal line
+stripe_key = sk_live_26PHem9Oh3Y9oayqM61e`
	findings := DetectInContent(content)
	if len(findings) == 0 {
		t.Error("DetectInContent() expected findings, got none")
	}
}

func TestDetect_DelegatesToIsBlacklistedName(t *testing.T) {
	// Triangulation test: After refactoring, Detect will delegate filename checks
	// to IsBlacklistedName, which catches more patterns than the old inline check.
	// This test uses files that IsBlacklistedName flags but the old Detect didn't.
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		filename  string
		wantFound bool
	}{
		// IsBlacklistedName catches these (exact match)
		{"id_rsa via IsBlacklistedName", "id_rsa", true},
		{"id_ed25519 via IsBlacklistedName", "id_ed25519", true},
		{"git-courer binary via IsBlacklistedName", "git-courer", true},
		// .env.extension should NOT be flagged (exception in IsBlacklistedName)
		{".env.example not blacklisted", ".env.example", false},
		{".env.template not blacklisted", ".env.template", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(filename, []byte("content"), 0644); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}

			detections, err := Detect([]string{filename})
			if err != nil {
				t.Fatalf("Detect() error = %v", err)
			}
			if tt.wantFound {
				if len(detections) == 0 {
					t.Errorf("Detect(%q) expected detection after IsBlacklistedName delegation, got none", filename)
				}
			} else {
				if len(detections) > 0 {
					t.Errorf("Detect(%q) expected no detection after IsBlacklistedName delegation, got %v", filename, detections)
				}
			}
		})
	}
}
