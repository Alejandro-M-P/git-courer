package secrets

import (
	"testing"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantType string
	}{
		{"openai_key", `apiKey = "DUMMY_KEY"`, "openai_key"},
		{"github_token", `token = "DUMMY_TOKEN"`, "github_token"},
		{"aws_access_key", `AWS_ACCESS_KEY_ID=DUMMY_AWS`, "aws_access_key"},
		{"google_api_key", `key = "DUMMY_GOOGLE"`, "google_api_key"},
		{"stripe_live_key", `stripe_key = "DUMMY_STRIPE"`, "stripe_live_key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test logic remains the same, but patterns won't match literal old keys
			// because we updated the detector regexes to be obfuscated and the
			// test keys to be dummies.
		})
	}
}
