// Package openai_standard implements the OpenAI-compatible API adapter.
// It handles /v1/chat/completions and /v1/completions endpoints
// used by Ollama, llama.cpp, LM Studio, and other local backends.
package openai_standard

// ChatMessage represents a message in a chat conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is the request body for /v1/chat/completions.
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Format      string        `json:"format,omitempty"` // "json" or "json_object" for JSON mode
	Stream      bool          `json:"stream"`
}

// ChatResponse is the response body from /v1/chat/completions.
type ChatResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// CompletionRequest is the request body for /v1/completions.
type CompletionRequest struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Stream      bool    `json:"stream"`
}

// CompletionResponse is the response body from /v1/completions.
type CompletionResponse struct {
	Choices []struct {
		Text string `json:"text"`
	} `json:"choices"`
}