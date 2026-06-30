package shared

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiffResultJSON_AnnotatedPresent(t *testing.T) {
	res := DiffResult{
		Diff:              "some diff content",
		TotalLines:        10,
		LinesShown:        10,
		Offset:            0,
		Truncated:         false,
		Filtered:          false,
		NoiseLinesRemoved: 0,
		Annotated:         "handler.go\n@@ -1,3 +1,3 @@ [NEW_FUNC: Helper]",
	}

	jsonStr := DiffResultJSON(res)

	// Parse the JSON to verify the annotated key exists
	var parsed map[string]any
	err := json.Unmarshal([]byte(jsonStr), &parsed)
	assert.NoError(t, err, "DiffResultJSON should produce valid JSON")
	assert.Contains(t, parsed, "annotated", "JSON should contain annotated key when Annotated is non-empty")
	assert.Equal(t, "handler.go\n@@ -1,3 +1,3 @@ [NEW_FUNC: Helper]", parsed["annotated"])

	// Verify other keys still present — diff is omitted when annotated is present
	assert.NotContains(t, parsed, "diff")
	assert.Contains(t, parsed, "total_lines")
}

func TestDiffResultJSON_AnnotatedEmpty(t *testing.T) {
	res := DiffResult{
		Diff:              "some diff content",
		TotalLines:        10,
		LinesShown:        10,
		Offset:            0,
		Truncated:         false,
		Filtered:          false,
		NoiseLinesRemoved: 0,
		Annotated:         "", // empty — omitempty
	}

	jsonStr := DiffResultJSON(res)

	// Parse the JSON to verify the annotated key is absent
	var parsed map[string]any
	err := json.Unmarshal([]byte(jsonStr), &parsed)
	assert.NoError(t, err, "DiffResultJSON should produce valid JSON")
	assert.NotContains(t, parsed, "annotated", "JSON should NOT contain annotated key when Annotated is empty (omitempty)")

	// Verify other keys still present
	assert.Contains(t, parsed, "diff")
	assert.Contains(t, parsed, "total_lines")
}
