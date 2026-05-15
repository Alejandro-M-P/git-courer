package mcp

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

func TestFormatBackupListJSON_WithUndoable(t *testing.T) {
	backups := []domain.Backup{
		{
			Ref:       "ref-local",
			Operation: "commit",
			CreatedAt: time.Now(),
			Undoable:  true,
		},
		{
			Ref:       "ref-remote",
			Operation: "push",
			CreatedAt: time.Now(),
			Undoable:  false,
		},
	}

	result := formatBackupListJSON(backups)
	var parsed map[string]any
	err := json.Unmarshal([]byte(result), &parsed)
	assert.NoError(t, err)

	items, ok := parsed["backups"].([]any)
	assert.True(t, ok, "backups should be an array")
	assert.Len(t, items, 2)

	first := items[0].(map[string]any)
	assert.Equal(t, true, first["undoable"])

	second := items[1].(map[string]any)
	assert.Equal(t, false, second["undoable"])
}
