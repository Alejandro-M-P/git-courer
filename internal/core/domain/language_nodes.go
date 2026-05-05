package domain

// LanguageNodes defines AST node types for a language.
type LanguageNodes struct {
	Functions []string
	Types     []string
}

// GetLanguageNodes returns the node types for a given language name.
// The second return value is false if the language is not mapped.
func GetLanguageNodes(lang string) (LanguageNodes, bool) {
	n, ok := languageNodeMapping[lang]
	return n, ok
}

// languageNodeMapping maps language names to their AST node types.
// Supports ALL languages provided by gotreesitter (206+ grammars).
var languageNodeMapping = map[string]LanguageNodes{
	"Go": {
		Functions: []string{"function_declaration", "method_declaration"},
		Types:     []string{"type_declaration", "type_spec"},
	},
	"Python": {
		Functions: []string{"function_definition", "async_function_definition"},
		Types:     []string{"class_definition"},
	},
	"JavaScript": {
		Functions: []string{"function_declaration", "arrow_function", "method_definition"},
		Types:     []string{"class_declaration"},
	},
	"TypeScript": {
		Functions: []string{"function_declaration", "arrow_function", "method_definition"},
		Types:     []string{"class_declaration", "interface_declaration", "type_alias_declaration"},
	},
	"Rust": {
		Functions: []string{"function_item"},
		Types:     []string{"struct_item", "trait_item", "impl_item", "enum_item"},
	},
	"Java": {
		Functions: []string{"method_declaration", "constructor_declaration"},
		Types:     []string{"class_declaration", "interface_declaration", "enum_declaration"},
	},
	"C#": {
		Functions: []string{"method_declaration", "constructor_declaration"},
		Types:     []string{"class_declaration", "interface_declaration"},
	},
	"C++": {
		Functions: []string{"function_definition", "method_definition"},
		Types:     []string{"class_definition", "struct_definition"},
	},
	"PHP": {
		Functions: []string{"function_definition", "method_definition"},
		Types:     []string{"class_definition", "interface_declaration"},
	},
	"Ruby": {
		Functions: []string{"method_definition"},
		Types:     []string{"class_definition", "module_definition"},
	},
	"Swift": {
		Functions: []string{"function_declaration", "method_declaration"},
		Types:     []string{"class_declaration", "struct_declaration", "enum_declaration"},
	},
	"Kotlin": {
		Functions: []string{"function_declaration", "method_declaration"},
		Types:     []string{"class_declaration", "interface_declaration"},
	},
	"Dart": {
		Functions: []string{"function_declaration", "method_declaration"},
		Types:     []string{"class_declaration"},
	},
	// Add more languages as needed for gotreesitter support
}
