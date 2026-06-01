package chunkers

import (
	"log"

	"github.com/blak0p/git-courer/internal/infra/chunkers/ext_lib"
)

// ConfigureGrammarCache sets the directory where grammars are downloaded and cached.
func ConfigureGrammarCache(cacheDir string) error {
	// ext_lib.Configure takes a PackConfig struct (value, not pointer).
	// CacheDir field is a *string.
	return ext_lib.Configure(ext_lib.PackConfig{
		CacheDir: &cacheDir,
	})
}

// EnsureLanguages ensures that the specified languages are available and downloads them if needed.
func EnsureLanguages(names []string) error {
	log.Printf("[INFO] Ensuring grammars for: %v", names)
	// The function is called Download at the package level
	_, err := ext_lib.Download(names)
	return err
}

// ProcessResult is a wrapper for ext_lib.ProcessResult.
type ProcessResult = ext_lib.ProcessResult

// AnalyzeSource parses source into a ProcessResult using the high-level kreuzberg API.
func AnalyzeSource(langName string, src []byte) (*ProcessResult, error) {
	// Create a minimal config for analysis
	// Use NewProcessConfig to get defaults
	config := ext_lib.ProcessConfig{
		Language:  langName,
		Structure: ptr(true),
		Symbols:   true,
		Imports:   ptr(true),
	}
	return ext_lib.Process(string(src), config)
}

func ptr[T any](v T) *T {
	return &v
}
