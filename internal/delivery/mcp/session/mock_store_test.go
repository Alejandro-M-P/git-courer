package session

import (
	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/stretchr/testify/mock"
)

// MockSessionStore is a testify-based mock implementing ports.SessionStore
// for session finish/status/discard tests.
type MockSessionStore struct {
	mock.Mock
}

func (m *MockSessionStore) Get(id string) (*domain.Session, error) {
	args := m.Called(id)
	var sess *domain.Session
	if v := args.Get(0); v != nil {
		sess = v.(*domain.Session)
	}
	return sess, args.Error(1)
}

func (m *MockSessionStore) Save(session *domain.Session) error {
	args := m.Called(session)
	return args.Error(0)
}

func (m *MockSessionStore) Delete(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockSessionStore) List() ([]*domain.Session, error) {
	args := m.Called()
	var sessions []*domain.Session
	if v := args.Get(0); v != nil {
		sessions = v.([]*domain.Session)
	}
	return sessions, args.Error(1)
}