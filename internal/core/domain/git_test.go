package domain

import (
	"encoding/json"
	"strings"
	"testing"
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
