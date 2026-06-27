package ports

import "github.com/blak0p/git-courer/internal/core/domain"

// SessionStore persists session metadata.
type SessionStore interface {
	Get(id string) (*domain.Session, error)
	Save(session *domain.Session) error
	Delete(id string) error
	List() ([]*domain.Session, error)
}