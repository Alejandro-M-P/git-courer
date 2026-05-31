package chunkers

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/data"
	"github.com/Alejandro-M-P/git-courer/internal/infra/chunkers/ext_lib"
	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

// receiverPattern extracts the receiver type name from a Go method signature
// like "func (s *Server) HandleReq()". Captures the type name after * or &.
var receiverPattern = regexp.MustCompile(`func\s*\([^)]*[\*\&]\s*(\w+)`)



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

// categoryLabel returns a non-code category label for a filename, or empty string.
// This is a pure filename-based heuristic — no AST parsing is needed.
func categoryLabel(filename string) string {
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

// UnifiedASTPass performs a single tree-sitter parse per file and produces
// both semantic chunk assignments and annotations.
type UnifiedASTPass struct {
	catalog       *LanguageCatalog
	ProcessCount  atomic.Int64 // test-only: counts ProcessWithContent invocations
}

func NewUnifiedASTPass(catalog *LanguageCatalog) *UnifiedASTPass {
	return &UnifiedASTPass{catalog: catalog}
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
)

// entity holds extracted function/type information from an AST.
type entity struct {
	Name      string // function or type name
	Receiver  string // receiver type name for methods (e.g. "Server"); empty for free functions
	Signature string // full declaration text for signature comparison
	Line      int    // 1-indexed start line
	Kind      string // "func" or "type"
	BodyStart int    // byte offset where entity body begins (from tree-sitter BodySpan.StartByte); 0 = no span
	BodyEnd   int    // byte offset where entity body ends (from tree-sitter Span.EndByte); 0 = no span
}

// entityKey returns the map key for an entity. Methods with a receiver
// use "Receiver.Name" format; free functions/types use just Name.
func entityKey(e entity) string {
	if e.Receiver != "" {
		return e.Receiver + "." + e.Name
	}
	return e.Name
}

// extractReceiverName parses the receiver type name from a Go method signature.
// For "func (s *Server) HandleReq()", it returns "Server".
// Returns empty string if no receiver pattern is found (free function or non-Go).
func extractReceiverName(sig string) string {
	m := receiverPattern.FindStringSubmatch(sig)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

// LevenshteinRatio computes the Levenshtein edit distance ratio between two strings.
// Returns 1.0 - (editDistance / max(len(a), len(b))).
// If both strings are empty, returns 1.0. If one is empty, returns 0.0.
func LevenshteinRatio(a, b string) float64 {
	if a == b {
		return 1.0
	}
	la, lb := len(a), len(b)
	if la == 0 || lb == 0 {
		return 0.0
	}

	// Dynamic programming Levenshtein distance
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(
				prev[j]+1,     // deletion
				curr[j-1]+1,   // insertion
				prev[j-1]+cost, // substitution
			)
		}
		prev, curr = curr, prev
	}

	distance := prev[lb]
	maxLen := la
	if lb > la {
		maxLen = lb
	}
	return 1.0 - float64(distance)/float64(maxLen)
}

func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

