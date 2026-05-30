package sync

import "github.com/Alejandro-M-P/git-courer/internal/core/ports"

type Handler struct {
	git ports.Git
}

func NewHandler(git ports.Git) *Handler {
	return &Handler{git: git}
}
