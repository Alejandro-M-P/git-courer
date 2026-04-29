// Package ollama implements the Ollama-specific adapter wrapping the
// OpenAI-compatible standard adapter. It adds lifecycle management
// (auto-start, pre-warm), model resolution via /api/tags, and JSON
// support detection via /api/generate.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ModelResolver checks model availability via /api/tags and detects
// JSON support via /api/generate. These are Ollama-specific endpoints
// (NOT /v1/) because only the native Ollama API exposes model listing
// and the format:json parameter.
type ModelResolver struct {
	host         string
	model        string
	SupportsJSON bool
	httpClient   *http.Client
	onWarm       func() // called after DetectJSONSupport succeeds
}

// ResolverOption is a functional option for ModelResolver configuration.
type ResolverOption func(*ModelResolver)

// WithResolverHTTPClient sets a custom HTTP client (useful for testing).
func WithResolverHTTPClient(client *http.Client) ResolverOption {
	return func(mr *ModelResolver) { mr.httpClient = client }
}

// WithResolverOnWarm sets a callback invoked after DetectJSONSupport succeeds.
func WithResolverOnWarm(fn func()) ResolverOption {
	return func(mr *ModelResolver) { mr.onWarm = fn }
}

// NewModelResolver creates a new ModelResolver for the given Ollama host and model.
func NewModelResolver(host, model string, opts ...ResolverOption) *ModelResolver {
	mr := &ModelResolver{
		host:       strings.TrimRight(host, "/"),
		model:      model,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	for _, opt := range opts {
		opt(mr)
	}
	return mr
}

// Model returns the resolved model name.
func (mr *ModelResolver) Model() string {
	return mr.model
}

// Resolve checks if the configured model is available in Ollama using /api/tags.
// If the model is not found but other models exist, falls back to the first available.
// After resolving, detects JSON support.
func (mr *ModelResolver) Resolve() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", mr.host+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("failed to check Ollama models: %w", err)
	}

	resp, err := mr.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Ollama not available: %w", err)
	}
	defer resp.Body.Close()

	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return fmt.Errorf("failed to parse Ollama tags: %w", err)
	}

	for _, m := range tags.Models {
		if m.Name == mr.model || m.Name == mr.model+":latest" {
			mr.model = m.Name // Use exact name from Ollama (may include :latest)
			mr.SupportsJSON = mr.DetectJSONSupport()
			if mr.onWarm != nil {
				mr.onWarm()
			}
			return nil
		}
	}

	if len(tags.Models) > 0 {
		oldModel := mr.model
		mr.model = tags.Models[0].Name
		fmt.Printf("⚠ Model %q not found. Using %q instead.\n", oldModel, mr.model)
		mr.SupportsJSON = mr.DetectJSONSupport()
		if mr.onWarm != nil {
			mr.onWarm()
		}
		return nil
	}

	return fmt.Errorf("no models available in Ollama. Pull one with: ollama pull qwen3.5:latest")
}

// DetectJSONSupport tests if the model responds correctly to format:json
// in the Ollama /api/generate endpoint. Returns true if the model produces
// valid JSON output.
func (mr *ModelResolver) DetectJSONSupport() bool {
	testPrompt := `responde solo con JSON: {"ok": true}`

	reqBody := map[string]interface{}{
		"model":  mr.model,
		"prompt": testPrompt,
		"stream": false,
		"format": "json",
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", mr.host+"/api/generate", bytes.NewBuffer(jsonBody))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := mr.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	var response struct {
		Response string `json:"response"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return false
	}

	result := strings.TrimSpace(response.Response)
	if result == "" {
		return false
	}

	// Try to parse as JSON
	var js map[string]interface{}
	if err := json.Unmarshal([]byte(result), &js); err != nil {
		return false
	}

	fmt.Printf("[JSON Detect] ✅ Model %q supports format:json\n", mr.model)
	return true
}