func (u *UnifiedASTPass) parseAndExtract(langName string, src []byte, nodes data.LanguageNodes) ([]entity, error) {
	if langName == "" || len(src) == 0 {
		return nil, nil
	}

	result, err := AnalyzeSource(langName, src)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}

	var entities []entity
	for _, s := range result.Structure {
		name := ""
		if s.Name != nil {
			name = *s.Name
		}
		if name == "" {
			continue
		}
		sig := ""
		if s.Signature != nil {
			sig = *s.Signature
		}
		if sig == "" {
			if s.BodySpan != nil {
				start := int(s.Span.StartByte)
				bodyStart := int(s.BodySpan.StartByte)
				if start >= 0 && bodyStart > start && bodyStart <= len(src) {
					rawSig := string(src[start:bodyStart])
					sig = strings.TrimSpace(rawSig)
				}
			} else {
				start := int(s.Span.StartByte)
				end := int(s.Span.EndByte)
				if start >= 0 && end > start && end <= len(src) {
					rawSig := string(src[start:end])
					sig = strings.TrimSpace(rawSig)
				}
			}
		}
		kind := "func"
		kindLower := strings.ToLower(string(s.Kind))
		switch kindLower {
		case string(ext_lib.StructureKindClass), string(ext_lib.StructureKindStruct),
			string(ext_lib.StructureKindInterface), string(ext_lib.StructureKindEnum),
			string(ext_lib.StructureKindTrait):
			kind = "type"
		}

		// Extract receiver type name for methods (e.g., "(s *Server)" → "Server").
		receiver := ""
		if kindLower == string(ext_lib.StructureKindMethod) {
			receiver = extractReceiverName(sig)
		}

		// Extract body byte span from tree-sitter.
		// BodySpan.StartByte → body start (e.g., after the '{')
		// Span.EndByte → entity end (includes closing brace)
		bodyStart := 0
		bodyEnd := 0
		if s.BodySpan != nil {
			bodyStart = int(s.BodySpan.StartByte)
			bodyEnd = int(s.Span.EndByte)
		}

		entities = append(entities, entity{
			Name:      name,
			Receiver:  receiver,
			Signature: sig,
			Line:      int(s.Span.StartLine) + 1,
			Kind:      kind,
			BodyStart: bodyStart,
			BodyEnd:   bodyEnd,
		})
	}

	return entities, nil
}

// entityMatchConfig holds the configuration for entity matching,
// including data needed for per-entity CFG computation.
type entityMatchConfig struct {
	nodes     data.LanguageNodes
	langName  string
	domainLang string
	filename  string
	beforeSrc []byte
	afterSrc  []byte
	cf        data.ControlFlowCategory
}

