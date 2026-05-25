package ports

import "github.com/Alejandro-M-P/git-courer/internal/core/domain"

// mockCommitStore is a test double that implements CommitStore.
// The compile-time check below ensures the interface is correctly defined.
type mockCommitStore struct {
	appended []domain.CommitEntry
	readErr  error
	clearErr error
}

func (m *mockCommitStore) Append(entries ...domain.CommitEntry) error {
	m.appended = append(m.appended, entries...)
	return nil
}

func (m *mockCommitStore) Read() ([]domain.CommitEntry, error) {
	return m.appended, m.readErr
}

func (m *mockCommitStore) Clear() error {
	return m.clearErr
}

func (m *mockCommitStore) SetBranch(name string) error {
	return nil
}

func (m *mockCommitStore) RemoveBranch(name string) error {
	return nil
}

// Compile-time interface satisfaction check.
// This will fail to compile if CommitStore is not correctly defined
// or if mockCommitStore doesn't implement all methods.
var _ CommitStore = (*mockCommitStore)(nil)