package mcp

import (
	"fmt"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/infra/filters"
)

// WhatChangedFilter restricts WHAT_CHANGED output to staged, unstaged, or all changes.
type WhatChangedFilter string

const (
	WhatChangedFilterAll      WhatChangedFilter = "all"
	WhatChangedFilterStaged   WhatChangedFilter = "staged"
	WhatChangedFilterUnstaged WhatChangedFilter = "unstaged"
)

// handleDiffCommand handles READ_DIFF and READ_DIFF_STAGED with optional range syntax.
func (s *Server) handleDiffCommand(arg string, limit, offset int, cachedFlag string, fileFilter string, compact bool) (string, error) {
	if strings.HasPrefix(arg, "..") || strings.HasPrefix(arg, "...") {
		current, err := s.git.CurrentBranch()
		if err != nil {
			return "", err
		}
		target := arg
		mode := ""
		if strings.HasPrefix(arg, "...") {
			mode = "..."
			target = strings.TrimPrefix(arg, "...")
		} else {
			mode = ".."
			target = strings.TrimPrefix(arg, "..")
		}
		raw, err := s.git.DiffRange(current, target, mode)
		if err != nil {
			return "", err
		}
		res := SanitizeDiff(raw, offset, limit)
		res.Mode = mode
		res.Base = current
		res.Target = target
		return diffResultJSON(res), nil
	}

	var raw string
	var err error
	if arg != "" {
		if cachedFlag != "" {
			raw, err = s.git.DiffStaged(arg)
		} else {
			raw, err = s.git.Diff(arg)
		}
	} else {
		if cachedFlag != "" {
			raw, err = s.git.DiffStaged()
		} else {
			raw, err = s.git.Diff()
		}
	}
	if err != nil {
		return "", err
	}

	if fileFilter != "" {
		raw = filterDiffByFile(raw, fileFilter)
	}
	if compact {
		raw = filterDiffCompact(raw)
	}

	res := SanitizeDiff(raw, offset, limit)
	return diffResultJSON(res), nil
}

// handleDiffStatCommand handles stats-only diff output.
func (s *Server) handleDiffStatCommand(arg string, cachedFlag ...string) (string, error) {
	var raw string
	var err error
	if len(cachedFlag) > 0 && cachedFlag[0] != "" {
		if arg != "" {
			raw, err = s.git.DiffStatStaged(arg)
		} else {
			raw, err = s.git.DiffStatStaged()
		}
	} else {
		if arg != "" {
			raw, err = s.git.DiffStat(arg)
		} else {
			raw, err = s.git.DiffStat()
		}
	}
	if err != nil {
		return "", err
	}
	return diffStatJSON(raw), nil
}

func diffStatJSON(raw string) string {
	files := 0
	additions := 0
	deletions := 0
	var fileList []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "-") {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) == 2 {
			files++
			fname := strings.TrimSpace(parts[0])
			fileList = append(fileList, fname)
			stats := strings.TrimSpace(parts[1])
			additions += strings.Count(stats, "+")
			deletions += strings.Count(stats, "-")
		}
	}
	return mustJSON(map[string]interface{}{
		"files":     files,
		"additions": additions,
		"deletions": deletions,
		"file_list": fileList,
	})
}

func (s *Server) handleWhatChangedCommand(filterArg string, filterMode WhatChangedFilter, useLLM bool) (string, error) {
	var stagedRaw, unstagedRaw string
	var err error

	switch filterMode {
	case WhatChangedFilterStaged:
		stagedRaw, err = s.git.DiffStaged()
		unstagedRaw = ""
	case WhatChangedFilterUnstaged:
		stagedRaw = ""
		unstagedRaw, err = s.git.Diff()
	case WhatChangedFilterAll, "":
		stagedRaw, err = s.git.DiffStaged()
		if err != nil {
			stagedRaw = ""
		}
		unstagedRaw, err = s.git.Diff()
		if err != nil {
			unstagedRaw = ""
		}
	default:
		return "", fmt.Errorf("invalid filter: %s (use: all, staged, unstaged)", filterMode)
	}
	if err != nil && stagedRaw == "" && unstagedRaw == "" {
		return "", err
	}

	statsRaw, err := s.git.DiffStat()
	if err != nil {
		statsRaw = ""
	}

	var summary string
	var llmUsed bool

	if useLLM && s.llm != nil {
		avail := s.llm.IsAvailable()
		if avail {
			diffRaw := stagedRaw
			if diffRaw != "" && unstagedRaw != "" {
				diffRaw += "\n" + unstagedRaw
			} else if unstagedRaw != "" {
				diffRaw = unstagedRaw
			}

			if diffRaw != "" {
				cleanDiff := filters.FilterDiffNoise(diffRaw)
				lines := strings.Split(cleanDiff, "\n")
				maxLines := 200
				if len(lines) > maxLines {
					cleanDiff = strings.Join(lines[:maxLines], "\n")
					cleanDiff += "\n... truncated"
				}

				done := make(chan string, 1)
				go func() {
					defer func() {
						if r := recover(); r != nil {
							done <- ""
						}
					}()
					chunk := domain.DiffChunk{
						Files: []string{},
						Diff:  cleanDiff,
					}
					result, llmErr := s.llm.GenerateChunkMessage(chunk)
					if llmErr == nil && result != "" {
						done <- extractSummary(result)
					} else {
						done <- ""
					}
				}()

				select {
				case summary = <-done:
					llmUsed = summary != ""
				case <-time.After(10 * time.Second):
					llmUsed = false
				}
			}
		}
	}

	return whatChangedJSONWithSummary(statsRaw, summary, string(filterMode), llmUsed), nil
}

func (s *Server) handleDiffAllCommand(arg string, limit, offset int, fileFilter string, compact bool) (string, error) {
	var stagedRaw, unstagedRaw string
	var err error
	if arg != "" {
		stagedRaw, err = s.git.DiffStaged(arg)
		if err != nil {
			stagedRaw = ""
		}
		unstagedRaw, err = s.git.Diff(arg)
		if err != nil {
			unstagedRaw = ""
		}
	} else {
		stagedRaw, err = s.git.DiffStaged()
		if err != nil {
			stagedRaw = ""
		}
		unstagedRaw, err = s.git.Diff()
		if err != nil {
			unstagedRaw = ""
		}
	}

	var sb strings.Builder
	if stagedRaw != "" {
		sb.WriteString(StagedMarker + "\n")
		sb.WriteString(stagedRaw)
		sb.WriteString("\n")
	}
	if sb.Len() > 0 && unstagedRaw != "" {
		sb.WriteString(UnstagedMarker + "\n")
	}
	if unstagedRaw != "" {
		sb.WriteString(unstagedRaw)
	}

	raw := sb.String()

	if fileFilter != "" {
		raw = filterDiffByFile(raw, fileFilter)
	}
	if compact {
		raw = filterDiffCompact(raw)
	}

	res := SanitizeDiff(raw, offset, limit)
	res.Mode = "all"
	return diffResultJSON(res), nil
}
