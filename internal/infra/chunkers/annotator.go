package chunkers

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	gotreesitter "github.com/odvcencio/gotreesitter"
	tsgrammars "github.com/odvcencio/gotreesitter/grammars"
)

var _ ports.ChunkAnnotator = (*ASTAnnotator)(nil)

// ASTAnnotator implements ports.ChunkAnnotator using gotreesitter AST analysis.
// It provides semantic annotation (function/type changes) per file.
//
// The caller should invoke Annotate once per file, with chunk.Files containing
// the file being annotated. Results are appended to chunk.AnnotatedDiff.
type ASTAnnotator struct{}

// NewASTAnnotator creates a new AST annotator.
func NewASTAnnotator() *ASTAnnotator {
	return &ASTAnnotator{}
}

// entity holds extracted function/type information from an AST.
type entity struct {
	Name      string // function or type name
	Signature string // full declaration text for signature comparison
	Line      int    // 1-indexed start line
	Kind      string // "func" or "type"
}

// grammarsNameToDomain maps gotreesitter grammars names (lowercase) to
// domain language names used by domain.GetLanguageNodes.
var grammarsNameToDomain = map[string]string{
	"go": "Go", "python": "Python", "javascript": "JavaScript",
	"typescript": "TypeScript", "rust": "Rust", "java": "Java",
	"c_sharp": "C#", "cpp": "C++", "php": "PHP",
	"ruby": "Ruby", "swift": "Swift", "kotlin": "Kotlin", "dart": "Dart",
}

// langFactories maps grammars names to their language constructors.
var langFactories = map[string]func() *gotreesitter.Language{
	"go":      tsgrammars.GoLanguage,
	"python":  tsgrammars.PythonLanguage,
	"javascript": tsgrammars.JavascriptLanguage,
	"typescript": tsgrammars.TypescriptLanguage,
	"rust":    tsgrammars.RustLanguage,
	"java":    tsgrammars.JavaLanguage,
	"c_sharp": tsgrammars.CSharpLanguage,
	"cpp":     tsgrammars.CppLanguage,
	"php":     tsgrammars.PhpLanguage,
	"ruby":    tsgrammars.RubyLanguage,
	"swift":   tsgrammars.SwiftLanguage,
	"kotlin":  tsgrammars.KotlinLanguage,
	"dart":    tsgrammars.DartLanguage,
}

// nonCodeExtensions maps file extensions to non-code category labels.
var nonCodeExtensions = map[string]string{
	".json": "CONFIG", ".yaml": "CONFIG", ".yml": "CONFIG",
	".toml": "CONFIG", ".ini": "CONFIG", ".cfg": "CONFIG", ".env": "CONFIG",
	".properties": "CONFIG", ".conf": "CONFIG",
	".md": "DOCS", ".txt": "DOCS", ".rst": "DOCS", ".adoc": "DOCS",
	".mdx": "DOCS", ".markdown": "DOCS",
}

// depsFilenames are filenames that indicate dependency manifests.
var depsFilenames = map[string]bool{
	"go.mod": true, "go.sum": true,
	"package.json": true, "package-lock.json": true, "yarn.lock": true,
	"pnpm-lock.yaml": true,
	"Cargo.toml": true, "Cargo.lock": true,
	"requirements.txt": true, "Pipfile": true, "Pipfile.lock": true,
	"poetry.lock": true, "setup.cfg": true, "setup.py": true, "pyproject.toml": true,
	"Gemfile": true, "Gemfile.lock": true,
	"composer.json": true, "composer.lock": true,
	"pom.xml": true, "build.gradle": true, "build.gradle.kts": true,
}

// ciIndicators are path segments or filenames that indicate CI/automation files.
var ciIndicators = []string{
	"Makefile", "Dockerfile", "Jenkinsfile",
	".gitlab-ci.yml", ".travis.yml", "docker-compose.yml", "docker-compose.yaml",
	".github/", ".circleci/", ".drone.yml",
}

