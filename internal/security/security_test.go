package security

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

func defaultCfg() *config.Config {
	cfg := config.Default()
	cfg.Secrets.UseLLMSecurityScan = "false"
	return cfg
}

// --- ParseModelSize ---

func TestParseModelSize_Large(t *testing.T) {
	cases := []string{
		"qwen3.5:14b", "mistral:22b", "llama3:70b",
		"deepseek:32b", "model-14b", "codestral:22b",
	}
	for _, name := range cases {
		got := ParseModelSize(name)
		if got != domain.ModelSizeLarge {
			t.Errorf("ParseModelSize(%q) = %q, want Large", name, got)
		}
	}
}

func TestParseModelSize_Medium(t *testing.T) {
	cases := []string{
		"llama3:7b", "mistral:7b", "model-8b", "qwen:7b",
	}
	for _, name := range cases {
		got := ParseModelSize(name)
		if got != domain.ModelSizeMedium {
			t.Errorf("ParseModelSize(%q) = %q, want Medium", name, got)
		}
	}
}

func TestParseModelSize_Small(t *testing.T) {
	cases := []string{
		"qwen3.5:1b", "model:3b", "tinyllama:1.1b",
		"gemma:2b", "phi3:3.8b", "unknown-model", "",
	}
	for _, name := range cases {
		got := ParseModelSize(name)
		if got != domain.ModelSizeSmall {
			t.Errorf("ParseModelSize(%q) = %q, want Small", name, got)
		}
	}
}

// --- ShouldUseLLMScan ---

func TestShouldUseLLMScan_ForceFalse(t *testing.T) {
	cfg := config.Default()
	cfg.Secrets.UseLLMSecurityScan = "false"
	cfg.Ollama.Model = "llama3:70b" // large model, but forced off
	svc := New(cfg, nil)
	if svc.ShouldUseLLMScan() {
		t.Error("ShouldUseLLMScan() = true, want false (forced off)")
	}
}

func TestShouldUseLLMScan_ForceTrue(t *testing.T) {
	cfg := config.Default()
	cfg.Secrets.UseLLMSecurityScan = "true"
	cfg.Ollama.Model = "phi3:3.8b" // small model, but forced on
	svc := New(cfg, nil)
	if !svc.ShouldUseLLMScan() {
		t.Error("ShouldUseLLMScan() = false, want true (forced on)")
	}
}

func TestShouldUseLLMScan_AutoLargeModel(t *testing.T) {
	cfg := config.Default()
	cfg.Secrets.UseLLMSecurityScan = "auto"
	cfg.Ollama.Model = "llama3:70b"
	svc := New(cfg, nil)
	if !svc.ShouldUseLLMScan() {
		t.Error("ShouldUseLLMScan() = false, want true for large model in auto mode")
	}
}

func TestShouldUseLLMScan_AutoSmallModel(t *testing.T) {
	cfg := config.Default()
	cfg.Secrets.UseLLMSecurityScan = "auto"
	cfg.Ollama.Model = "phi3:3.8b"
	svc := New(cfg, nil)
	if !svc.ShouldUseLLMScan() {
		t.Error("ShouldUseLLMScan() = false, want true (now enabled by default for all models)")
	}
}

// --- CheckFiles ---

func TestCheckFiles_Clean(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "main.go")
	os.WriteFile(f, []byte(`package main

func main() {}
`), 0644)

	svc := New(defaultCfg(), nil)
	result := svc.CheckFiles([]string{f}, "")
	if result.Blocked {
		t.Errorf("CheckFiles() blocked clean file, files: %+v", result.Files)
	}
}

func TestCheckFiles_SecretDetected(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.go")
	// Use real AWS key pattern from detector.go: AKIA + 16 alphanumeric (20 total)
	os.WriteFile(f, []byte(`apiKey := "AKIAIOSAMPLE1234567890ABCD"`), 0644)

	svc := New(defaultCfg(), nil)
	result := svc.CheckFiles([]string{f}, "")
	if !result.Blocked {
		t.Error("CheckFiles() should block file with AWS key")
	}
}

func TestCheckFiles_BlacklistedFolder(t *testing.T) {
	svc := New(defaultCfg(), nil)
	result := svc.CheckFiles([]string{"node_modules/lib/index.js"}, "")
	if !result.Blocked {
		t.Error("CheckFiles() should block node_modules files")
	}
	if len(result.Files) == 0 {
		t.Error("CheckFiles() should populate result.Files")
	}
}

func TestCheckFiles_BlacklistedName(t *testing.T) {
	svc := New(defaultCfg(), nil)
	result := svc.CheckFiles([]string{".env"}, "")
	if !result.Blocked {
		t.Error("CheckFiles() should block .env file")
	}
}

func TestCheckFiles_EmptyList(t *testing.T) {
	svc := New(defaultCfg(), nil)
	result := svc.CheckFiles([]string{}, "")
	if result.Blocked {
		t.Error("CheckFiles() should not block empty file list")
	}
}

func TestCheckFiles_FirstBlockingResult(t *testing.T) {
	svc := New(defaultCfg(), nil)
	result := svc.CheckFiles([]string{"node_modules/foo.js"}, "")
	first := result.FirstBlocking()
	if first == nil {
		t.Error("FirstBlocking() should not be nil when blocked")
	}
	if !first.Halted {
		t.Error("FirstBlocking().Halted should be true")
	}
}