func (u *UnifiedASTPass) matchEntities(before, after []entity, cfg entityMatchConfig, fileCfgDiff domain.CFGDiff) []domain.Label {
	beforeMap := buildEntityMap(before)
	afterMap := buildEntityMap(after)

	var labels []domain.Label

	// Track which entities are matched by name to find orphan candidates for rename detection.
	matchedBefore := make(map[string]bool)
	matchedAfter := make(map[string]bool)

	// Entities that exist in both → modified or unchanged.
	for name, bEnt := range beforeMap {
		if aEnt, exists := afterMap[name]; exists {
			matchedBefore[name] = true
			matchedAfter[name] = true
			isFunc := bEnt.Kind == "func"
			if bEnt.Signature == aEnt.Signature {
				// Compute per-entity CFG diff when body span is available.
				entityCfg := fileCfgDiff // default: file-level CFG
				if (bEnt.BodyStart != 0 || bEnt.BodyEnd != 0) && (aEnt.BodyStart != 0 || aEnt.BodyEnd != 0) {
					entityCfg = ComputeEntityCFGDiff(cfg.langName, cfg.beforeSrc, cfg.afterSrc, bEnt.BodyStart, bEnt.BodyEnd, aEnt.BodyStart, aEnt.BodyEnd, cfg.cf)
				} else {
					slog.Debug("per-entity CFG unavailable: body span not provided by tree-sitter", "entity", name, "lang", cfg.langName)
				}
				labels = append(labels, domain.Label{
					Type:     modLabelFromCFG(isFunc, entityCfg),
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
					Breaking: isPublicEntity(aEnt, cfg.nodes),
				})
			}
		}
	}

	// Collect unmatched entities (candidates for rename or delete+new).
	var unmatchedBefore []entity
	for name, bEnt := range beforeMap {
		if !matchedBefore[name] {
			unmatchedBefore = append(unmatchedBefore, bEnt)
		}
	}
	var unmatchedAfter []entity
	for name, aEnt := range afterMap {
		if !matchedAfter[name] {
			unmatchedAfter = append(unmatchedAfter, aEnt)
		}
	}

	// Rename detection: try to match unmatched before/after entities by name similarity.
	renamedBefore := make(map[string]bool)
	renamedAfter := make(map[string]bool)
	for _, bEnt := range unmatchedBefore {
		bestKey := ""
		bestRatio := 0.0
		for _, aEnt := range unmatchedAfter {
			aKey := entityKey(aEnt)
			if renamedAfter[aKey] {
				continue
			}
			ratio := LevenshteinRatio(bEnt.Name, aEnt.Name)
			if ratio >= renamedSimilarityThreshold && ratio > bestRatio {
				bestRatio = ratio
				bestKey = aKey
			}
		}
		if bestKey != "" {
			bKey := entityKey(bEnt)
			renamedBefore[bKey] = true
			renamedAfter[bestKey] = true
			// Find the after entity for line info
			aEnt := afterMap[bestKey]

			// Guard: if signatures differ beyond the name change, this is a
			// rename + signature change → not a pure rename. Classify as MOD_SIG
			// (func) or MOD_TYPE instead of RENAMED, per spec scenario.
			if bEnt.Signature != aEnt.Signature {
				isFunc := bEnt.Kind == "func"
				// Replace the name in before signature to compare structure only.
				// If after signature differs beyond the name, it's a sig change.
				beforeStripped := strings.Replace(bEnt.Signature, bEnt.Name, aEnt.Name, 1)
				if beforeStripped != aEnt.Signature {
					if isFunc {
						labels = append(labels, domain.Label{
							Type:     domain.MOD_SIG,
							Name:     bestKey,
							Line:     aEnt.Line,
							Breaking: isPublicEntity(aEnt, cfg.nodes),
						})
					} else {
						labels = append(labels, domain.Label{
							Type:     domain.MOD_TYPE,
							Name:     bestKey,
							Line:     aEnt.Line,
							Breaking: isPublicEntity(aEnt, cfg.nodes),
						})
					}
					continue
				}
			}

			labels = append(labels, domain.Label{
				Type:     labelForKind(bEnt.Kind, labelRenamed),
				Name:     bestKey, // use after name (the new name)
				Line:     aEnt.Line,
				Breaking: isPublicEntity(aEnt, cfg.nodes),
			})
		}
	}

	// Remaining unmatched after entities → NEW.
	for _, aEnt := range unmatchedAfter {
		aKey := entityKey(aEnt)
		if !renamedAfter[aKey] {
			labels = append(labels, domain.Label{
				Type:     labelForKind(aEnt.Kind, labelNew),
				Name:     aKey,
				Line:     aEnt.Line,
				Breaking: false,
			})
		}
	}

	// Remaining unmatched before entities → DELETED.
	for _, bEnt := range unmatchedBefore {
		bKey := entityKey(bEnt)
		if !renamedBefore[bKey] {
			labels = append(labels, domain.Label{
				Type:     labelForKind(bEnt.Kind, labelDeleted),
				Name:     bKey,
				Line:     bEnt.Line,
				Breaking: isPublicEntity(bEnt, cfg.nodes),
			})
		}
	}

	for i := range labels {
		labels[i].File = cfg.filename
	}

	return labels
}

func buildEntityMap(ents []entity) map[string]entity {
	m := make(map[string]entity, len(ents))
	for _, e := range ents {
		m[entityKey(e)] = e
	}
	return m
}

// labelFamily classifies the label category for an entity change.
type labelFamily int

const (
	labelNew     labelFamily = iota // entity is new (only in after)
	labelDeleted                    // entity was deleted (only in before)
	labelRenamed                    // entity was renamed (high similarity match)
)

func labelForKind(kind string, family labelFamily) domain.LabelType {
	switch {
	case kind == "func" && family == labelNew:
		return domain.NEW_FUNC
	case kind == "func" && family == labelDeleted:
		return domain.DELETED_FUNC
	case kind == "func" && family == labelRenamed:
		return domain.RENAMED_FUNC
	case kind == "type" && family == labelNew:
		return domain.NEW_TYPE
	case kind == "type" && family == labelDeleted:
		return domain.DELETED_TYPE
	case kind == "type" && family == labelRenamed:
		return domain.RENAMED_TYPE
	default:
		if family == labelRenamed {
			return domain.RENAMED
		}
		if family == labelNew {
			return domain.NEW_TYPE // fallback
		}
		return domain.DELETED_TYPE // fallback
	}
}

