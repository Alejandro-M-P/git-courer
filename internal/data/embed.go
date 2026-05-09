package data

import (
	_ "embed"
	"encoding/json"
	"log"
	"sync"
)

//go:embed languages.json
var languagesJSON []byte

// LanguageNodes defines AST node types for a language.
type LanguageNodes struct {
	Functions []string
	Types     []string
}

type languagesFile struct {
	Languages map[string]jsonNodeEntry `json:"languages"`
}

type jsonNodeEntry struct {
	Functions []string `json:"functions"`
	Types     []string `json:"types"`
}

var (
	mu     sync.RWMutex
	loaded map[string]LanguageNodes
)

func init() {
	if err := LoadLanguages(); err != nil {
		log.Fatalf("internal/data: failed to load languages.json: %v", err)
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
