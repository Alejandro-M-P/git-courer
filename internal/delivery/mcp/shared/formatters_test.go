package shared

import (
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

func TestFormatStatusJSON(t *testing.T) {
	t.Run("formats status with files", func(t *testing.T) {
		status := domain.Status{
			Branch:  "main",
			Ahead:   2,
			Behind:  1,
			Files: []domain.FileStatus{
				{Path: "main.go", Status: "M", Staged: true},
				{Path: "README.md", Status: "A", Staged: false},
			},
			Staged:    1,
			Modified:  1,
			Untracked: 0,
			IsClean:   false,
		}

		result := FormatStatusJSON(status, 100, 0, "")
		assert.Contains(t, result, `"branch"`)
		assert.Contains(t, result, `"main"`)
		assert.Contains(t, result, `"main.go"`)
		assert.Contains(t, result, `"README.md"`)
		assert.Contains(t, result, `"staged"`)
		assert.Contains(t, result, `"files"`)
	})

	t.Run("formats status with filter", func(t *testing.T) {
		status := domain.Status{
			Branch: "feature",
			Files: []domain.FileStatus{
				{Path: "src/main.go", Status: "M", Staged: true},
				{Path: "docs/readme.md", Status: "A", Staged: false},
			},
		}

		result := FormatStatusJSON(status, 100, 0, "src")
		assert.Contains(t, result, `"src/main.go"`)
		assert.NotContains(t, result, `"docs/readme.md"`)
	})

	t.Run("formats status with pagination", func(t *testing.T) {
		files := make([]domain.FileStatus, 50)
		for i := range files {
			files[i] = domain.FileStatus{Path: "file.go", Status: "M", Staged: true}
		}
		status := domain.Status{
			Branch: "main",
			Files:  files,
		}

		result := FormatStatusJSON(status, 10, 0, "")
		assert.Contains(t, result, `"total":50`)
		assert.Contains(t, result, `"returned":10`)
	})

	t.Run("empty status returns valid JSON", func(t *testing.T) {
		status := domain.Status{Branch: "main", IsClean: true}
		result := FormatStatusJSON(status, 100, 0, "")
		assert.Contains(t, result, `"branch"`)
		assert.Contains(t, result, `"main"`)
		assert.Contains(t, result, `"clean":true`)
	})
}

func TestDiffResultJSON(t *testing.T) {
	t.Run("formats diff result with all fields", func(t *testing.T) {
		res := DiffResult{
			Diff:              "diff content here",
			TotalLines:        100,
			LinesShown:        50,
			Offset:            0,
			Truncated:         true,
			NextOffset:        50,
			Filtered:          false,
			NoiseLinesRemoved: 10,
			Mode:              "working_tree",
			Base:              "main",
			Target:            "feature",
		}

		result := DiffResultJSON(res)
		assert.Contains(t, result, `"diff"`)
		assert.Contains(t, result, `"total_lines":100`)
		assert.Contains(t, result, `"lines_shown":50`)
		assert.Contains(t, result, `"mode":"working_tree"`)
		assert.Contains(t, result, `"base":"main"`)
		assert.Contains(t, result, `"target":"feature"`)
	})

	t.Run("formats diff result with minimal fields", func(t *testing.T) {
		res := DiffResult{
			Diff:       "minimal diff",
			TotalLines: 10,
			LinesShown: 10,
		}

		result := DiffResultJSON(res)
		assert.Contains(t, result, `"diff"`)
		assert.Contains(t, result, `"total_lines":10`)
	})
}