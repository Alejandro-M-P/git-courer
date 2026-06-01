package classifier

import (
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
)

// functionInfo holds an extracted function declaration with its position.
type functionInfo struct {
	Name     string // function name
	File     string // source file path
	BodyHash string // SHA-256 hash of the normalized function body
}

// normalizeBody parses a Go source file, finds the named function, and
// returns its body with all identifiers replaced by positional names (v1, v2, v3...).
// Comments are stripped. Returns empty string if the function is not found
// or the source cannot be parsed.
func normalizeBody(src string, funcName string) string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.AllErrors|parser.ParseComments)
	if err != nil {
		return ""
	}

	// Find the target function
	var fn *ast.FuncDecl
	for _, decl := range f.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == funcName {
			fn = fd
			break
		}
	}
	if fn == nil || fn.Body == nil {
		return ""
	}

	// Strip comments from the entire file
	ast.Inspect(f, func(n ast.Node) bool {
		if cg, ok := n.(*ast.CommentGroup); ok {
			cg.List = nil
			return false
		}
		return true
	})

	// Walk the function body (including parameters and receiver) and
	// replace all non-builtin identifiers with positional names.
	identMap := map[string]string{}
	counter := 0
	normalizeIdents(fn, &counter, identMap)

	// Format the modified AST to get canonical output
	var buf strings.Builder
	if err := format.Node(&buf, fset, fn.Body); err != nil {
		// If format fails, fall back gracefully
		return ""
	}

	return strings.TrimSpace(buf.String())
}

// normalizeIdents walks the AST of a function declaration and replaces
// all non-builtin identifiers with positional names (v1, v2, etc.).
// It handles receiver, parameters, results, and body identifiers.
// Selector expressions (pkg.Func) preserve both parts.
func normalizeIdents(fn *ast.FuncDecl, counter *int, identMap map[string]string) {
	// Process receiver
	if fn.Recv != nil {
		for _, field := range fn.Recv.List {
			normalizeFieldIdents(field, counter, identMap)
		}
	}

	// Process parameters
	if fn.Type.Params != nil {
		for _, field := range fn.Type.Params.List {
			normalizeFieldIdents(field, counter, identMap)
		}
	}

	// Process results
	if fn.Type.Results != nil {
		for _, field := range fn.Type.Results.List {
			normalizeFieldIdents(field, counter, identMap)
		}
	}

	// Process body
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.Ident:
			if !isBuiltin(v.Name) {
				mapped, ok := identMap[v.Name]
				if !ok {
					*counter++
					mapped = fmt.Sprintf("v%d", *counter)
					identMap[v.Name] = mapped
				}
				v.Name = mapped
			}
		case *ast.SelectorExpr:
			// Keep the X (package) and Sel (method) names as-is
			// but still walk into X in case it has nested structures
			// We don't rename idents inside selector expressions
			// because pkg.Func should remain as-is
			return false
		}
		return true
	})
}

// normalizeFieldIdents replaces identifiers in a field list (params/results/receiver)
// with their positional names, preserving type identifiers.
func normalizeFieldIdents(field *ast.Field, counter *int, identMap map[string]string) {
	for _, name := range field.Names {
		if !isBuiltin(name.Name) {
			mapped, ok := identMap[name.Name]
			if !ok {
				*counter++
				mapped = fmt.Sprintf("v%d", *counter)
				identMap[name.Name] = mapped
			}
			name.Name = mapped
		}
	}
	// Type identifiers (int, string, custom types) - we normalize these too
	// unless they're part of a selector expression (pkg.Type)
	normalizeTypeExpr(field.Type, counter, identMap)
}

// normalizeTypeExpr replaces identifiers within type expressions.
func normalizeTypeExpr(expr ast.Expr, counter *int, identMap map[string]string) {
	switch v := expr.(type) {
	case *ast.Ident:
		if !isBuiltin(v.Name) {
			mapped, ok := identMap[v.Name]
			if !ok {
				*counter++
				mapped = fmt.Sprintf("v%d", *counter)
				identMap[v.Name] = mapped
			}
			v.Name = mapped
		}
	case *ast.SelectorExpr:
		// pkg.Type — keep both parts, don't normalize
		return
	case *ast.StarExpr:
		normalizeTypeExpr(v.X, counter, identMap)
	case *ast.ArrayType:
		normalizeTypeExpr(v.Elt, counter, identMap)
	case *ast.MapType:
		normalizeTypeExpr(v.Key, counter, identMap)
		normalizeTypeExpr(v.Value, counter, identMap)
	case *ast.ChanType:
		normalizeTypeExpr(v.Value, counter, identMap)
	case *ast.FuncType:
		if v.Params != nil {
			for _, field := range v.Params.List {
				normalizeFieldIdents(field, counter, identMap)
			}
		}
		if v.Results != nil {
			for _, field := range v.Results.List {
				normalizeFieldIdents(field, counter, identMap)
			}
		}
	}
}