// Annotate processes before/after content for a single file and appends
// semantic labels to chunk.AnnotatedDiff.
//
// Precondition: chunk.Files should contain the file being annotated
// (typically as chunk.Files[0]). The method detects the language, parses
// both versions, extracts function/type entities, compares them, formats
// the result, and appends to chunk.AnnotatedDiff.
//
// Parse errors are treated as soft failures: the file is skipped and
// processing continues. Unknown or unmapped languages fall back to
// non-code category labels where applicable.
func (a *ASTAnnotator) Annotate(chunk *domain.DiffChunk, filename string, before, after []byte) error {
	if filename == "" {
		return nil
	}

	// AST annotation path.
	if a.tryAnnotateAST(chunk, filename, before, after) {
		return nil
	}

	// Non-code category fallback.
	cat := categoryLabel(filename)
	if cat != "" {
		appendAnnotation(chunk, formatSingleLabel(filename, cat))
	}

	return nil
}

func (a *ASTAnnotator) tryAnnotateAST(chunk *domain.DiffChunk, filename string, before, after []byte) bool {
	gName := detectLanguage(filename)
	domainLang := grammarsNameToDomain[gName]
	if domainLang == "" {
		return false
	}

	nodes, ok := domain.GetLanguageNodes(domainLang)
	if !ok {
		return false
	}

	langFn := langFactories[gName]
	if langFn == nil {
		return false
	}
	grammar := langFn()

	beforeEnts := a.safeParse(grammar, before, nodes)
	afterEnts := a.safeParse(grammar, after, nodes)

	labels := matchEntities(beforeEnts, afterEnts, nodes, domainLang, filename)
	if len(labels) > 0 {
		appendAnnotation(chunk, formatLabels(filename, labels))
	}

	return true
}

func (a *ASTAnnotator) safeParse(grammar *gotreesitter.Language, src []byte, nodes domain.LanguageNodes) []entity {
	if len(src) == 0 {
		return nil
	}
	ents, err := parseAndExtract(grammar, src, nodes)
	if err != nil {
		return nil
	}
	return ents
}

// detectLanguage detects the programming language for a filename using
// gotreesitter's built-in extension registry.
func detectLanguage(filename string) string {
	entry := tsgrammars.DetectLanguage(filename)
	if entry == nil {
		return ""
	}
	return entry.Name
}

// parseAndExtract parses source with gotreesitter and extracts function/type entities.
func parseAndExtract(lang *gotreesitter.Language, src []byte, nodes domain.LanguageNodes) ([]entity, error) {
	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(src)
	if err != nil {
		return nil, err
	}
	defer tree.Release()

	root := tree.RootNode()
	if root == nil {
		return nil, fmt.Errorf("empty AST root")
	}

	funcSet := make(map[string]bool, len(nodes.Functions))
	for _, f := range nodes.Functions {
		funcSet[f] = true
	}
	typeSet := make(map[string]bool, len(nodes.Types))
	for _, t := range nodes.Types {
		typeSet[t] = true
	}

	var entities []entity
	walkTree(root, lang, src, funcSet, typeSet, &entities, nil)
	return entities, nil
}

// ancestorTypeSet contains node types that suppress function extraction.
// When a function-like node (method_elem, identifier in func context) is under
// one of these ancestors, it is part of a type declaration and should NOT be
// extracted as a standalone function entity.
var ancestorTypeSet = map[string]bool{
	"interface_type": true,
	"type_spec":      true,
}

