package mcp

import (
	"encoding/json"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/delivery/mcp/shared"
)

func formatStatusJSON(s domain.Status, limit, offset int, filter string) string {
	files := s.Files
	if filter != "" {
		var filtered []domain.FileStatus
		for _, f := range files {
			if shared.MatchesFilter(f.Path, filter) {
				filtered = append(filtered, f)
			}
		}
		files = filtered
	}

	total := len(files)
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	page := files[offset:end]
	truncated := end < total

	nextOffset := 0
	if truncated {
		nextOffset = end
	}

	type fileItem struct {
		Path   string `json:"path"`
		Status string `json:"status"`
		Staged bool   `json:"staged"`
	}

	fItems := make([]fileItem, 0, len(page))
	for _, f := range page {
		fItems = append(fItems, fileItem{
			Path:   f.Path,
			Status: f.Status,
			Staged: f.Staged,
		})
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"branch":       s.Branch,
		"ahead":        s.Ahead,
		"behind":       s.Behind,
		"has_upstream": s.HasUpstream,
		"clean":        s.IsClean,
		"total":        total,
		"returned":     len(page),
		"offset":       offset,
		"truncated":    truncated,
		"next_offset":  nextOffset,
		"staged":       s.Staged,
		"modified":     s.Modified,
		"untracked":    s.Untracked,
		"files":        fItems,
	})
	return string(resp)
}
