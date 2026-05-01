//go:build e2e

package e2e

import (
	"net/http"
	"testing"
)

func TestConnection(t *testing.T) {
	resp, err := http.Get("http://localhost:11434/v1/models")
	if err != nil {
		t.Fatalf("Failed to connect to Ollama: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Unexpected status code: %d", resp.StatusCode)
	}
}
