package sync

import "github.com/blak0p/git-courer/internal/core/ports"

type Handler struct {
	git ports.Git
}

func NewHandler(git ports.Git) *Handler {
	return &Handler{git: git}
}
