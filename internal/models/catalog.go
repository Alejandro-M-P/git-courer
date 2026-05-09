package models

import (
	"context"
	"strings"
	"sync"

	"github.com/Alejandro-M-P/git-courer/internal/data"
)

// DetectorIface is the interface for runtime model context-window detectors.
// Satisfied by *OllamaDetector.
type DetectorIface interface {
	Lookup(ctx context.Context, model string) (int, bool)
}

// ModelCatalog resolves context-window sizes for LLM models through a
// three-tier strategy:
//   1. Runtime detector (e.g. Ollama /api/show) — highest fidelity.
//   2. Embedded catalog (models.json shipped in the binary).
//   3. Conservative default of 4096 tokens.
type ModelCatalog struct {
	mu       sync.RWMutex
	detector DetectorIface           // nil when Ollama is unavailable
	data     map[string]int          // full model name → context window
	baseName map[string]int          // base name (strip tag) → smallest window for the family
	default_ int                     // always 4096
}

// NewModelCatalog creates a catalog backed by embedded model data and an
// optional runtime detector.  Pass nil for the detector if Ollama is not
// reachable (e.g. unit tests, offline mode).
func NewModelCatalog(detector DetectorIface) *ModelCatalog {
	// Clone embedded data so we can mutate in tests if needed.
	raw := make(map[string]int)
	for name, w := range data.GetAllModelData() {
		raw[name] = w
	}

	return &ModelCatalog{
		detector: detector,
		data:     raw,
		baseName: buildBaseNameIndex(raw),
		default_: 4096,
	}
}

// NewModelCatalogNoOllama is a convenience constructor for tests and
// environments where Ollama is not available.
func NewModelCatalogNoOllama() *ModelCatalog {
	return NewModelCatalog(nil)
}

// NewModelCatalogWithDetector is a convenience constructor when you already
// have a detector instance (useful in tests with mocks).
func NewModelCatalogWithDetector(detector DetectorIface) *ModelCatalog {
	return NewModelCatalog(detector)
}

// GetContextWindow returns the best-effort context-window size for the given
// model name.  It never does I/O — the model string alone drives the lookup.
func (c *ModelCatalog) GetContextWindow(model string) int {
	return c.GetContextWindowWithCtx(context.Background(), model)
}

// GetContextWindowWithCtx is the context-aware variant; useful when the
// detector may need timeouts or cancellation.
func (c *ModelCatalog) GetContextWindowWithCtx(ctx context.Context, model string) int {
	if model == "" {
		return c.default_
	}

	// Tier 1 — ask the runtime detector if available.
	if c.detector != nil {
		if w, ok := c.detector.Lookup(ctx, model); ok {
			return w
		}
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	// Tier 2 — exact match in embedded catalog.
	if w, ok := c.data[model]; ok {
		return w
	}

	// Tier 3 — model-family lookup via base-name index.
	if w, ok := longestPrefixMatch(model, c.baseName); ok {
		return w
	}

	// Tier 4 — conservative default.
	return c.default_
}

// stripTag removes the tag portion from a model name.  The tag is the
// substring after the LAST colon.
// Examples:
//   "llama3.1:8b"   → "llama3.1"
//   "mistral:7b"    → "mistral"
//   "deepseek-v3"   → "deepseek-v3"
//   "a:b:c:7b"      → "a:b:c"
//   ""                → ""
func stripTag(model string) string {
	idx := strings.LastIndex(model, ":")
	if idx == -1 {
		return model
	}
	return model[:idx]
}

// buildBaseNameIndex creates an index from base model name to the SMALLEST
// context-window value found for that family.  Smallest wins because we
// prefer conservative estimates when we can only match by prefix.
func buildBaseNameIndex(data map[string]int) map[string]int {
	index := make(map[string]int, len(data))
	for name, w := range data {
		base := stripTag(name)
		if existing, ok := index[base]; !ok || w < existing {
			index[base] = w
		}
	}
	return index
}

// longestPrefixMatch looks for the longest catalog key that is a prefix of
// the query.  It returns the associated value and true if found.
//
// The query must be longer than the key (so "llama" won't match "llama3.1",
// but "llama3.1:13b" WILL match "llama3.1").
func longestPrefixMatch(query string, index map[string]int) (int, bool) {
	var bestKey string
	var bestLen int
	for key := range index {
		if !strings.HasPrefix(query, key) {
			continue
		}
		if len(key) > bestLen {
			bestLen = len(key)
			bestKey = key
		}
	}
	if bestLen == 0 {
		return 0, false
	}
	return index[bestKey], true
}
