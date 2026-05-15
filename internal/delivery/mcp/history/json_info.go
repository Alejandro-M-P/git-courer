package history

import (
	"encoding/json"
)

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
