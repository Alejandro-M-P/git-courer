package data

import (
	_ "embed"
	"encoding/json"
	"log"
	"sync"
)

//go:embed languages.json
var languagesJSON []byte

//go:embed models.json
var modelsJSON []byte

// TestPattern defines a test file pattern for a language.
type TestPattern struct {
	Type        string `json:"type"`         // "suffix" | "prefix" | "import_match" | "inline"
	Value       string `json:"value"`        // pattern string (e.g., "_test.go", "test_")
	InDir       string `json:"in_dir,omitempty"`     // optional: only match in this directory
	SamePackage bool   `json:"same_package,omitempty"` // suffix only: enforce same package
	Fallback    bool   `json:"fallback,omitempty"`     // import_match only: use as fallback
}

// LanguageNodes defines AST node types for a language.
type LanguageNodes struct {
	Functions    []string      `json:"functions"`
	Types        []string      `json:"types"`
	TestPatterns []TestPattern `json:"test_patterns,omitempty"`  // NEW — additive, backwards compat
}

type languagesFile struct {
	Languages map[string]jsonNodeEntry `json:"languages"`
}

type jsonNodeEntry struct {
	Functions    []string      `json:"functions"`
	Types        []string      `json:"types"`
	TestPatterns []TestPattern `json:"test_patterns"`  // NEW
}

type modelsFile struct {
	Models map[string]int `json:"models"`
}

var (
	mu        sync.RWMutex
	loaded    map[string]LanguageNodes

	modelsMu   sync.RWMutex
	modelsData map[string]int
)

func init() {
	if err := LoadLanguages(); err != nil {
		log.Fatalf("internal/data: failed to load languages.json: %v", err)
	}
	if err := LoadModels(); err != nil {
		log.Fatalf("internal/data: failed to load models.json: %v", err)
	}
}

// LoadLanguages loads language nodes from the embedded languages.json.
func LoadLanguages() error {
	return LoadLanguagesFromBytes(languagesJSON)
}

// LoadLanguagesFromBytes loads language nodes from JSON data.
// If the data is invalid, the previous state is preserved and an error is returned.
func LoadLanguagesFromBytes(data []byte) error {
	var file languagesFile
	if err := json.Unmarshal(data, &file); err != nil {
		return err
	}

	newLoaded := make(map[string]LanguageNodes, len(file.Languages))
	for name, entry := range file.Languages {
		newLoaded[name] = LanguageNodes{
			Functions: entry.Functions,
			Types:     entry.Types,
			TestPatterns: entry.TestPatterns,
		}
	}

	mu.Lock()
	loaded = newLoaded
	mu.Unlock()

	return nil
}

// GetLanguageNodes returns the node types for a given language name.
// The second return value is false if the language is not mapped.
func GetLanguageNodes(lang string) (LanguageNodes, bool) {
	mu.RLock()
	n, ok := loaded[lang]
	mu.RUnlock()
	return n, ok
}

// GetAllLanguageNames returns a slice of all available language names.
func GetAllLanguageNames() []string {
	mu.RLock()
	defer mu.RUnlock()
	
	names := make([]string, 0, len(loaded))
	for name := range loaded {
		names = append(names, name)
	}
	return names
}

// LoadModels loads model context windows from the embedded models.json.
func LoadModels() error {
	return LoadModelsFromBytes(modelsJSON)
}

// LoadModelsFromBytes loads model context windows from JSON data.
// If the data is invalid, the previous state is preserved and an error is returned.
func LoadModelsFromBytes(data []byte) error {
	var file modelsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return err
	}

	newData := make(map[string]int, len(file.Models))
	for name, entry := range file.Models {
		newData[name] = entry
	}

	modelsMu.Lock()
	modelsData = newData
	modelsMu.Unlock()

	return nil
}

// GetModelContext returns the context window for a given model name.
// The second return value is false if the model is not mapped.
func GetModelContext(name string) (int, bool) {
	modelsMu.RLock()
	entry, ok := modelsData[name]
	modelsMu.RUnlock()
	return entry, ok
}

// GetAllModelNames returns a slice of all available model names.
func GetAllModelNames() []string {
	modelsMu.RLock()
	defer modelsMu.RUnlock()

	names := make([]string, 0, len(modelsData))
	for name := range modelsData {
		names = append(names, name)
	}
	return names
}
