package ports

import "github.com/Alejandro-M-P/git-courer/internal/core/domain"

// CatalogProvider provides access to the language catalog.
// Implemented by DiffChunker and other infra types that hold a catalog.
type CatalogProvider interface {
	// GetLanguageCatalog returns the language catalog used for AST analysis
	// and code-test file pairing.
	GetLanguageCatalog() *domain.LanguageCatalog
}
