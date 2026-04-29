package chunkers

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strings"
)

func (c *DiffChunker) extractAllSymbols(files []fileInfo) map[string]FileSymbols {
	result := make(map[string]FileSymbols)
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f.name))
		if ext == ".go" {
			result[f.name] = c.extractGoSymbols(f.name, f.diff)
		} else if grammar, ok := grammars[ext]; ok {
			result[f.name] = c.extractGrammarSymbols(f.diff, grammar)
		} else {
			result[f.name] = c.extractGenericSymbols(f.diff)
		}
	}
	return result
}

func (c *DiffChunker) extractGoSymbols(name, diff string) FileSymbols {
	symbols := FileSymbols{
		Definitions: make(map[string]bool),
		References:  make(map[string]bool),
	}
	var newCode strings.Builder
	lines := strings.Split(diff, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			newCode.WriteString(strings.TrimPrefix(line, "+") + "\n")
		}
	}
	dummyCode := "package dummy\n" + newCode.String()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, dummyCode, 0)
	if err != nil {
		return c.extractGenericSymbols(diff)
	}

	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			symbols.Definitions[x.Name.Name] = true
		case *ast.TypeSpec:
			symbols.Definitions[x.Name.Name] = true
		case *ast.CallExpr:
			if id, ok := x.Fun.(*ast.Ident); ok {
				symbols.References[id.Name] = true
			}
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
				symbols.References[sel.Sel.Name] = true
			}
		}
		return true
	})
	return symbols
}

func (c *DiffChunker) extractGrammarSymbols(diff string, grammar LanguageGrammar) FileSymbols {
	symbols := FileSymbols{
		Definitions: make(map[string]bool),
		References:  make(map[string]bool),
	}
	lines := strings.Split(diff, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			content := strings.TrimPrefix(line, "+")
			if matches := grammar.DefRegex.FindStringSubmatch(content); len(matches) > 1 {
				symbols.Definitions[matches[1]] = true
			}
			if matches := grammar.RefRegex.FindAllStringSubmatch(content, -1); len(matches) > 0 {
				for _, m := range matches {
					if len(m) > 1 {
						symbols.References[m[1]] = true
					}
				}
			}
		}
	}
	return symbols
}

func (c *DiffChunker) extractGenericSymbols(diff string) FileSymbols {
	symbols := FileSymbols{
		Definitions: make(map[string]bool),
		References:  make(map[string]bool),
	}
	wordRegex := regexp.MustCompile(`[a-zA-Z_][a-zA-Z0-9_]*`)
	lines := strings.Split(diff, "\n")
	for _, line := range lines {
		if !strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "-") {
			continue
		}
		words := wordRegex.FindAllString(line, -1)
		for _, w := range words {
			if len(w) > 3 && !isCommonWord(w) {
				symbols.References[w] = true
			}
		}
	}
	return symbols
}

func isCommonWord(word string) bool {
	common := map[string]bool{
		"func": true, "type": true, "var": true, "const": true,
		"import": true, "return": true, "if": true, "else": true,
		"for": true, "range": true, "switch": true, "case": true,
		"default": true, "break": true, "continue": true,
		"package": true, "struct": true, "interface": true,
		"go": true, "defer": true, "select": true,
	}
	return common[word]
}