// isBuiltin returns true for Go built-in identifiers that should not be
// replaced during normalization.
func isBuiltin(name string) bool {
	switch name {
	case "true", "false", "nil",
		"int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64", "complex64", "complex128",
		"string", "byte", "rune", "bool", "error",
		"len", "cap", "make", "new", "append", "copy",
		"delete", "close", "panic", "recover", "print",
		"println", "complex", "real", "imag",
		"iota":
		return true
	}
	return false
}

// hashFunctionBody normalizes the body of the specified function in src
// and returns its SHA-256 hash. Returns empty string if the function
// cannot be found or the source cannot be parsed.
func hashFunctionBody(src string, funcName string) string {
	normalized := normalizeBody(src, funcName)
	if normalized == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("%x", hash)
}

// extractFunctions parses a Go source file and returns all top-level
// function declarations with their normalized body hashes.
// Returns nil if the source cannot be parsed.
func extractFunctions(src string, filePath string) []functionInfo {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.AllErrors|parser.ParseComments)
	if err != nil {
		return nil
	}

	var funcs []functionInfo
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}

		bodyHash := hashFunctionBody(src, fd.Name.Name)
		if bodyHash == "" {
			continue
		}

		funcs = append(funcs, functionInfo{
			Name:     fd.Name.Name,
			File:     filePath,
			BodyHash: bodyHash,
		})
	}

	return funcs
}

// detectRefactorByASTHash uses AST-based identity checking to detect
// function renames and moves. It compares before and after function
// declarations across files, hashing their normalized bodies.
//
// Returns ("refactor", 1.0) if a rename or move is detected,
// ("", 0.0) otherwise (including on parse failure).
func (c *Classifier) detectRefactorByASTHash(
	files []string,
	beforeSrc map[string]string,
	afterSrc map[string]string,
) (string, float64) {
	// Extract all functions from before and after sources
	var beforeFuncs, afterFuncs []functionInfo
	for file, src := range beforeSrc {
		funcs := extractFunctions(src, file)
		beforeFuncs = append(beforeFuncs, funcs...)
	}
	for file, src := range afterSrc {
		funcs := extractFunctions(src, file)
		afterFuncs = append(afterFuncs, funcs...)
	}

	// If we couldn't extract any functions, pass through gracefully
	if len(beforeFuncs) == 0 && len(afterFuncs) == 0 {
		return "", 0.0
	}

	// Build maps keyed by body hash
	type funcKey struct {
		Name string
		File string
	}

	beforeByHash := map[string][]funcKey{}
	for _, f := range beforeFuncs {
		beforeByHash[f.BodyHash] = append(beforeByHash[f.BodyHash], funcKey{Name: f.Name, File: f.File})
	}

	afterByHash := map[string][]funcKey{}
	for _, f := range afterFuncs {
		afterByHash[f.BodyHash] = append(afterByHash[f.BodyHash], funcKey{Name: f.Name, File: f.File})
	}

	// Compare: for each hash that appears in both before and after
	for hash, beforeKeys := range beforeByHash {
		afterKeys, exists := afterByHash[hash]
		if !exists {
			continue // No matching hash → logic change, not a rename/move
		}

		// Find keys that disappeared (existed before, but not now)
		var disappeared []funcKey
		for _, bk := range beforeKeys {
			found := false
			for _, ak := range afterKeys {
				if bk.Name == ak.Name && bk.File == ak.File {
					found = true
					break
				}
			}
			if !found {
				disappeared = append(disappeared, bk)
			}
		}

		// Find keys that appeared (exist now, but not before)
		var appeared []funcKey
		for _, ak := range afterKeys {
			found := false
			for _, bk := range beforeKeys {
				if bk.Name == ak.Name && bk.File == ak.File {
					found = true
					break
				}
			}
			if !found {
				appeared = append(appeared, ak)
			}
		}

		// If a function disappeared and a new function with the same body hash appeared,
		// then it is a rename or move!
		if len(disappeared) > 0 && len(appeared) > 0 {
			return "refactor", 1.0
		}
	}

	// No rename or move detected — pass through to next pillar
	return "", 0.0
}