// modLabelFromCFG maps CFG signals to a MOD_BODY subtype when the entity is a
// function.  The decision table follows the design:
//
//   After.Error  != Before.Error → MOD_BODY_ERROR
//   After.Branch != Before.Branch OR After.Loop != Before.Loop → MOD_BODY_LOGIC
//   After.Return != Before.Return (and no branch/loop/error change) → MOD_BODY_REORDER
//   Before == After (identical CFG) → MOD_BODY_CALL
//   fallthrough (nil/zero CFGDiff) → MOD_BODY (generic — no CFG signal available)
//   non-func → MOD_TYPE
func modLabelFromCFG(isFunc bool, cfgDiff domain.CFGDiff) domain.LabelType {
	if !isFunc {
		return domain.MOD_TYPE
	}

	before, after := cfgDiff.Before, cfgDiff.After

	// Zero CFGDiff means "not computed" — no signal available.
	// Return MOD_BODY (generic body change) since we don't know the subtype.
	if before == (domain.CFGCount{}) && after == (domain.CFGCount{}) {
		return domain.MOD_BODY
	}

	// If error count changed → MOD_BODY_ERROR
	if after.Error != before.Error {
		return domain.MOD_BODY_ERROR
	}

	// RC4: If return count increased AND error handling present → error return, not reorder
	if after.Return > before.Return && after.Error > before.Error {
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

	// Fallthrough (shouldn't reach here normally, but safe default) → MOD_BODY
	return domain.MOD_BODY
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
			symbols[name] = u.extractSymbols(entry.Name, []byte(diffStr), entry.Nodes)
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
				if cat := categoryLabel(name); cat != "" {
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
				// Grammar-supported files do NOT inject generic labels here.
				// annotateChunks() is the sole authority for semantic labels.
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
	u.ProcessCount.Add(1)

	// 1. Check for non-code category labels first (e.g. DEPS, DOCS)
	if cat := categoryLabel(filename); cat != "" {
		return []domain.Label{{Type: domain.LabelType(cat), File: filename, Name: filename}}, domain.CFGDiff{}, nil
	}

	ext := filepath.Ext(filename)
	entry, ok := u.catalog.ExtensionToLanguage(ext)
	if !ok || !entry.HasGrammar {
		return []domain.Label{{Type: "UNKNOWN_GENERIC", File: filename, Name: filename}}, domain.CFGDiff{}, nil
	}

	beforeEnts, _ := u.parseAndExtract(entry.Name, before, entry.Nodes)
	afterEnts, _ := u.parseAndExtract(entry.Name, after, entry.Nodes)

	// Compute CFG diff for control-flow metadata
	cfgDiff := ComputeCFGDiff(entry.Name, before, after, entry.Nodes.ControlFlow)

	labels := u.matchEntities(beforeEnts, afterEnts, entityMatchConfig{
		nodes:      entry.Nodes,
		langName:   entry.Name,
		domainLang: entry.DomainName,
		filename:   filename,
		beforeSrc:  before,
		afterSrc:   after,
		cf:         entry.Nodes.ControlFlow,
	}, cfgDiff)

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

func (u *UnifiedASTPass) extractSymbols(langName string, src []byte, nodes data.LanguageNodes) FileSymbols {
	defs := make(map[string]bool)
	refs := make(map[string]bool)

	if langName == "" || len(src) == 0 {
		return FileSymbols{Definitions: defs, References: refs}
	}

	result, err := AnalyzeSource(langName, src)
	if err != nil || result == nil {
		return FileSymbols{Definitions: defs, References: refs}
	}

	for _, sym := range result.Symbols {
		if sym.Name != "" {
			defs[sym.Name] = true
		}
	}

	return FileSymbols{Definitions: defs, References: refs}
}

// FileSymbols represents semantic information for a file.
type FileSymbols struct {
	Definitions map[string]bool
	References  map[string]bool
}
