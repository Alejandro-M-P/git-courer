package domain

import (
	"path/filepath"
	"strings"

	"github.com/blak0p/git-courer/internal/data"
)

// ExtDetector detects a language from a file extension (without the leading dot).
// Returns the language name (e.g. "go", "python") or nil if unknown.
// This is injected at construction to avoid domain depending on infrastructure.
type ExtDetector func(ext string) *string

// LanguageEntry combines domain catalog info with grammar availability.
type LanguageEntry struct {
	DomainName string // e.g. "Go"
	Name       string // language name (e.g. "go", "c_sharp")
	Nodes      data.LanguageNodes
	HasGrammar bool
}

// LanguageCatalog provides efficient access to language node definitions
// and test pattern matching for code-test file pairing.
type LanguageCatalog struct {
	byName        map[string]data.LanguageNodes // full node types
	testIndex     map[string][]data.TestPattern // indexed by domain name
	extDetector   ExtDetector                   // optional: detects language from extension
	domainNameMap map[string]string             // kreuzberg name → domain name mapping
}

// NewLanguageCatalog creates a new LanguageCatalog from pre-built language maps.
// byName maps domain names (e.g. "Go") to their node definitions.
// testIndex maps domain names to their test patterns.
// extDetector is optional — pass nil to disable extension-based language detection.
func NewLanguageCatalog(byName map[string]data.LanguageNodes, testIndex map[string][]data.TestPattern, extDetector ExtDetector) *LanguageCatalog {
	if byName == nil {
		byName = make(map[string]data.LanguageNodes)
	}
	if testIndex == nil {
		testIndex = make(map[string][]data.TestPattern)
	}

	return &LanguageCatalog{
		byName:        byName,
		testIndex:     testIndex,
		extDetector:   extDetector,
		domainNameMap: defaultDomainNameMap(),
	}
}

// defaultDomainNameMap maps kreuzberg lowercase language names to domain names.
func defaultDomainNameMap() map[string]string {
	return map[string]string{
		"c_sharp":    "C#",
		"csharp":     "C#",
		"fsharp":     "F#",
		"cpp":        "C++",
		"c":          "C",
		"go":         "Go",
		"gomod":      "Go",
		"d":          "D",
		"javascript": "JavaScript",
		"typescript": "TypeScript",
		"tsx":        "TypeScript",
		"python":     "Python",
		"rust":       "Rust",
		"ruby":       "Ruby",
		"java":       "Java",
		"kotlin":     "Kotlin",
		"swift":      "Swift",
		"dart":       "Dart",
		"php":        "PHP",
		"markdown":   "Markdown",
	}
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
	ext := filepath.Ext(filename)
	likelyLanguage := c.guessLanguageFromExtension(ext)

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
func (c *LanguageCatalog) guessLanguageFromExtension(ext string) string {
	if ext == "" {
		return ""
	}

	lookupExt := strings.TrimPrefix(ext, ".")

	// Check all languages in our catalog for this extension
	for langName := range c.byName {
		if strings.Contains(strings.ToLower(langName), strings.ToLower(lookupExt)) {
			return langName
		}

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

// ExtensionToLanguage maps a file extension to a LanguageEntry using the
// injected extension detector (if set) and the language catalog.
// It returns false for empty or unknown extensions, or when no detector is set.
func (c *LanguageCatalog) ExtensionToLanguage(ext string) (LanguageEntry, bool) {
	if ext == "" {
		return LanguageEntry{}, false
	}

	if c.extDetector == nil {
		return LanguageEntry{}, false
	}

	extKey := strings.TrimPrefix(ext, ".")
	if extKey == "" {
		return LanguageEntry{}, false
	}

	langPtr := c.extDetector(extKey)
	if langPtr == nil {
		return LanguageEntry{}, false
	}
	kreuzbergName := *langPtr

	// Map to domain name using the injected domain name map
	domainName := c.domainNameFromKreuzberg(kreuzbergName)
	nodes, nodesFound := c.ByName(domainName) // Check if grammar exists in catalog
	hasGrammar := nodesFound

	return LanguageEntry{
		DomainName: domainName,
		Name:       kreuzbergName,
		Nodes:      nodes,
		HasGrammar: hasGrammar,
	}, true
}

// domainNameFromKreuzberg converts a kreuzberg language name (lowercase, e.g. "python")
// to the corresponding domain name used in the language catalog (e.g. "Python").
func (c *LanguageCatalog) domainNameFromKreuzberg(name string) string {
	if name == "" {
		return ""
	}
	if domain, ok := c.domainNameMap[name]; ok {
		return domain
	}
	// Default: capitalize first letter
	return strings.ToUpper(name[:1]) + name[1:]
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
		baseWithoutSuffix := strings.TrimSuffix(testBase, testPattern.Value)
		if strings.TrimSuffix(codeBase, filepath.Ext(codeBase)) == baseWithoutSuffix {
			return true
		}
	}

	// For prefix patterns, check if base names match after removing prefix
	if testPattern.Type == "prefix" {
		codeBase := filepath.Base(codeFile)
		testBase := filepath.Base(testFile)
		if strings.HasPrefix(testBase, testPattern.Value) {
			baseWithoutPrefix := strings.TrimPrefix(testBase, testPattern.Value)
			codeBaseNoExt := strings.TrimSuffix(codeBase, filepath.Ext(codeBase))
			if codeBaseNoExt == baseWithoutPrefix {
				return true
			}
		}
	}

	return false
}

// matchesPattern checks if a filename matches a test pattern.
func matchesPattern(baseName, fullPath string, pattern data.TestPattern) bool {
	switch pattern.Type {
	case "suffix":
		return strings.HasSuffix(baseName, pattern.Value)
	case "prefix":
		if pattern.InDir != "" {
			if !strings.Contains(fullPath, pattern.InDir) {
				return false
			}
		}
		if !hasConsistentExtension(baseName, pattern) {
			return false
		}
		return strings.HasPrefix(baseName, pattern.Value)
	case "import_match":
		return false
	case "inline":
		return false
	default:
		return false
	}
}

// hasConsistentExtension checks if a filename's extension is consistent with
// the language typically associated with the pattern.
func hasConsistentExtension(filename string, pattern data.TestPattern) bool {
	if pattern.InDir != "" {
		return true
	}
	return true
}
