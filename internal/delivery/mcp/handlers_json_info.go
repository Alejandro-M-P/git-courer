package mcp

import (
	"encoding/json"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

func formatStatusJSON(s domain.Status, limit, offset int, filter string) string {
	files := s.Files
	if filter != "" {
		var filtered []domain.FileStatus
		for _, f := range files {
			if matchesFilter(f.Path, filter) {
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

func formatBranchListJSON(branches []string, current string, limit, offset int) string {
	total := len(branches)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	page := branches[offset:end]
	truncated := end < total

	nextOffset := 0
	if truncated {
		nextOffset = end
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"current":     current,
		"total":       total,
		"returned":    len(page),
		"offset":      offset,
		"truncated":   truncated,
		"next_offset": nextOffset,
		"branches":    page,
	})
	return string(resp)
}

func formatTagListJSON(tags []string, limit, offset int) string {
	total := len(tags)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	page := tags[offset:end]
	truncated := end < total

	nextOffset := 0
	if truncated {
		nextOffset = end
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"total":       total,
		"returned":    len(page),
		"offset":      offset,
		"next_offset": nextOffset,
		"truncated":   truncated,
		"tags":        page,
	})
	return string(resp)
}

func formatFileListJSON(files []string, limit, offset int) string {
	total := len(files)
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

	resp, _ := json.Marshal(map[string]interface{}{
		"total":       total,
		"returned":    len(page),
		"offset":      offset,
		"next_offset": nextOffset,
		"truncated":   truncated,
		"files":       page,
	})
	return string(resp)
}

func formatRemoteBranchListJSON(branches []string, limit, offset int) string {
	return formatBranchListJSON(branches, "", limit, offset)
}

func formatRemoteTagListJSON(tags []string, limit, offset int) string {
	return formatTagListJSON(tags, limit, offset)
}