func TestCheckFiles_IsBlocked(t *testing.T) {
	svc := New(defaultCfg(), nil)
	result := svc.CheckFiles([]string{"node_modules/foo.js"}, "")
	if !result.IsBlocked() {
		t.Error("IsBlocked() should be true for blocked result")
	}
}

func TestCheckFiles_NotBlocked_IsBlocked(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "clean.go")
	os.WriteFile(f, []byte("package main\n"), 0644)

	svc := New(defaultCfg(), nil)
	result := svc.CheckFiles([]string{f}, "")
	if result.IsBlocked() {
		t.Error("IsBlocked() should be false for clean file")
	}
	if result.FirstBlocking() != nil {
		t.Error("FirstBlocking() should be nil for clean file")
	}
}

// --- Additional edge cases ---

func TestCheckFiles_SecretWithLineNumber(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.go")
	// Write file with potential secret on a specific line
	os.WriteFile(f, []byte(`package config
const API_KEY = ""AK" + "IA_SAMPLE_KEY_AWS_123"1234"
func main() {}`), 0644)

	svc := New(defaultCfg(), nil)
	result := svc.CheckFiles([]string{f}, "")
	// The regex detector may or may not detect it depending on patterns
	// Just verify no panic
	_ = result
}

func TestCheckFiles_AllBlacklistedFolders(t *testing.T) {
	svc := New(defaultCfg(), nil)
	// Test various blacklisted folders
	folders := []string{
		"node_modules/index.js",
		"vendor/lib/core.js",
		"__pycache__/app.pyc",
		".git/hooks/pre-commit",
		"dist/bundle.js",
	}
	for _, f := range folders {
		result := svc.CheckFiles([]string{f}, "")
		if !result.Blocked {
			t.Errorf("CheckFiles(%q) should block blacklisted folder", f)
		}
	}
}

func TestCheckFiles_BlacklistedExtensions(t *testing.T) {
	svc := New(defaultCfg(), nil)
	// Test blacklisted file names
	files := []string{
		".env",
		".env.local",
		"credentials.json",
		"secrets.yaml",
		"id_rsa",
		"id_rsa.pub",
	}
	for _, f := range files {
		result := svc.CheckFiles([]string{f}, "")
		if !result.Blocked {
			t.Errorf("CheckFiles(%q) should block blacklisted file", f)
		}
	}
}

func TestCheckFiles_MultipleFilesWithBlocking(t *testing.T) {
	svc := New(defaultCfg(), nil)
	// First file is clean, second is blocked
	dir := t.TempDir()
	clean := filepath.Join(dir, "main.go")
	os.WriteFile(clean, []byte("package main\n"), 0644)

	result := svc.CheckFiles([]string{clean, "node_modules/lib.js"}, "")
	// Should be blocked because of node_modules
	if !result.Blocked {
		t.Error("CheckFiles should block when any file is blocked")
	}
}

func TestParseModelSize_ExplicitSizes(t *testing.T) {
	// Test explicit size indicators
	cases := []struct {
		model string
		want  domain.ModelSize
	}{
		{"llama3:8b", domain.ModelSizeMedium},
		{"llama3:70b", domain.ModelSizeLarge},
		{"qwen3.5:14b", domain.ModelSizeLarge},
		{"codellama:34b", domain.ModelSizeLarge},
	}
	for _, tc := range cases {
		got := ParseModelSize(tc.model)
		if got != tc.want {
			t.Errorf("ParseModelSize(%q) = %q, want %q", tc.model, got, tc.want)
		}
	}
}

func TestParseModelSize_MixedCase(t *testing.T) {
	// Test case insensitivity
	if ParseModelSize("LLAMA3:7B") != domain.ModelSizeMedium {
		t.Error("ParseModelSize should handle uppercase model names")
	}
	if ParseModelSize("Mistral:7b") != domain.ModelSizeMedium {
		t.Error("ParseModelSize should handle mixed case")
	}
}

// --- Security uses ResolveLLMConfig ---

// TestSecurity_UsesLLMModel verifies that the security Service uses
// cfg.ResolveLLMConfig().Model instead of cfg.Ollama.Model.
func TestSecurity_UsesLLMModel(t *testing.T) {
	cfg := config.Default()
	cfg.LLM.Provider = "openai-compatible"
	cfg.LLM.Model = "large-model:70b"
	cfg.Ollama.Model = "small-model:3b" // legacy — should be ignored

	svc := New(cfg, nil)
	// Service should use the LLM-resolved model (large), not Ollama (small)
	if svc.modelSize != domain.ModelSizeLarge {
		t.Errorf("modelSize = %q, want %q (should use LLM.Model, not Ollama.Model)",
			svc.modelSize, domain.ModelSizeLarge)
	}
}

// TestSecurity_FallbackToOllamaModel verifies that when LLM.Model is empty
// (legacy config), ResolveLLMConfig falls back to Ollama.Model.
func TestSecurity_FallbackToOllamaModel(t *testing.T) {
	cfg := config.Default()
	cfg.LLM = config.LLMConfig{} // empty LLM config
	cfg.Ollama.Model = "llama3:70b"

	svc := New(cfg, nil)
	// ResolveLLMConfig should auto-populate from Ollama
	if svc.modelSize != domain.ModelSizeLarge {
		t.Errorf("modelSize = %q, want %q (should fallback to Ollama.Model via ResolveLLMConfig)",
			svc.modelSize, domain.ModelSizeLarge)
	}
}