func walkTree(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte, funcSet, typeSet map[string]bool, result *[]entity, ancestors []string) {
	nodeType := node.Type(lang)

	if funcSet[nodeType] || typeSet[nodeType] {
		// Bug #6 fix: Skip function-like nodes under type declarations.
		// Interface method declarations (method_elem) and identifiers under
		// type_spec/interface_type are NOT standalone functions — they belong
		// to the type definition and should not produce NEW_FUNC labels.
		if funcSet[nodeType] && !typeSet[nodeType] {
			underTypeDecl := false
			for _, a := range ancestors {
				if ancestorTypeSet[a] {
					underTypeDecl = true
					break
				}
			}
			if underTypeDecl {
				// Skip — this func-like node is part of a type declaration
				// (e.g., interface method). Don't descend further.
				return
			}
		}

		kind := "func"
		if typeSet[nodeType] {
			kind = "type"
		}

		name := extractName(node, lang, src)
		if name != "" {
			*result = append(*result, entity{
				Name:      name,
				Signature: signatureText(node, src),
				Line:      int(node.StartPoint().Row) + 1, // 0-indexed → 1-indexed
				Kind:      kind,
			})
		}
	}

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child != nil {
			walkTree(child, lang, src, funcSet, typeSet, result, append(ancestors, nodeType))
		}
	}
}

// extractName returns the entity name using tree-sitter's universal "name" field.
func extractName(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte) string {
	if nameNode := node.ChildByFieldName("name", lang); nameNode != nil {
		return nameNode.Text(src)
	}
	return ""
}

// signatureText returns declaration-only text (stops at the body block `{`).
// This allows detecting MOD_BODY vs MOD_SIG correctly — same declaration
// with different body is MOD_BODY, not MOD_SIG.
func signatureText(node *gotreesitter.Node, src []byte) string {
	full := node.Text(src)
	// Cut at opening brace that starts a block body.
	braceIdx := strings.Index(full, "{")
	if braceIdx < 0 {
		return full
	}
	return strings.TrimSpace(full[:braceIdx])
}

// matchEntities compares before/after entities and generates domain labels.
func matchEntities(before, after []entity, nodes domain.LanguageNodes, domainLang, filename string) []domain.Label {
	beforeMap := buildEntityMap(before)
	afterMap := buildEntityMap(after)

	var labels []domain.Label

	// Only in after → new.
	for name, aEnt := range afterMap {
		if _, exists := beforeMap[name]; !exists {
			labels = append(labels, domain.Label{
				Type:     labelForKind(aEnt.Kind, true),
				Name:     name,
				Line:     aEnt.Line,
				Breaking: false, // new additions are NOT breaking
			})
		}
	}

	// In both → modified or deleted.
	for name, bEnt := range beforeMap {
		if aEnt, exists := afterMap[name]; exists {
			isFunc := bEnt.Kind == "func"
			if bEnt.Signature == aEnt.Signature {
				labels = append(labels, domain.Label{
					Type:     modLabel(isFunc),
					Name:     name,
					Line:     aEnt.Line,
					Breaking: false, // MOD_BODY/MOD_TYPE = not breaking per spec
				})
			} else {
				lt := domain.MOD_TYPE
				if isFunc {
					lt = domain.MOD_SIG
				}
				labels = append(labels, domain.Label{
					Type:     lt,
					Name:     name,
					Line:     aEnt.Line,
					Breaking: isFunc && isPublicEntity(aEnt, domainLang),
				})
			}
		} else {
			// Only in before → deleted.
			lt := labelForKind(bEnt.Kind, false)
			labels = append(labels, domain.Label{
				Type:     lt,
				Name:     name,
				Line:     bEnt.Line,
				Breaking: isPublicEntity(bEnt, domainLang),
			})
		}
	}

	// Fill file field after generation.
	for i := range labels {
		labels[i].File = filename
	}

	return labels
}

func buildEntityMap(ents []entity) map[string]entity {
	m := make(map[string]entity, len(ents))
	for _, e := range ents {
		m[e.Name] = e
	}
	return m
}

func labelForKind(kind string, isNew bool) domain.LabelType {
	switch {
	case kind == "func" && isNew:
		return domain.NEW_FUNC
	case kind == "func" && !isNew:
		return domain.DELETED_FUNC
	case kind == "type" && isNew:
		return domain.NEW_TYPE
	default:
		return domain.DELETED_TYPE
	}
}

func modLabel(isFunc bool) domain.LabelType {
	if isFunc {
		return domain.MOD_BODY
	}
	return domain.MOD_TYPE
}

