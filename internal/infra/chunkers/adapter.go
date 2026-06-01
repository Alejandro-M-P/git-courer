package chunkers

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/core/ports"
)

// ChunkAnnotatorAdapter wraps a UnifiedASTPass to implement ports.ChunkAnnotator.
// This is the adapter that translates between the domain port and the infra implementation.
type ChunkAnnotatorAdapter struct {
	unifiedPass *UnifiedASTPass
	catalog     *LanguageCatalog
}

// NewChunkAnnotatorAdapter creates a new ChunkAnnotatorAdapter with the given language catalog.
func NewChunkAnnotatorAdapter(catalog *LanguageCatalog) *ChunkAnnotatorAdapter {
	return &ChunkAnnotatorAdapter{
		unifiedPass: NewUnifiedASTPass(catalog),
		catalog:     catalog,
	}
}

// Compile-time interface check
var _ ports.ChunkAnnotator = (*ChunkAnnotatorAdapter)(nil)

// Annotate analyses before/after content for a single file and populates
// chunk.AnnotatedDiff and CFG metadata.
func (a *ChunkAnnotatorAdapter) Annotate(chunk *domain.DiffChunk, filename string, before, after []byte) error {
	return a.unifiedPass.Annotate(chunk, filename, before, after)
}

// AnnotateWithContent processes all files in the content list using the unified AST pass,
// populates AnnotatedDiff (using AnnotateDiffForRead for inline labels with diff lines),
// CFGBefore/CFGAfter, and BeforeSource/AfterSource on the chunk.
//
// IMPORTANT: ProcessWithContent is called exactly ONCE per file. The labels are then
// shared with AnnotateDiffForRead, which no longer creates its own pass.
func (a *ChunkAnnotatorAdapter) AnnotateWithContent(chunk *domain.DiffChunk, files []ports.FileContent, rawDiff string) error {
	// Phase 1: Compute labels and metadata for all files (one pass per file).
	labelsMap := make(map[string][]domain.Label)
	var accumulatedBefore, accumulatedAfter domain.CFGCount
	var hasCFG bool

	for _, fc := range files {
		labels, cfgDiff, err := a.unifiedPass.ProcessWithContent(fc.Filename, fc.Before, fc.After, nil)
		if err != nil {
			log.Printf("[WARN] Failed to get AST metadata for %s: %v", fc.Filename, err)
			continue
		}

		// Store labels for AnnotateDiffForRead
		if len(labels) > 0 {
			labelsMap[fc.Filename] = labels
		}

		// Accumulate CFG metadata when non-zero
		if cfgDiff.Before != (domain.CFGCount{}) || cfgDiff.After != (domain.CFGCount{}) {
			hasCFG = true
			accumulatedBefore.Branch += cfgDiff.Before.Branch
			accumulatedBefore.Loop += cfgDiff.Before.Loop
			accumulatedBefore.Return += cfgDiff.Before.Return
			accumulatedBefore.Error += cfgDiff.Before.Error

			accumulatedAfter.Branch += cfgDiff.After.Branch
			accumulatedAfter.Loop += cfgDiff.After.Loop
			accumulatedAfter.Return += cfgDiff.After.Return
			accumulatedAfter.Error += cfgDiff.After.Error
		}

		// Populate BeforeSource/AfterSource using LanguageCatalog filter
		ext := filepath.Ext(fc.Filename)
		if entry, ok := a.catalog.ExtensionToLanguage(ext); ok && entry.HasGrammar {
			if chunk.BeforeSource == nil {
				chunk.BeforeSource = make(map[string]string)
			}
			if chunk.AfterSource == nil {
				chunk.AfterSource = make(map[string]string)
			}
			if len(fc.Before) > 0 {
				chunk.BeforeSource[fc.Filename] = string(fc.Before)
			}
			if len(fc.After) > 0 {
				chunk.AfterSource[fc.Filename] = string(fc.After)
			}
		}
	}

	if hasCFG {
		chunk.CFGBefore = &accumulatedBefore
		chunk.CFGAfter = &accumulatedAfter
	}

	// Phase 2: Build annotated diff using pre-computed labels (no duplicate ProcessWithContent).
	contentProvider := &fileContentProvider{files: files}
	annotatedDiff := AnnotateDiffForRead(rawDiff, contentProvider, labelsMap, a.catalog)

	if annotatedDiff != "" {
		chunk.AnnotatedDiff = annotatedDiff
	} else {
		// Fallback: build labels manually using pre-computed labels
		chunk.AnnotatedDiff = buildLabelOnlyAnnotation(labelsMap)
	}

	return nil
}

// fileContentProvider wraps a []ports.FileContent to implement the
// ports.ContentProvider interface needed by AnnotateDiffForRead.
type fileContentProvider struct {
	files []ports.FileContent
}

func (f *fileContentProvider) GetContents(filenames []string) ([]ports.FileContent, error) {
	// Return only the files that match the requested filenames
	var result []ports.FileContent
	for _, name := range filenames {
		for _, fc := range f.files {
			if fc.Filename == name {
				result = append(result, fc)
				break
			}
		}
	}
	return result, nil
}

// buildLabelOnlyAnnotation constructs a label-only annotation string as a fallback
// when AnnotateDiffForRead cannot process the diff (e.g., no grammar available).
// Uses pre-computed labels instead of calling ProcessWithContent again.
func buildLabelOnlyAnnotation(labelsMap map[string][]domain.Label) string {
	var sb strings.Builder
	for _, labels := range labelsMap {
		for _, l := range labels {
			if sb.Len() > 0 {
				sb.WriteByte('\n')
			}
			breaking := ""
			if l.Breaking {
				breaking = " ⚠ BREAKING"
			}
			sb.WriteString(fmt.Sprintf("📄 %s\n%s [%s%s] %s:%d", l.File, l.Name, l.Type, breaking, l.File, l.Line))
		}
	}
	return sb.String()
}
