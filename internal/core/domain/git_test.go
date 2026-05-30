package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReleaseIntent_CustomTagMessageJSON(t *testing.T) {
	t.Run("serializes custom_tag_message", func(t *testing.T) {
		intent := ReleaseIntent{
			TagName:          "v1.0.0",
			CustomTagMessage: "custom release message",
		}
		data, err := json.Marshal(intent)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}
		if !strings.Contains(string(data), `"custom_tag_message"`) {
			t.Errorf("expected custom_tag_message in JSON, got: %s", string(data))
		}
	})

	t.Run("deserializes custom_tag_message", func(t *testing.T) {
		jsonData := `{"tag_name":"v1.0.0","custom_tag_message":"my custom msg"}`
		var intent ReleaseIntent
		if err := json.Unmarshal([]byte(jsonData), &intent); err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}
		if intent.CustomTagMessage != "my custom msg" {
			t.Errorf("CustomTagMessage = %q, want %q", intent.CustomTagMessage, "my custom msg")
		}
	})

	t.Run("omits custom_tag_message when empty", func(t *testing.T) {
		intent := ReleaseIntent{
			TagName: "v1.0.0",
		}
		data, err := json.Marshal(intent)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}
		if strings.Contains(string(data), `"custom_tag_message"`) {
			t.Errorf("expected custom_tag_message to be omitted when empty with omitempty, got: %s", string(data))
		}
	})
}
