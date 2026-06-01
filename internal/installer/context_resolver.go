package installer

import (
	"context"
	"net/http"

	"github.com/blak0p/git-courer/internal/models"
)

// ResolveContextWindow determines model context window via cascade:
// 1. Ollama /api/show (POST, using baseURL)
// 2. Config llm.context_window (passed as configContextWindow)
// 3. Conservative default 8192
func ResolveContextWindow(model, baseURL string, configContextWindow int) (int, error) {
	return resolveContextWindow(model, baseURL, http.DefaultClient, configContextWindow), nil
}

func resolveContextWindow(model, baseURL string, client *http.Client, configContextWindow int) int {
	// Tier 1: Ollama (Local truth)
	// Querying Ollama first ensures we get the real context length of the model
	// as reported by the user's actual runtime.
	detector := models.NewOllamaDetector(baseURL, client)
	if window, ok := detector.Lookup(context.Background(), model); ok && window > 0 {
		return window
	}

	// Tier 2: Config (User-specified override)
	// If Ollama is unavailable or doesn't know the model, use user config.
	if configContextWindow > 0 {
		return configContextWindow
	}

	// Tier 3: Default (Safe fallback)
	return 8192
}
