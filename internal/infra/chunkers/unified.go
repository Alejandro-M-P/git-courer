package chunkers

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/data"
	"github.com/bluekeyes/go-gitdiff/gitdiff"
	gotreesitter "github.com/odvcencio/gotreesitter"
	tsgrammars "github.com/odvcencio/gotreesitter/grammars"
)

// LanguageEntry combines domain catalog info with grammar availability.
type LanguageEntry struct {
	DomainName string            // e.g. "Go"
	Name       string            // gotreesitter canonical name (e.g. "go", "c_sharp")
	Nodes      data.LanguageNodes
	Grammar    *gotreesitter.Language // nil if no gotreesitter grammar
	HasGrammar bool
}

// extToDomain is the lazy-built inverse index: extension → domain name from our catalog.
// It is populated once from AllLanguages (gotreesitter metadata) and augmented with
// languages.json keys when available.  The extensions map is keyed without the leading
// dot so that lookups are consistent.
var (
	extToDomain   map[string]string
	extIndexOnce  sync.Once
)

// buildExtIndex constructs the extension → domain name map.
// Priority order:
//   1. languages.json keys drive the primary mapping.
//      For each key, find the gotreesitter grammar whose DisplayName
//      matches the key. Register that grammar's extensions under the
//      canonical JSON key. This ensures "F#" beats "Forth" for .fs.
//   2. Any remaining gotreesitter-only languages are added without
//      overriding step 1 entries.
func buildExtIndex() {
	extToDomain = make(map[string]string)

	// Index gotreesitter entries by display name for fast lookup.
	gtByDisplay := make(map[string]tsgrammars.LangEntry)
	for _, ent := range tsgrammars.AllLanguages() {
		dName := strings.TrimSpace(tsgrammars.DisplayName(&ent))
		if dName == "" {
			continue
		}
		// Prefer earlier entry if display names collide.
		if _, exists := gtByDisplay[dName]; !exists {
			gtByDisplay[dName] = ent
		}
	}

	// Step 1 — languages.json keys have priority.
	for domain := range getAllLanguages() {
		ent, found := gtByDisplay[domain]
		if !found {
			continue
		}
		for _, raw := range ent.Extensions {
			extKey := strings.ToLower(normalizeExt(raw))
			if extKey == "" {
				continue
			}
			extToDomain[extKey] = domain
		}
	}

	// Step 2 — add gotreesitter-only languages, but never override step 1.
	for _, ent := range tsgrammars.AllLanguages() {
		dName := strings.TrimSpace(tsgrammars.DisplayName(&ent))
		for _, raw := range ent.Extensions {
			extKey := strings.ToLower(normalizeExt(raw))
			if extKey == "" {
				continue
			}
			if _, exists := extToDomain[extKey]; !exists {
				extToDomain[extKey] = dName
			}
		}
	}

	// Step 3 — Manual overrides for ambiguity and missing mappings.
	extToDomain[".fs"] = "F#"
	extToDomain[".md"] = "Markdown"
	extToDomain[".markdown"] = "Markdown"
	extToDomain[".mod"] = "Go"
	
	if tsDomain, ok := extToDomain[".ts"]; ok {
		extToDomain[".tsx"] = tsDomain
	}
	if jsDomain, ok := extToDomain[".js"]; ok {
		extToDomain[".jsx"] = jsDomain
	}
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

func (u *UnifiedASTPass) categoryLabel(filename string) string {
	base := filepath.Base(filename)
	if depsFilenames[base] {
		return "DEPS"
	}
	for _, ci := range ciIndicators {
		if strings.Contains(filename, ci) {
			return "CI"
		}
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if label, ok := nonCodeExtensions[ext]; ok {
		return label
	}
	return ""
}

// normalizeExt ensures an extension starts with exactly one leading dot.
func normalizeExt(ext string) string {
	if ext == "" {
		return ""
	}
	return "." + strings.TrimPrefix(ext, ".")
}

// guessMatchesDomain is a lightweight heuristic for extensions that do not have
// explicit gotreesitter metadata but are strongly associated with a domain name.
func guessMatchesDomain(ext, domainLower string) bool {
	switch ext {
	case ".go":
		return domainLower == "go"
	case ".mod":
		return domainLower == "go"
	case ".js", ".mjs", ".cjs":
		return domainLower == "javascript"
	case ".ts", ".tsx":
		return domainLower == "typescript"
	case ".jsx":
		return false // mapped gotreesitter via step 1
	case ".py":
		return domainLower == "python"
	case ".rs":
		return domainLower == "rust"
	case ".java":
		return domainLower == "java"
	case ".cs":
		return domainLower == "c#"
	case ".cpp", ".cc", ".cxx", ".hpp":
		return domainLower == "c++"
	case ".c", ".h":
		return domainLower == "c"
	case ".php":
		return domainLower == "php"
	case ".rb":
		return domainLower == "ruby"
	case ".swift":
		return domainLower == "swift"
	case ".kt":
		return domainLower == "kotlin"
	case ".dart":
		return domainLower == "dart"
	case ".md", ".markdown":
		return domainLower == "markdown"
	case ".fs", ".fsi", ".fsx":
		return domainLower == "f#"
	default:
		return false
	}
}

// UnifiedASTPass performs a single tree-sitter parse per file and produces
// both semantic chunk assignments and annotations.
type UnifiedASTPass struct {
	catalog *LanguageCatalog
}

func NewUnifiedASTPass(catalog *LanguageCatalog) *UnifiedASTPass {
	return &UnifiedASTPass{catalog: catalog}
}

// NewASTAnnotator returns a new UnifiedASTPass instance as a ports.ChunkAnnotator.
// This is a legacy alias to maintain backward compatibility with the old annotator.
func NewASTAnnotator() *UnifiedASTPass {
	return NewUnifiedASTPass(NewLanguageCatalog())
}

// UnifiedResult holds the output of the unified pass for a single file.
type UnifiedResult struct {
	Chunks      []domain.DiffChunk
	Labels      []domain.Label
	Definitions map[string]bool // for graph edges
	References  map[string]bool // for graph edges
}

// HunkType classifies a diff hunk's semantic content.
type HunkType int

const (
	HunkSemantic HunkType = iota // contains real code changes
	HunkStructural               // only braces, parens, separators
	HunkWhitespace               // blank lines, formatting
	HunkCommentOnly              // only comment changes
)

// entity holds extracted function/type information from an AST.
type entity struct {
	Name      string // function or type name
	Signature string // full declaration text for signature comparison
	Line      int    // 1-indexed start line
	Kind      string // "func" or "type"
}

func (u *UnifiedASTPass) parseAndExtract(lang *gotreesitter.Language, src []byte, nodes data.LanguageNodes) ([]entity, error) {
	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(src)
	if err != nil {
		return nil, err
	}
	defer tree.Release()

	root := tree.RootNode()
	if root == nil {
		return nil, nil
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

var ancestorTypeSet = map[string]bool{
	"interface_type": true,
	"type_spec":      true,
}

func walkTree(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte, funcSet, typeSet map[string]bool, result *[]entity, ancestors []string) {
	nodeType := node.Type(lang)

	if funcSet[nodeType] || typeSet[nodeType] {
		if funcSet[nodeType] && !typeSet[nodeType] {
			underTypeDecl := false
			for _, a := range ancestors {
				if ancestorTypeSet[a] {
					underTypeDecl = true
					break
				}
			}
			if underTypeDecl {
				return
			}
		}

		kind := "func"
		if typeSet[nodeType] {
			kind = "type"
		}

		name := ""
		if nameNode := node.ChildByFieldName("name", lang); nameNode != nil {
			name = nameNode.Text(src)
		}

		if name != "" {
			*result = append(*result, entity{
				Name:      name,
				Signature: signatureText(node, src),
				Line:      int(node.StartPoint().Row) + 1,
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

func signatureText(node *gotreesitter.Node, src []byte) string {
	full := node.Text(src)
	limit := len(full)
	if idx := strings.Index(full, "{"); idx >= 0 && idx < limit {
		limit = idx
	}
	if idx := strings.Index(full, "="); idx >= 0 && idx < limit {
		limit = idx
	}
	return strings.TrimSpace(full[:limit])
}

func (u *UnifiedASTPass) matchEntities(before, after []entity, nodes data.LanguageNodes, domainLang, filename string, cfgDiff domain.CFGDiff) []domain.Label {
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
				Breaking: isPublicEntity(aEnt, nodes),
			})
		}
	}

	// In both → modified or deleted.
	for name, bEnt := range beforeMap {
		if aEnt, exists := afterMap[name]; exists {
			isFunc := bEnt.Kind == "func"
			if bEnt.Signature == aEnt.Signature {
				labels = append(labels, domain.Label{
					Type:     modLabelFromCFG(isFunc, cfgDiff),
					Name:     name,
					Line:     aEnt.Line,
					Breaking: false,
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
					Breaking: isPublicEntity(aEnt, nodes),
				})
			}
		} else {
			// Only in before → deleted.
			lt := labelForKind(bEnt.Kind, false)
			labels = append(labels, domain.Label{
				Type:     lt,
				Name:     name,
				Line:     bEnt.Line,
				Breaking: isPublicEntity(bEnt, nodes),
			})
		}
	}

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

// modLabelFromCFG maps CFG signals to a MOD_BODY subtype when the entity is a
// function.  The decision table follows the design:
//
//   After.Error  != Before.Error → MOD_BODY_ERROR
//   After.Branch != Before.Branch OR After.Loop != Before.Loop → MOD_BODY_LOGIC
//   After.Return != Before.Return (and no branch/loop/error change) → MOD_BODY_REORDER
//   Before == After (identical CFG) → MOD_BODY_CALL
//   fallthrough (nil/zero CFGDiff) → MOD_BODY_LOGIC
//   non-func → MOD_TYPE
func modLabelFromCFG(isFunc bool, cfgDiff domain.CFGDiff) domain.LabelType {
	if !isFunc {
		return domain.MOD_TYPE
	}

	before, after := cfgDiff.Before, cfgDiff.After

	// Zero CFGDiff means "not computed" — no signal available.
	// Fall through to MOD_BODY_LOGIC (safest default for behavioral changes).
	if before == (domain.CFGCount{}) && after == (domain.CFGCount{}) {
		return domain.MOD_BODY_LOGIC
	}

	// If error count changed → MOD_BODY_ERROR
	if after.Error != before.Error {
		return domain.MOD_BODY_ERROR
	}

	// If branch or loop count changed → MOD_BODY_LOGIC
	if after.Branch != before.Branch || after.Loop != before.Loop {
		return domain.MOD_BODY_LOGIC
	}

	// If only return count changed (no branch/loop/error change) → MOD_BODY_REORDER
	if after.Return != before.Return {
		return domain.MOD_BODY_REORDER
	}

	// If before == after (identical CFG) → MOD_BODY_CALL
	if before == after {
		return domain.MOD_BODY_CALL
	}

	// Fallthrough (shouldn't reach here normally, but safe default) → MOD_BODY_LOGIC
	return domain.MOD_BODY_LOGIC
}

func isPublicEntity(ent entity, nodes data.LanguageNodes) bool {
	if ent.Name == "" {
		return false
	}

	// 1. Explicit Visibility Strategy from Catalog
	switch nodes.Visibility {
	case "capital":
		return ent.Name[0] >= 'A' && ent.Name[0] <= 'Z'
	case "underscore":
		return ent.Name[0] != '_'
	case "public_keyword":
		sig := strings.ToLower(ent.Signature)
		if sig == "" {
			return true // safe default
		}
		// Inclusion: keywords that mean public
		publicKeywords := []string{"pub ", "public ", "export ", "open "}
		for _, k := range publicKeywords {
			if strings.Contains(sig, k) {
				return true
			}
		}
		// Exclusion: if it has private/internal keywords, it's definitely not public
		privateKeywords := []string{"private ", "internal ", "protected "}
		for _, k := range privateKeywords {
			if strings.Contains(sig, k) {
				return false
			}
		}
		return true
	}

	// 2. Generic Heuristics (Fallback)
	if strings.HasPrefix(ent.Name, "_") {
		return false
	}
	sig := strings.ToLower(ent.Signature)
	if sig != "" && (strings.Contains(sig, "private ") || strings.Contains(sig, "internal ")) {
		return false
	}
	
	return true
}

// Process executes the unified pass over a parsed diff.
func (u *UnifiedASTPass) createClusters(graph map[string]map[string]int, filenames []string) [][]string {
	visited := make(map[string]bool)
	var clusters [][]string
	for _, f := range filenames {
		if visited[f] {
			continue
		}
		cluster := u.bfsCluster(f, graph, visited)
		clusters = append(clusters, cluster)
	}
	return clusters
}

func (u *UnifiedASTPass) bfsCluster(start string, graph map[string]map[string]int, visited map[string]bool) []string {
	var cluster []string
	queue := []string{start}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if visited[current] {
			continue
		}
		visited[current] = true
		cluster = append(cluster, current)
		for neighbor := range graph[current] {
			if !visited[neighbor] {
				queue = append(queue, neighbor)
			}
		}
	}
	return cluster
}

func (u *UnifiedASTPass) Process(files []*gitdiff.File, maxChunkSize int) ([]domain.DiffChunk, []domain.Label, error) {
	var allLabels []domain.Label
	fileMap := make(map[string]*gitdiff.File)
	var filenames []string
	
	for _, file := range files {
		name := file.NewName
		if name == "" { name = file.OldName }
		if name == "" || file.IsBinary { continue }
		fileMap[name] = file
		filenames = append(filenames, name)
	}

	// Helper to extract file diff (handling potential gitdiff issues)
	extractDiff := func(f *gitdiff.File) string {
		if len(f.TextFragments) > 0 {
			var sb strings.Builder
			for _, frag := range f.TextFragments {
				sb.WriteString(frag.String())
			}
			return sb.String()
		}
		return f.String()
	}

	// 1. Build Semantic Symbols Map
	symbols := make(map[string]FileSymbols)
	for _, name := range filenames {
		file := fileMap[name]
		ext := filepath.Ext(name)
		entry, ok := u.catalog.ExtensionToLanguage(ext)
		if ok && entry.HasGrammar {
			diffStr := extractDiff(file)
			symbols[name] = u.extractSymbols(entry.Grammar, []byte(diffStr), entry.Nodes)
		}
	}

	// 2. Build Relationship Graph
	graph := u.buildGraph(files, symbols)

	// 3. Create Clusters
	clusters := u.createClusters(graph, filenames)

	// 4. Build Chunks from Clusters
	var allChunks []domain.DiffChunk
	for _, cluster := range clusters {
		var chunkFiles []string
		var chunkDiff strings.Builder
		var chunkLabels []domain.Label
		
		for _, name := range cluster {
			file := fileMap[name]
			diffText := extractDiff(file)
			
			// Granular splitting if a single file is huge
			if maxChunkSize > 0 && len(diffText) > maxChunkSize && !u.isPairedWithAny(name, chunkFiles) {
				if chunkDiff.Len() > 0 {
					allChunks = append(allChunks, domain.DiffChunk{
						Files:         chunkFiles,
						Diff:          chunkDiff.String(),
						AnnotatedDiff: u.formatLabelsForChunk(chunkLabels),
					})
					chunkFiles, chunkLabels = nil, nil
					chunkDiff.Reset()
				}

				var currentFiles []string = []string{name}
				var currentDiff strings.Builder
				for _, frag := range file.TextFragments {
					fragText := frag.String()
					if currentDiff.Len() > 0 && currentDiff.Len()+len(fragText) > maxChunkSize {
						allChunks = append(allChunks, domain.DiffChunk{
							Files: currentFiles,
							Diff:  currentDiff.String(),
						})
						currentDiff.Reset()
					}
					currentDiff.WriteString(fragText)
				}
				if currentDiff.Len() > 0 {
					allChunks = append(allChunks, domain.DiffChunk{
						Files: currentFiles,
						Diff:  currentDiff.String(),
					})
				}
				continue
			}

			if maxChunkSize > 0 && chunkDiff.Len() > 0 && chunkDiff.Len()+len(diffText) > maxChunkSize && !u.isPairedWithAny(name, chunkFiles) {
				allChunks = append(allChunks, domain.DiffChunk{
					Files:         chunkFiles,
					Diff:          chunkDiff.String(),
					AnnotatedDiff: u.formatLabelsForChunk(chunkLabels),
				})
				chunkFiles, chunkLabels = nil, nil
				chunkDiff.Reset()
			}
			
			chunkFiles = append(chunkFiles, name)
			
			// Annotation logic
			ext := filepath.Ext(name)
			entry, ok := u.catalog.ExtensionToLanguage(ext)
			if !ok || !entry.HasGrammar {
				chunkDiff.WriteString(diffText)
				labelType := "UNKNOWN_GENERIC"
				if cat := u.categoryLabel(name); cat != "" {
					labelType = cat
				}
				label := domain.Label{Type: domain.LabelType(labelType), File: name, Name: name}
				chunkLabels = append(chunkLabels, label)
				allLabels = append(allLabels, label)
				continue
			}

			frags := u.FilterNoiseFragments(file.TextFragments)
			if len(frags) > 0 {
				chunkDiff.WriteString(u.reconstructFragments(frags))
				label := domain.Label{Type: modLabelFromCFG(true, domain.CFGDiff{}), File: name, Name: name}
				chunkLabels = append(chunkLabels, label)
				allLabels = append(allLabels, label)
			} else {
				chunkDiff.WriteString(diffText)
			}
		}
		
		if chunkDiff.Len() > 0 {
			allChunks = append(allChunks, domain.DiffChunk{
				Files:         chunkFiles,
				Diff:          chunkDiff.String(),
				AnnotatedDiff: u.formatLabelsForChunk(chunkLabels),
			})
		}
	}

	return allChunks, allLabels, nil
}

	func (u *UnifiedASTPass) formatLabelsForChunk(labels []domain.Label) string {
	var sb strings.Builder
	for _, l := range labels {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		breaking := ""
		if l.Breaking {
			breaking = " ⚠ BREAKING"
		}
		sb.WriteString(fmt.Sprintf("📄 %s\n%s [%s%s] %s:%d\n", l.File, l.Name, l.Type, breaking, l.File, l.Line))
	}
	return sb.String()
	}
func (u *UnifiedASTPass) reconstructDiff(file *gitdiff.File) string {
	var sb strings.Builder
	for _, frag := range file.TextFragments {
		sb.WriteString(frag.String())
	}
	return sb.String()
}

func (u *UnifiedASTPass) reconstructFragments(fragments []*gitdiff.TextFragment) string {
	var sb strings.Builder
	for _, frag := range fragments {
		sb.WriteString(frag.String())
	}
	return sb.String()
}

// Annotate processes before/after content for a single file and appends
// semantic labels to chunk.AnnotatedDiff. Also computes and stores CFG
// control-flow metadata on the chunk (CFGBefore/CFGAfter).
// This implements the ports.ChunkAnnotator interface.
func (u *UnifiedASTPass) Annotate(chunk *domain.DiffChunk, filename string, before, after []byte) error {
	labels, cfgDiff, err := u.ProcessWithContent(filename, before, after, nil)
	if err != nil {
		return err
	}

	// Store CFG results on the chunk (only if computed, i.e., non-zero).
	if cfgDiff.Before != (domain.CFGCount{}) || cfgDiff.After != (domain.CFGCount{}) {
		chunk.CFGBefore = &cfgDiff.Before
		chunk.CFGAfter = &cfgDiff.After
	}

	for _, l := range labels {
		if chunk.AnnotatedDiff != "" {
			chunk.AnnotatedDiff += "\n"
		}
		breaking := ""
		if l.Breaking {
			breaking = " ⚠ BREAKING"
		}
		chunk.AnnotatedDiff += fmt.Sprintf("📄 %s\n%s [%s%s] %s:%d\n", l.File, l.Name, l.Type, breaking, l.File, l.Line)
	}

	return nil
}

// ProcessWithContent executes the unified pass with full file contents.
// Returns labels, a CFGDiff for control-flow metadata, and any error.
func (u *UnifiedASTPass) ProcessWithContent(filename string, before, after []byte, fragments []*gitdiff.TextFragment) ([]domain.Label, domain.CFGDiff, error) {
	// 1. Check for non-code category labels first (e.g. DEPS, DOCS)
	if cat := u.categoryLabel(filename); cat != "" {
		return []domain.Label{{Type: domain.LabelType(cat), File: filename, Name: filename}}, domain.CFGDiff{}, nil
	}

	ext := filepath.Ext(filename)
	entry, ok := u.catalog.ExtensionToLanguage(ext)
	if !ok || !entry.HasGrammar {
		return []domain.Label{{Type: "UNKNOWN_GENERIC", File: filename, Name: filename}}, domain.CFGDiff{}, nil
	}

	beforeEnts, _ := u.parseAndExtract(entry.Grammar, before, entry.Nodes)
	afterEnts, _ := u.parseAndExtract(entry.Grammar, after, entry.Nodes)

	// Compute CFG diff for control-flow metadata
	cfgDiff := ComputeCFGDiff(entry.Grammar, before, after, entry.Nodes.ControlFlow)

	labels := u.matchEntities(beforeEnts, afterEnts, entry.Nodes, entry.DomainName, filename, cfgDiff)

	if len(labels) == 0 {
		return []domain.Label{{Type: modLabelFromCFG(true, cfgDiff), File: filename, Name: filename}}, cfgDiff, nil
	}

	return labels, cfgDiff, nil
}

// ClassifyFragmentNoise determines if a fragment contains meaningful semantic changes.
func (u *UnifiedASTPass) ClassifyFragmentNoise(frag *gitdiff.TextFragment) HunkType {
	hasSemantic := false
	for _, line := range frag.Lines {
		if line.Op == gitdiff.OpAdd || line.Op == gitdiff.OpDelete {
			content := strings.TrimSpace(line.Line)
			if content == "" {
				continue
			}
			// Noise: only braces or simple separators
			if content == "}" || content == "{" || content == "};" || content == ")," || content == ")" {
				continue
			}
			// Noise: only comments
			if strings.HasPrefix(content, "//") || strings.HasPrefix(content, "#") || strings.HasPrefix(content, "/*") || strings.HasPrefix(content, "*") {
				continue
			}
			hasSemantic = true
			break
		}
	}

	if !hasSemantic {
		return HunkStructural
	}
	return HunkSemantic
}

// FilterNoiseFragments removes fragments that don't contribute semantic value.
func (u *UnifiedASTPass) FilterNoiseFragments(fragments []*gitdiff.TextFragment) []*gitdiff.TextFragment {
	var filtered []*gitdiff.TextFragment
	for _, f := range fragments {
		if u.ClassifyFragmentNoise(f) == HunkSemantic {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

func (u *UnifiedASTPass) buildGraph(files []*gitdiff.File, symbols map[string]FileSymbols) map[string]map[string]int {
	graph := make(map[string]map[string]int)
	for i := 0; i < len(files); i++ {
		for j := i + 1; j < len(files); j++ {
			f1, f2 := files[i], files[j]
			name1 := f1.NewName
			if name1 == "" { name1 = f1.OldName }
			name2 := f2.NewName
			if name2 == "" { name2 = f2.OldName }
			
			force := u.calculateForce(name1, name2, symbols[name1], symbols[name2])
			if force > 0 {
				if graph[name1] == nil {
					graph[name1] = make(map[string]int)
				}
				if graph[name2] == nil {
					graph[name2] = make(map[string]int)
				}
				graph[name1][name2] = force
				graph[name2][name1] = force
			}
		}
	}
	return graph
}

func (u *UnifiedASTPass) isPairedWithAny(name string, chunkFiles []string) bool {
	for _, f := range chunkFiles {
		if u.catalog.ArePaired(f, name) || u.catalog.ArePaired(name, f) {
			return true
		}
	}
	return false
}

func (u *UnifiedASTPass) calculateForce(name1, name2 string, s1, s2 FileSymbols) int {
	force := 0
	// 1. Code-Test Pair (+1000)
	if u.catalog.ArePaired(name1, name2) || u.catalog.ArePaired(name2, name1) {
		force += 1000
	}
	// 2. Directory Affinity (+100)
	if filepath.Dir(name1) == filepath.Dir(name2) {
		force += 100
	}
	// 3. Semantic Link (+500)
	for ref := range s1.References {
		if s2.Definitions[ref] {
			force += 500
		}
	}
	for ref := range s2.References {
		if s1.Definitions[ref] {
			force += 500
		}
	}
	return force
}

func (u *UnifiedASTPass) extractSymbols(lang *gotreesitter.Language, src []byte, nodes data.LanguageNodes) FileSymbols {
	defs := make(map[string]bool)
	refs := make(map[string]bool)

	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(src)
	if err != nil {
		return FileSymbols{Definitions: defs, References: refs}
	}
	defer tree.Release()

	root := tree.RootNode()
	if root == nil {
		return FileSymbols{Definitions: defs, References: refs}
	}

	funcSet := make(map[string]bool, len(nodes.Functions))
	for _, f := range nodes.Functions {
		funcSet[f] = true
	}
	typeSet := make(map[string]bool, len(nodes.Types))
	for _, t := range nodes.Types {
		typeSet[t] = true
	}

	walkForSymbols(root, lang, src, funcSet, typeSet, defs, refs)
	return FileSymbols{Definitions: defs, References: refs}
}

// FileSymbols represents semantic information for a file.
type FileSymbols struct {
	Definitions map[string]bool
	References  map[string]bool
}

func walkForSymbols(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte, funcSet, typeSet map[string]bool, defs, refs map[string]bool) {
	nodeType := node.Type(lang)

	if funcSet[nodeType] || typeSet[nodeType] {
		if nameNode := node.ChildByFieldName("name", lang); nameNode != nil {
			defs[nameNode.Text(src)] = true
		}
	}

	// Simple reference extraction: identifiers that are NOT in a 'name' field
	if nodeType == "identifier" || nodeType == "type_identifier" {
		name := node.Text(src)
		if !defs[name] {
			refs[name] = true
		}
	}

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child != nil {
			walkForSymbols(child, lang, src, funcSet, typeSet, defs, refs)
		}
	}
}
// domain display name.  It handles special cases like "C#" → "c_sharp",
// "F#" → "fsharp", "C++" → "cpp".
func canonicalGrammarName(domainName string) string {
	switch domainName {
	case "C#":
		return "c_sharp"
	case "F#":
		return "fsharp"
	case "C++":
		return "cpp"
	}
	ent := tsgrammars.DetectLanguageByName(strings.ToLower(domainName))
	if ent != nil {
		return ent.Name
	}
	return strings.ToLower(domainName)
}

// grammarFactoryCache is a one-time registry that maps canonical grammar
// names to factory functions via reflection over the grammars package.
// It is built at init time so ResolveGrammar is O(1).
var (
	grammarFactories map[string]func() *gotreesitter.Language
	grammarOnce      sync.Once
)

// buildGrammarFactoryCache enumerates available gotreesitter grammars by
// reflecting over the tsgrammars package.  Factory functions have the
// signature `func() *gotreesitter.Language` and are named like
// `GoLanguage`, `CLanguage`, `CSharpLanguage`, etc.
func buildGrammarFactoryCache() {
	grammarFactories = make(map[string]func() *gotreesitter.Language)

	// Hardcoded safety map for the 13 originally-supported plus any known gaps.
	// This is a bootstrap layer: each entry is verified against tsgrammars at
	// runtime; if the factory is nil the entry is skipped.  The real work is
	// done below via AllLanguages.
	for _, ent := range tsgrammars.AllLanguages() {
		if ent.Language == nil {
			continue
		}
		// Capture in loop-local variable to avoid closure aliasing.
		factory := ent.Language
		grammarFactories[ent.Name] = factory

		// Also index by display name (the key used in languages.json).
		display := strings.TrimSpace(tsgrammars.DisplayName(&ent))
		if display != "" && display != ent.Name {
			grammarFactories[strings.ToLower(display)] = factory
		}
	}
}

// ResolveGrammar resolves a gotreesitter grammar dynamically given a language
// display name (e.g. "Go", "JavaScript", "C#").  It falls back to a
// hardcoded safety map for edge cases.  Returns nil, false when no grammar
// exists so the caller can fall back to word-tokenization.
func ResolveGrammar(langID string) (*gotreesitter.Language, bool) {
	if langID == "" {
		return nil, false
	}

	grammarOnce.Do(buildGrammarFactoryCache)

	// Try exact match first.
	if fn, ok := grammarFactories[langID]; ok {
		return fn(), true
	}

	// Try normalized key (lowercase display).
	lower := strings.ToLower(langID)
	if fn, ok := grammarFactories[lower]; ok {
		return fn(), true
	}

	// Try canonical name derived from display name.
	canonical := canonicalGrammarName(langID)
	if fn, ok := grammarFactories[canonical]; ok {
		return fn(), true
	}

	return nil, false
}
