package installer

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/models"
)

var liteLLMURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"

// ResolveContextWindow determines model context window via cascade:
// 1. LiteLLM model_prices_and_context_window.json (GET)
// 2. Ollama /api/show (POST, using baseURL)
// 3. Conservative default 8192
func ResolveContextWindow(model, baseURL string) (int, error) {
	return resolveContextWindow(model, baseURL, http.DefaultClient, liteLLMURL), nil
}

func resolveContextWindow(model, baseURL string, client *http.Client, liteURL string) int {
	// Tier 1: Ollama (Local truth)
	// Querying Ollama first ensures we get the real context length of the model
	// as reported by the user's actual runtime.
	detector := models.NewOllamaDetector(baseURL, client)
	if window, ok := detector.Lookup(context.Background(), model); ok && window > 0 {
		return window
	}

	// Tier 2: LiteLLM (Global registry)
	// If Ollama is not available or doesn't know the model, check the registry.
	if window := queryLiteLLM(model, client, liteURL); window > 0 {
		return window
	}

	// Tier 3: Default (Safe fallback)
	return 8192
}

type liteLLMModel struct {
	MaxInputTokens int `json:"max_input_tokens"`
}

func queryLiteLLM(model string, client *http.Client, url string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Printf("[WARN] LiteLLM query: failed to create request: %v", err)
		return 0
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[WARN] LiteLLM query: HTTP request failed: %v", err)
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[WARN] LiteLLM query: endpoint returned status %s", resp.Status)
		return 0
	}

	var data map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		log.Printf("[WARN] LiteLLM query: failed to decode response: %v", err)
		return 0
	}

	raw, ok := data[model]
	if !ok {
		log.Printf("[WARN] LiteLLM query: model %s not found in registry", model)
		return 0
	}

	var m liteLLMModel
	if err := json.Unmarshal(raw, &m); err != nil {
		log.Printf("[WARN] LiteLLM query: failed to unmarshal model detail: %v", err)
		return 0
	}

	return m.MaxInputTokens
}
