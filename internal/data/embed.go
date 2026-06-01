package data

import (
	_ "embed"
	"encoding/json"

	"sync"
)

//go:embed languages.json
var languagesJSON []byte

// TestPattern defines a test file pattern for a language.
type TestPattern struct {
	Type        string `json:"type"`                   // "suffix" | "prefix" | "import_match" | "inline"
	Value       string `json:"value"`                  // pattern string (e.g., "_test.go", "test_")
	InDir       string `json:"in_dir,omitempty"`       // optional: only match in this directory
	SamePackage bool   `json:"same_package,omitempty"` // suffix only: enforce same package
	Fallback    bool   `json:"fallback,omitempty"`     // import_match only: use as fallback
}

// ControlFlowCategory groups control-flow AST node types by semantic category.
type ControlFlowCategory struct {
	Branch []string `json:"branch,omitempty"`
	Loop   []string `json:"loop,omitempty"`
	Return []string `json:"return,omitempty"`
	Error  []string `json:"error,omitempty"`
}

// LanguageNodes defines AST node types for a language.
type LanguageNodes struct {
	Functions    []string            `json:"functions"`
	Types        []string            `json:"types"`
	TestPatterns []TestPattern       `json:"test_patterns,omitempty"`
	Visibility   string              `json:"visibility,omitempty"` // "capital", "underscore", "public_keyword", or empty
	ControlFlow  ControlFlowCategory `json:"control_flow,omitempty"`
}

type languagesFile struct {
	Languages map[string]jsonNodeEntry `json:"languages"`
}

type jsonNodeEntry struct {
	Functions    []string            `json:"functions"`
	Types        []string            `json:"types"`
	TestPatterns []TestPattern       `json:"test_patterns"`
	Visibility   string              `json:"visibility"`
	ControlFlow  ControlFlowCategory `json:"control_flow,omitempty"`
}

var (
	mu            sync.RWMutex
	loaded        map[string]LanguageNodes
	languagesOnce sync.Once
)

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
			Functions:    entry.Functions,
			Types:        entry.Types,
			TestPatterns: entry.TestPatterns,
			Visibility:   entry.Visibility,
			ControlFlow:  entry.ControlFlow,
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
	languagesOnce.Do(func() {
		_ = LoadLanguages()
	})
	mu.RLock()
	n, ok := loaded[lang]
	mu.RUnlock()
	return n, ok
}

// GetAllLanguageNames returns a slice of all available language names.
func GetAllLanguageNames() []string {
	languagesOnce.Do(func() {
		_ = LoadLanguages()
	})
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(loaded))
	for name := range loaded {
		names = append(names, name)
	}
	return names
}
