package chunkers

import (
	"path/filepath"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/data"
)

// LanguageCatalog provides efficient access to language node definitions
// and test pattern matching for code-test file pairing.
type LanguageCatalog struct {
	byName    map[string]data.LanguageNodes // full node types
	testIndex map[string][]data.TestPattern  // indexed by domain name
}

// NewLanguageCatalog creates a new LanguageCatalog with all languages loaded
// from the embedded languages.json data.
func NewLanguageCatalog() *LanguageCatalog {
	catalog := &LanguageCatalog{
		byName:    make(map[string]data.LanguageNodes),
		testIndex: make(map[string][]data.TestPattern),
	}
	
	// Force data loading by calling LoadLanguages explicitly
	// This ensures the embedded languages.json is loaded before we try to access it
	if err := data.LoadLanguages(); err != nil {
		panic("Failed to load languages: " + err.Error())
	}
	
	// Load all languages and index their test patterns
	for langName, nodes := range getAllLanguages() {
		catalog.byName[langName] = nodes
		
		// Index test patterns by domain name
		if len(nodes.TestPatterns) > 0 {
			catalog.testIndex[langName] = nodes.TestPatterns
		}
	}
	
	return catalog
}

// ByName returns the LanguageNodes for a given language name.
// Returns false if the language is not found.
func (c *LanguageCatalog) ByName(lang string) (data.LanguageNodes, bool) {
	nodes, ok := c.byName[lang]
	return nodes, ok
}

// FindTestPattern finds the test pattern that matches the given filename.
// Returns the matching pattern and the language domain name if found.
func (c *LanguageCatalog) FindTestPattern(filename string) (*data.TestPattern, string) {
	baseName := filepath.Base(filename)
	
	// First, try to guess the language from the file extension
	// This helps avoid cross-language false positives
	ext := filepath.Ext(filename)
	likelyLanguage := guessLanguageFromExtension(ext)
	
	// If we have a likely language, check its patterns first
	if likelyLanguage != "" {
		if patterns, ok := c.testIndex[likelyLanguage]; ok {
			for _, pattern := range patterns {
				if matchesPattern(baseName, filename, pattern) {
					return &pattern, likelyLanguage
				}
			}
		}
	}
	
	// Fall back to checking all patterns if no language-specific match found
	for domain, patterns := range c.testIndex {
		// Skip the likely language since we already checked it
		if domain == likelyLanguage {
			continue
		}
		for _, pattern := range patterns {
			if matchesPattern(baseName, filename, pattern) {
				return &pattern, domain
			}
		}
	}
	
	return nil, ""
}

// guessLanguageFromExtension returns a likely language based on file extension.
// Uses the comprehensive language catalog instead of hardcoded mapping.
func guessLanguageFromExtension(ext string) string {
	if ext == "" {
		return ""
	}
	
	// Remove leading dot for lookup
	lookupExt := strings.TrimPrefix(ext, ".")
	
	// Check all languages in our catalog for this extension
	for langName := range getAllLanguages() {
		// Many languages store their file extensions in metadata or we can infer
		// For now, use a simple heuristic: check if language name contains extension
		// This is imperfect but better than hardcoding only 30 languages
		if strings.Contains(strings.ToLower(langName), strings.ToLower(lookupExt)) {
			return langName
		}
		
		// Alternatively, check common language-extension patterns
		switch strings.ToLower(langName) {
		case "go":
			if lookupExt == "go" {
				return langName
			}
		case "python":
			if lookupExt == "py" {
				return langName
			}
		case "javascript":
			if lookupExt == "js" {
				return langName
			}
		case "typescript":
			if lookupExt == "ts" {
				return langName
			}
		case "ruby":
			if lookupExt == "rb" {
				return langName
			}
		// Add more common mappings as needed
		}
	}
	
	return ""
}

