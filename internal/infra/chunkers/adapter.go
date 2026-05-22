package chunkers

import (
	"fmt"
	"log"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// ChunkAnnotatorAdapter wraps a UnifiedASTPass to implement ports.ChunkAnnotator.
// This is the adapter that translates between the domain port and the infra implementation.
type ChunkAnnotatorAdapter struct {
	unifiedPass *UnifiedASTPass
	catalog    *LanguageCatalog
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
// CFGBefore/CFGAfter, and GoBefore/GoAfter on the chunk.
func (a *ChunkAnnotatorAdapter) AnnotateWithContent(chunk *domain.DiffChunk, files []ports.FileContent, rawDiff string) error {
	// Use AnnotateDiffForRead for the AnnotatedDiff field — it produces the correct
	// inline format with labels in @@ headers and full diff lines, which is what
	// both the LLM and the MCP diff tool expect.
	// Fall back to manual label construction if AnnotateDiffForRead returns empty.
	contentProvider := &fileContentProvider{files: files}
	annotatedDiff := AnnotateDiffForRead(rawDiff, contentProvider)

	if annotatedDiff != "" {
		chunk.AnnotatedDiff = annotatedDiff
	} else {
		// Fallback: build labels manually if AnnotateDiffForRead couldn't process
		chunk.AnnotatedDiff = buildLabelOnlyAnnotation(files, a.unifiedPass)
	}

	// Populate per-file metadata from the AST pass
	for _, fc := range files {
		labels, cfgDiff, err := a.unifiedPass.ProcessWithContent(fc.Filename, fc.Before, fc.After, nil)
		if err != nil {
			log.Printf("[WARN] Failed to get AST metadata for %s: %v", fc.Filename, err)
			continue
		}

		// Populate CFG metadata when non-zero
		if cfgDiff.Before != (domain.CFGCount{}) || cfgDiff.After != (domain.CFGCount{}) {
			chunk.CFGBefore = &cfgDiff.Before
			chunk.CFGAfter = &cfgDiff.After
		}

		// Track whether we found AST labels for commit type classification
		_ = labels

		if strings.HasSuffix(fc.Filename, ".go") {
			if chunk.GoBefore == nil {
				chunk.GoBefore = make(map[string]string)
			}
			if chunk.GoAfter == nil {
				chunk.GoAfter = make(map[string]string)
			}
			if len(fc.Before) > 0 {
				chunk.GoBefore[fc.Filename] = string(fc.Before)
			}
			if len(fc.After) > 0 {
				chunk.GoAfter[fc.Filename] = string(fc.After)
			}
		}
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
func buildLabelOnlyAnnotation(files []ports.FileContent, pass *UnifiedASTPass) string {
	var sb strings.Builder
	for _, fc := range files {
		labels, _, err := pass.ProcessWithContent(fc.Filename, fc.Before, fc.After, nil)
		if err != nil || len(labels) == 0 {
			continue
		}
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

// Compile-time interface check
var _ ports.ChunkAnnotator = (*ChunkAnnotatorAdapter)(nil)