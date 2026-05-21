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
}

// NewChunkAnnotatorAdapter creates a new ChunkAnnotatorAdapter with the given language catalog.
func NewChunkAnnotatorAdapter(catalog *LanguageCatalog) *ChunkAnnotatorAdapter {
	return &ChunkAnnotatorAdapter{
		unifiedPass: NewUnifiedASTPass(catalog),
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
// populates AnnotatedDiff, CFGBefore/CFGAfter, and GoBefore/GoAfter on the chunk,
// then merges diff into annotations.
func (a *ChunkAnnotatorAdapter) AnnotateWithContent(chunk *domain.DiffChunk, files []ports.FileContent, rawDiff string) error {
	var annotated strings.Builder
	for _, fc := range files {
		labels, cfgDiff, err := a.unifiedPass.ProcessWithContent(fc.Filename, fc.Before, fc.After, nil)
		if err != nil {
			log.Printf("[WARN] Failed to annotate file %s: %v", fc.Filename, err)
		}

		for _, l := range labels {
			if annotated.Len() > 0 {
				annotated.WriteString("\n")
			}
			breaking := ""
			if l.Breaking {
				breaking = " ⚠ BREAKING"
			}
			annotated.WriteString(fmt.Sprintf("📄 %s\n%s [%s%s] %s:%d\n", l.File, l.Name, l.Type, breaking, l.File, l.Line))
		}

		// Populate CFG metadata from annotator when non-zero
		if cfgDiff.Before != (domain.CFGCount{}) || cfgDiff.After != (domain.CFGCount{}) {
			chunk.CFGBefore = &cfgDiff.Before
			chunk.CFGAfter = &cfgDiff.After
		}

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
	chunk.AnnotatedDiff = annotated.String()

	MergeDiffIntoAnnotations(chunk, rawDiff)
	return nil
}