package chunkers

import (
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/data"
	"github.com/Alejandro-M-P/git-courer/internal/infra/chunkers/ext_lib"
)

// LanguageCatalog is an alias to domain.LanguageCatalog for backward compatibility.
// All methods are defined on domain.LanguageCatalog; this package provides
// the factory function that builds the catalog from embedded data.
type LanguageCatalog = domain.LanguageCatalog

// LanguageEntry is an alias to domain.LanguageEntry for backward compatibility.
type LanguageEntry = domain.LanguageEntry

// extDetector wraps ext_lib.DetectLanguageFromExtension as a domain.ExtDetector.
var extDetector domain.ExtDetector = func(ext string) *string {
	return ext_lib.DetectLanguageFromExtension(ext)
}

// NewLanguageCatalog creates a new LanguageCatalog with all languages loaded
// from the embedded languages.json data. The extension detector is wired to
// use the kreuzberg grammar registry for language detection.
func NewLanguageCatalog() *LanguageCatalog {
	// Force data loading by calling LoadLanguages explicitly
	if err := data.LoadLanguages(); err != nil {
		panic("Failed to load languages: " + err.Error())
	}

	// Build language maps from embedded data
	byName := make(map[string]data.LanguageNodes)
	testIndex := make(map[string][]data.TestPattern)

	for langName, nodes := range getAllLanguages() {
		byName[langName] = nodes
		if len(nodes.TestPatterns) > 0 {
			testIndex[langName] = nodes.TestPatterns
		}
	}

	return domain.NewLanguageCatalog(byName, testIndex, extDetector)
}

// getAllLanguages returns all languages from the embedded data.
func getAllLanguages() map[string]data.LanguageNodes {
	result := make(map[string]data.LanguageNodes)

	langNames := data.GetAllLanguageNames()
	if len(langNames) == 0 {
		panic("No language names found - data not loaded")
	}

	for _, langName := range langNames {
		if nodes, ok := data.GetLanguageNodes(langName); ok {
			result[langName] = nodes
		}
	}

	return result
}