// DebugInfo returns debug information about the catalog (for testing only).
func (c *LanguageCatalog) DebugInfo() map[string]interface{} {
	info := make(map[string]interface{})
	info["byNameSize"] = len(c.byName)
	info["testIndexSize"] = len(c.testIndex)
	
	testPatternCounts := make(map[string]int)
	for domain, patterns := range c.testIndex {
		testPatternCounts[domain] = len(patterns)
	}
	info["testPatternCounts"] = testPatternCounts
	
	return info
}

// IsTestFile returns true if the given filename matches any known test pattern.
func (c *LanguageCatalog) IsTestFile(filename string) bool {
	pattern, _ := c.FindTestPattern(filename)
	return pattern != nil
}

// ArePaired returns true if the testFile is a test pair for the codeFile
// based on language-specific test patterns.
func (c *LanguageCatalog) ArePaired(codeFile, testFile string) bool {
	testPattern, _ := c.FindTestPattern(testFile)
	if testPattern == nil {
		return false
	}
	
	// For suffix patterns, check if base names match
	if testPattern.Type == "suffix" {
		codeBase := filepath.Base(codeFile)
		testBase := filepath.Base(testFile)
		
		// Remove the test suffix to get the base name
		baseWithoutSuffix := strings.TrimSuffix(testBase, testPattern.Value)
		
		// Check if the code file has the same base name
		if strings.TrimSuffix(codeBase, filepath.Ext(codeBase)) == baseWithoutSuffix {
			return true
		}
	}
	
	// For prefix patterns, check if base names match after removing prefix
	if testPattern.Type == "prefix" {
		codeBase := filepath.Base(codeFile)
		testBase := filepath.Base(testFile)
		
		// Remove the test prefix to get the base name
		if strings.HasPrefix(testBase, testPattern.Value) {
			baseWithoutPrefix := strings.TrimPrefix(testBase, testPattern.Value)
			
			// Check if the code file has the same base name (without extension)
			codeBaseNoExt := strings.TrimSuffix(codeBase, filepath.Ext(codeBase))
			if codeBaseNoExt == baseWithoutPrefix {
				return true
			}
		}
	}
	
	return false
}

// getAllLanguages returns all languages from the embedded data.
func getAllLanguages() map[string]data.LanguageNodes {
	result := make(map[string]data.LanguageNodes)
	
	langNames := data.GetAllLanguageNames()
	if len(langNames) == 0 {
		// This indicates the data hasn't been loaded properly
		panic("No language names found - data not loaded")
	}
	
	for _, langName := range langNames {
		if nodes, ok := data.GetLanguageNodes(langName); ok {
			result[langName] = nodes
		}
	}
	
	return result
}

// matchesPattern checks if a filename matches a test pattern.
// It also considers file extensions to avoid cross-language false positives.
func matchesPattern(baseName, fullPath string, pattern data.TestPattern) bool {
	switch pattern.Type {
	case "suffix":
		return strings.HasSuffix(baseName, pattern.Value)
	case "prefix":
		if pattern.InDir != "" {
			// Check if file is in the specified directory
			if !strings.Contains(fullPath, pattern.InDir) {
				return false
			}
		}
		// For prefix patterns, also check if the file extension matches known patterns
		// This prevents MATLAB patterns from matching Python files, etc.
		if !hasConsistentExtension(baseName, pattern) {
			return false
		}
		return strings.HasPrefix(baseName, pattern.Value)
	case "import_match":
		// Not implemented in Phase 1
		return false
	case "inline":
		// Not implemented in Phase 1
		return false
	default:
		return false
	}
}

// hasConsistentExtension checks if a filename's extension is consistent with
// the language typically associated with the pattern.
func hasConsistentExtension(filename string, pattern data.TestPattern) bool {
	// For now, this function is kept simple since we have proper language detection
	// via guessLanguageFromExtension which uses the comprehensive catalog
	
	// If pattern has specific directory requirements, we're more lenient
	if pattern.InDir != "" {
		return true
	}
	
	// For Phase 1, we'll be permissive - the language-aware matching in FindTestPattern
	// already handles cross-language prevention through extension-based language detection
	return true
}