package domain

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReleaseIntent_JSONSerialization(t *testing.T) {
	t.Run("serializes to JSON", func(t *testing.T) {
		intent := ReleaseIntent{
			TagName:     "v1.0.0",
			IsRelease:   true,
			VersionBump: "minor",
		}
		data, err := json.Marshal(intent)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}
		if !strings.Contains(string(data), `"tag_name"`) {
			t.Errorf("expected tag_name in JSON, got: %s", string(data))
		}
	})

	t.Run("deserializes from JSON", func(t *testing.T) {
		jsonData := `{"tag_name":"v1.0.0","is_release":true,"version_bump":"patch"}`
		var intent ReleaseIntent
		if err := json.Unmarshal([]byte(jsonData), &intent); err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}
		if intent.TagName != "v1.0.0" {
			t.Errorf("TagName = %q, want v1.0.0", intent.TagName)
		}
		if !intent.IsRelease {
			t.Error("IsRelease should be true")
		}
		if intent.VersionBump != "patch" {
			t.Errorf("VersionBump = %q, want patch", intent.VersionBump)
		}
	})
}

// TestStatus_AheadBehind_NullableJSON verifies the *int contract for
// Status.Ahead/Behind: nil MUST serialize as JSON null (not omitted, not 0),
// and a concrete value MUST serialize as a number. This is the unborn-repo
// signal contract — see spec delta "Status ahead/behind on unborn or
// upstream-less repos".
func TestStatus_AheadBehind_NullableJSON(t *testing.T) {
	t.Run("nil Ahead/Behind serialize as null", func(t *testing.T) {
		s := Status{Branch: "main"}
		data, err := json.Marshal(s)
		assert.NoError(t, err)
		jsonStr := string(data)
		// null is the explicit signal — NOT omitted, NOT 0.
		assert.Contains(t, jsonStr, `"ahead":null`, "nil Ahead must be null, not omitted")
		assert.Contains(t, jsonStr, `"behind":null`, "nil Behind must be null, not omitted")
	})

	t.Run("set Ahead/Behind serialize as numbers", func(t *testing.T) {
		ahead, behind := 3, 1
		s := Status{Branch: "main", Ahead: &ahead, Behind: &behind}
		data, err := json.Marshal(s)
		assert.NoError(t, err)
		jsonStr := string(data)
		assert.Contains(t, jsonStr, `"ahead":3`)
		assert.Contains(t, jsonStr, `"behind":1`)
	})

	t.Run("deserializes null back to nil", func(t *testing.T) {
		jsonData := `{"is_clean":false,"repo_path":"","branch":"main","ahead":null,"behind":null,"has_upstream":false,"files":null,"staged_count":0,"modified_count":0,"untracked_count":0,"conflicted_count":0}`
		var s Status
		assert.NoError(t, json.Unmarshal([]byte(jsonData), &s))
		assert.Nil(t, s.Ahead, "null must deserialize to nil")
		assert.Nil(t, s.Behind, "null must deserialize to nil")
	})
}

// requires import of assert (added above)
