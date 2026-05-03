package mcp

import (
	"encoding/json"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

func logResultJSON(res LogResult) string {
	type commitItem struct {
		Hash    string `json:"hash"`
		Message string `json:"message"`
		Author  string `json:"author"`
		Date    string `json:"date"`
	}
	items := make([]commitItem, 0, len(res.Commits))
	for _, c := range res.Commits {
		items = append(items, commitItem{
			Hash:    c.Hash,
			Message: c.Message,
			Author:  c.Author,
			Date:    c.Date,
		})
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"commits":     items,
		"total":       res.TotalCommits,
		"returned":    res.Returned,
		"offset":      res.Offset,
		"truncated":   res.Truncated,
		"next_offset": res.NextOffset,
		"message":     res.Message,
	})
	return string(resp)
}

func blameResultJSON(filepath string, lines []domain.BlameLine, limit, offset int) string {
	type lineItem struct {
		LineNumber int    `json:"line_number"`
		Author     string `json:"author,omitempty"`
		Hash       string `json:"hash"`
	}
	if offset > len(lines) {
		offset = len(lines)
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	end := offset + limit
	if end > len(lines) {
		end = len(lines)
	}
	items := make([]lineItem, 0, end-offset)
	for _, l := range lines[offset:end] {
		items = append(items, lineItem{LineNumber: l.Line, Author: l.Author, Hash: l.Hash})
	}
	resp, _ := json.Marshal(map[string]interface{}{
		"file":     filepath,
		"lines":    items,
		"total":    len(lines),
		"offset":   offset,
		"limit":    limit,
		"next_off": end,
	})
	return string(resp)
}

func showResultJSON(res domain.ShowResult) string {
	resp, _ := json.Marshal(map[string]interface{}{
		"hash":    res.Hash,
		"message": res.Message,
		"author":  res.Author,
		"date":    res.Date,
		"files":   res.FileList,
		"stats": map[string]int{
			"files":     res.Files,
			"additions": res.Additions,
			"deletions": res.Deletions,
		},
	})
	return string(resp)
}

func reflogResultJSON(entries []domain.ReflogEntry, limit, offset int) string {
	type opItem struct {
		Index  int    `json:"index"`
		Action string `json:"action,omitempty"`
		Hash   string `json:"hash"`
	}
	if offset > len(entries) {
		offset = len(entries)
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	end := offset + limit
	if end > len(entries) {
		end = len(entries)
	}
	items := make([]opItem, 0, end-offset)
	for _, e := range entries[offset:end] {
		items = append(items, opItem{Index: e.Index, Action: e.Action, Hash: e.Hash})
	}
	resp, _ := json.Marshal(map[string]interface{}{
		"operations": items,
		"total":      len(entries),
		"offset":     offset,
		"limit":      limit,
		"next":       end,
	})
	return string(resp)
}

func stashListResultJSON(entries []domain.StashEntry, limit, offset int) string {
	type stItem struct {
		Index   int    `json:"index"`
		Message string `json:"message"`
		Hash    string `json:"hash"`
	}
	if offset > len(entries) {
		offset = len(entries)
	}
	if limit <= 0 || limit > 50 {
		limit = 50
	}
	end := offset + limit
	if end > len(entries) {
		end = len(entries)
	}
	items := make([]stItem, 0, end-offset)
	for _, e := range entries[offset:end] {
		items = append(items, stItem{Index: e.Index, Message: e.Message, Hash: e.Hash})
	}
	resp, _ := json.Marshal(map[string]interface{}{
		"stashes":     items,
		"total":       len(entries),
		"offset":      offset,
		"limit":       limit,
		"next_offset": end,
	})
	return string(resp)
}

func mergeBaseResultJSON(base, a, b string) string {
	resp, _ := json.Marshal(map[string]interface{}{
		"base": base,
		"a":    a,
		"b":    b,
	})
	return string(resp)
}

func formatBackupListJSON(backups []domain.Backup) string {
	type backupItem struct {
		Ref       string `json:"ref"`
		Operation string `json:"operation"`
		CreatedAt string `json:"created_at"`
	}
	items := make([]backupItem, 0, len(backups))
	for _, b := range backups {
		items = append(items, backupItem{
			Ref:       b.Ref,
			Operation: b.Operation,
			CreatedAt: b.CreatedAt.Format(time.RFC3339),
		})
	}
	resp, _ := json.Marshal(map[string]interface{}{
		"backups": items,
	})
	return string(resp)
}
