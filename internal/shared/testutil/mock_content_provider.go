package testutil

import (
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// MockContentProvider implements ports.ContentProvider for testing.
type MockContentProvider struct {
	Files map[string]ports.FileContent
}

func NewMockContentProvider() *MockContentProvider {
	return &MockContentProvider{
		Files: make(map[string]ports.FileContent),
	}
}

func (m *MockContentProvider) AddFile(filename string, before, after []byte) {
	m.Files[filename] = ports.FileContent{
		Filename: filename,
		Before:   before,
		After:    after,
	}
}

func (m *MockContentProvider) GetContents(files []string) ([]ports.FileContent, error) {
	var result []ports.FileContent
	for _, f := range files {
		if fc, ok := m.Files[f]; ok {
			result = append(result, fc)
		}
	}
	return result, nil
}