// isPublicEntity determines if an entity is publicly visible per language rules.
func isPublicEntity(ent entity, lang string) bool {
	if ent.Name == "" {
		return false
	}

	switch lang {
	case "Go":
		return ent.Name[0] >= 'A' && ent.Name[0] <= 'Z'
	case "Python", "JavaScript", "TypeScript", "Dart":
		return ent.Name[0] != '_'
	case "Rust":
		return strings.HasPrefix(strings.TrimSpace(ent.Signature), "pub ")
	case "Java", "C#", "PHP":
		return strings.Contains(ent.Signature, "public ")
	case "Swift":
		return strings.Contains(ent.Signature, "public ") || strings.Contains(ent.Signature, "open ")
	case "Kotlin":
		// Kotlin: public by default; private when "private" keyword present.
		return !strings.Contains(ent.Signature, "private ")
	case "C++", "Ruby":
		return true // no function-level visibility keywords.
	default:
		return true
	}
}

// formatLabels formats labels for a single file with 📄 header.
func formatLabels(filename string, labels []domain.Label) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("📄 %s\n", filename))

	for _, l := range labels {
		breaking := ""
		if l.Breaking {
			breaking = " ⚠ BREAKING"
		}
		b.WriteString(fmt.Sprintf("%s [%s%s] %s:%d\n", l.Name, l.Type, breaking, l.File, l.Line))
	}

	return b.String()
}

func formatSingleLabel(filename string, category string) string {
	return fmt.Sprintf("📄 %s\n%s [%s] %s\n", filename, filename, category, filename)
}

// categoryLabel returns a non-code category label for a filename, or empty.
// Order of priority: CI indicators > DEPS filenames > extension-based labels.
func categoryLabel(filename string) string {
	name := filepath.Base(filename)
	ext := strings.ToLower(filepath.Ext(filename))

	// CI indicators first (overrides extension — e.g. .gitlab-ci.yml).
	for _, pattern := range ciIndicators {
		if strings.Contains(filename, pattern) || name == pattern {
			return "CI"
		}
	}
	// DEPS filenames second (overrides extension — e.g. Cargo.toml).
	if depsFilenames[name] {
		return "DEPS"
	}
	// Extension-based labels last (e.g. .json → CONFIG, .md → DOCS).
	if cat, ok := nonCodeExtensions[ext]; ok {
		return cat
	}

	return ""
}

// appendAnnotation appends text to chunk.AnnotatedDiff with newline separator.
func appendAnnotation(chunk *domain.DiffChunk, text string) {
	if text == "" {
		return
	}
	if chunk.AnnotatedDiff != "" {
		chunk.AnnotatedDiff += "\n"
	}
	chunk.AnnotatedDiff += text
}

// isCommentOnlyHunk checks if all added/removed lines in a diff are comments.
// Used for Bug #2: comment-only changes should be labeled DOCS instead of MOD_BODY.
func isCommentOnlyHunk(diff string, filename string) bool {
	if diff == "" {
		return false
	}

	// For non-code files (markdown, etc.), always treat as docs
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == ".md" || ext == ".rst" || ext == ".adoc" || ext == ".txt" || ext == ".mdx" || ext == ".markdown" {
		return true
	}

	// Language-specific comment prefixes
	commentPrefixes := []string{"//", "#", "--", "/*", "*", "*/", "<!--"}

	hasChanges := false
	allCommentLines := true

	for _, line := range strings.Split(diff, "\n") {
		// Only look at added (+) or removed (-) lines
		if !strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "-") {
			continue
		}
		// Skip diff metadata lines
		if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			continue
		}

		hasChanges = true
		// Strip the + or - prefix
		content := strings.TrimSpace(line[1:])

		if content == "" {
			continue // blank lines are neutral
		}

		isComment := false
		for _, prefix := range commentPrefixes {
			if strings.HasPrefix(content, prefix) {
				isComment = true
				break
			}
		}
		// Also handle block comment lines (starting with just *)
		if strings.HasPrefix(content, "* ") {
			isComment = true
		}

		if !isComment {
			allCommentLines = false
			break
		}
	}

	return hasChanges && allCommentLines
}
