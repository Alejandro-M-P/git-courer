package config

// modelWindows maps known model identifiers to their context window sizes.
// These values are derived from model documentation and testing.
var modelWindows = map[string]int{
	"qwen3.5:0.8b": 32768,
	"llama3.1:8b":  8192,
	"llama3.1:70b": 128000,
	"gemma2:9b":    8192,
	"mistral:7b":   32768,
}

// DeriveContextWindow returns the context window size for a given model.
// If the model is not in the registry, it returns a conservative fallback of 4096.
func DeriveContextWindow(model string) int {
	if w, ok := modelWindows[model]; ok {
		return w
	}
	return 4096 // conservative fallback
}