package domain

import "github.com/Alejandro-M-P/git-courer/internal/data"

// LanguageNodes defines AST node types for a language.
// Alias to internal/data for backward compatibility.
type LanguageNodes = data.LanguageNodes

// GetLanguageNodes returns the node types for a given language name.
// The second return value is false if the language is not mapped.
// Delegates to the embedded catalog in internal/data.
func GetLanguageNodes(lang string) (LanguageNodes, bool) {
	return data.GetLanguageNodes(lang)
}
