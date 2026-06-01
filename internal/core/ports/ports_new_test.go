package ports

import (
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
)

// TestCatalogProvider_Interface verifies that CatalogProvider interface
// can be satisfied by a concrete type returning *domain.LanguageCatalog.
func TestCatalogProvider_Interface(t *testing.T) {
	t.Parallel()

	// Compile-time interface satisfaction check
	var _ CatalogProvider = (*mockCatalogProvider)(nil)

	provider := &mockCatalogProvider{catalog: domain.NewLanguageCatalog(nil, nil, nil)}
	got := provider.GetLanguageCatalog()
	if got == nil {
		t.Error("GetLanguageCatalog() returned nil, want non-nil")
	}
}

// mockCatalogProvider is a test mock implementing CatalogProvider.
type mockCatalogProvider struct {
	catalog *domain.LanguageCatalog
}

func (m *mockCatalogProvider) GetLanguageCatalog() *domain.LanguageCatalog {
	return m.catalog
}

// TestCommitTypeHelper_Interface verifies that CommitTypeHelper interface
// can be satisfied by a concrete type.
func TestCommitTypeHelper_Interface(t *testing.T) {
	t.Parallel()

	// Compile-time interface satisfaction check
	var _ CommitTypeHelper = (*mockCommitTypeHelper)(nil)

	helper := &mockCommitTypeHelper{}
	chunk := domain.DiffChunk{CommitType: "feat"}
	got := helper.InferCommitType(chunk)
	if got != "feat" {
		t.Errorf("InferCommitType() = %q, want %q", got, "feat")
	}
	if w := helper.CommitTypeWeight("feat"); w != 9 {
		t.Errorf("CommitTypeWeight(\"feat\") = %d, want 9", w)
	}
}

type mockCommitTypeHelper struct{}

func (m *mockCommitTypeHelper) InferCommitType(chunk domain.DiffChunk) string {
	return domain.InferCommitType(chunk)
}

func (m *mockCommitTypeHelper) CommitTypeWeight(commitType string) int {
	return domain.CommitTypeWeight(commitType)
}

// TestChunkAnnotator_AnnotateWithContent verifies that the expanded
// ChunkAnnotator interface can be satisfied by a concrete type.
func TestChunkAnnotator_AnnotateWithContent(t *testing.T) {
	t.Parallel()

	// Compile-time: ChunkAnnotator now requires both Annotate and AnnotateWithContent
	var _ ChunkAnnotator = (*mockChunkAnnotator)(nil)

	annotator := &mockChunkAnnotator{}
	chunk := &domain.DiffChunk{Files: []string{"test.go"}}
	err := annotator.AnnotateWithContent(chunk, nil, "")
	if err != nil {
		t.Errorf("AnnotateWithContent() returned unexpected error: %v", err)
	}
}

type mockChunkAnnotator struct{}

func (m *mockChunkAnnotator) Annotate(chunk *domain.DiffChunk, filename string, before, after []byte) error {
	return nil
}

func (m *mockChunkAnnotator) AnnotateWithContent(chunk *domain.DiffChunk, files []FileContent, rawDiff string) error {
	return nil
}
