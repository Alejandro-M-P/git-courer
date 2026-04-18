package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "secret_*.go")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

func TestDetect_KnownPatterns(t *testing.T) {
	cases := []struct {
		name     string
		content  string
		wantType string
	}{
		{"openai_key", `apiKey = "sk-abcdefghijklmnopqrst"`, "openai_key"},
		{"github_token", `token = "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZ123456abcd"`, "github_token"},
		{"aws_access_key", `AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE`, "aws_access_key"},
		{"google_api_key", `key = "AIzaSyD-9tSrke72Ypx8Sf3BBBbbbbbbbbbbbbbb"`, "google_api_key"},
		{"stripe_fake_key", `stripe_key = "sk_live_xxxx_TEST_FAKE_NOT_REAL_KEY"`, "stripe_live_key"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file := writeTemp(t, tc.content)
			results, err := Detect([]string{file})
			if err != nil {
				t.Fatalf("Detect() error: %v", err)
			}
			found := false
			for _, r := range results {
				if r.Type == tc.wantType {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected secret type %q not detected in %q", tc.wantType, tc.content)
			}
		})
	}
}

func TestDetect_NewPatterns(t *testing.T) {
	cases := []struct {
		name     string
		content  string
		wantType string
	}{
		{
			"anthropic_key",
			`ANTHROPIC_API_KEY=sk-ant-api03-` + repeat("A", 90),
			"anthropic_key",
		},
		{
			"huggingface_token",
			`HF_TOKEN=hf_` + repeat("A", 34),
			"huggingface_token",
		},
		{
			"replicate_token",
			`REPLICATE_API_TOKEN=r8_` + repeat("A", 40),
			"replicate_token",
		},
		{
			"jwt_token",
			`token = "eyJ` + repeat("a", 100) + `"`,
			"jwt_token",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file := writeTemp(t, tc.content)
			results, err := Detect([]string{file})
			if err != nil {
				t.Fatalf("Detect() error: %v", err)
			}
			found := false
			for _, r := range results {
				if r.Type == tc.wantType {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected secret type %q not detected", tc.wantType)
			}
		})
	}
}

func TestDetect_SensitiveExtensions(t *testing.T) {
	dir := t.TempDir()
	pemFile := filepath.Join(dir, "server.pem")
	os.WriteFile(pemFile, []byte("fake pem content"), 0644)

	results, err := Detect([]string{pemFile})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected .pem file to be flagged")
	}
}

func TestDetect_CommentsSkipped(t *testing.T) {
	content := "// sk-abcdefghijklmnopqrst\n# AKIAIOSFODNN7EXAMPLE\n"
	file := writeTemp(t, content)
	results, err := Detect([]string{file})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected no results for commented secrets, got %d", len(results))
	}
}

func TestDetect_EmptyFileList(t *testing.T) {
	results, err := Detect([]string{})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected no results for empty file list")
	}
}

func repeat(s string, n int) string {
	result := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		result = append(result, s...)
	}
	return string(result)
}
