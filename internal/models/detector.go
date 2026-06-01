// Package models provides context window resolution for LLM models
// through a 3-tier strategy: Ollama runtime detection, embedded catalog,
// and conservative default fallback.
package models

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// HTTPClient is the interface for making HTTP requests, enabling testability.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// OllamaDetector queries Ollama's /api/show endpoint for model context windows.
// Results are cached per-model. Lazy — no HTTP calls until first Lookup.
type OllamaDetector struct {
	baseURL string
	client  HTTPClient
	cache   sync.Map // model → int (context window)
}

// NewOllamaDetector creates an OllamaDetector with a custom base URL and HTTP client.
func NewOllamaDetector(baseURL string, client HTTPClient) *OllamaDetector {
	return &OllamaDetector{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  client,
	}
}

// NewDefaultOllamaDetector creates an OllamaDetector with default settings
// (http.DefaultClient, localhost:11434).
func NewDefaultOllamaDetector() *OllamaDetector {
	return &OllamaDetector{
		baseURL: "http://localhost:11434",
		client:  http.DefaultClient,
	}
}

// ollamaShowRequest is the request body for /api/show.
type ollamaShowRequest struct {
	Model string `json:"model"`
}

// ollamaShowResponse is the response body from /api/show.
type ollamaShowResponse struct {
	ModelInfo map[string]any `json:"model_info"`
}

// Lookup queries Ollama for the context window of a given model.
// Returns the context window size and true if found, or (0, false) on any error.
// Results are cached; subsequent calls for the same model skip the HTTP request.
func (d *OllamaDetector) Lookup(ctx context.Context, model string) (int, bool) {
	// Check cache first
	if cached, ok := d.cache.Load(model); ok {
		return cached.(int), true
	}

	// Create request with 30-second timeout
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	reqBody := ollamaShowRequest{Model: model}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("[WARN] Ollama lookup: failed to marshal request body for model %s: %v", model, err)
		return 0, false
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, d.baseURL+"/api/show", strings.NewReader(string(bodyBytes)))
	if err != nil {
		log.Printf("[WARN] Ollama lookup: failed to create request for model %s: %v", model, err)
		return 0, false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		log.Printf("[WARN] Ollama lookup: HTTP request failed for model %s: %v", model, err)
		return 0, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[WARN] Ollama lookup: endpoint returned status %s for model %s", resp.Status, model)
		return 0, false
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[WARN] Ollama lookup: failed to read response body for model %s: %v", model, err)
		return 0, false
	}

	var showResp ollamaShowResponse
	if err := json.Unmarshal(respBytes, &showResp); err != nil {
		log.Printf("[WARN] Ollama lookup: failed to unmarshal response for model %s: %v", model, err)
		return 0, false
	}

	contextWindow := extractContextLength(showResp.ModelInfo)
	if contextWindow == 0 {
		log.Printf("[WARN] Ollama lookup: model_info does not contain context length for model %s", model)
		return 0, false
	}

	// Cache the result
	d.cache.Store(model, contextWindow)
	return contextWindow, true
}

// extractContextLength extracts the context length from Ollama model_info.
// It looks for the pattern <architecture>.context_length, where architecture
// comes from general.architecture. Falls back to "context_length" directly.
func extractContextLength(modelInfo map[string]any) int {
	if modelInfo == nil {
		return 0
	}

	// Get architecture name
	archVal, ok := modelInfo["general.architecture"]
	if !ok {
		// Fallback: try direct context_length key
		return getContextLengthDirect(modelInfo)
	}

	arch, ok := archVal.(string)
	if !ok {
		return getContextLengthDirect(modelInfo)
	}

	// Look for <architecture>.context_length
	key := fmt.Sprintf("%s.context_length", arch)
	if ctxLen, ok := modelInfo[key]; ok {
		switch v := ctxLen.(type) {
		case float64:
			return int(v)
		case int:
			return v
		case json.Number:
			n, err := v.Int64()
			if err != nil {
				return 0
			}
			return int(n)
		}
	}

	// Fallback: try direct context_length key
	return getContextLengthDirect(modelInfo)
}

// getContextLengthDirect tries to find a "context_length" key directly in model_info.
func getContextLengthDirect(modelInfo map[string]any) int {
	// Try common direct keys
	for _, key := range []string{"context_length", "general.context_length"} {
		if ctxLen, ok := modelInfo[key]; ok {
			switch v := ctxLen.(type) {
			case float64:
				return int(v)
			case int:
				return v
			case json.Number:
				n, err := v.Int64()
				if err != nil {
					return 0
				}
				return int(n)
			}
		}
	}
	return 0
}
