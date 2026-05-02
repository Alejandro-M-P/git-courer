// Package openai_standard implements the OpenAI-compatible API adapter.
// It handles the /v1/chat/completions endpoint
// used by Ollama, llama.cpp, LM Studio, and other local backends.
package openai_standard

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// ChatMessage represents a message in a chat conversation.
type ChatMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Reasoning string `json:"reasoning,omitempty"`
}

// ChatRequest is the request body for /v1/chat/completions.
type ChatRequest struct {
	Model            string                 `json:"model"`
	Messages         []ChatMessage          `json:"messages"`
	Temperature      *float64               `json:"temperature,omitempty"`
	MaxTokens        int                    `json:"max_tokens,omitempty"`
	TopP             *float64               `json:"top_p,omitempty"`
	FrequencyPenalty *float64               `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64               `json:"presence_penalty,omitempty"`
	Seed             *int                   `json:"seed,omitempty"`
	Stop             []string               `json:"stop,omitempty"`
	ReasoningEffort  string                 `json:"reasoning_effort,omitempty"`
	Think            bool                   `json:"think,omitempty"`
	Format           string                 `json:"format,omitempty"` // "json" or "json_object" for JSON mode
	Stream           bool                   `json:"stream"`
	Options          map[string]interface{} `json:"options,omitempty"`
}

// MarshalJSON implements custom marshaling to ensure `think: false` is always
// included in requests (not omitted by omitempty).
func (c ChatRequest) MarshalJSON() ([]byte, error) {
	type Alias ChatRequest // avoid recursion
	return json.Marshal(struct {
		Alias
		Think bool `json:"think"` // always emit, even when false
	}{
		Alias: (Alias)(c),
		Think: c.Think,
	})
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

// UnmarshalYAML implements custom unmarshaling for ChatRequest.
func (c *ChatRequest) UnmarshalYAML(node *yaml.Node) error {
	type Alias ChatRequest
	var a Alias
	if err := node.Decode(&a); err != nil {
		return err
	}
	*c = ChatRequest(a)
	return nil
}

// Validate checks the request for consistency.
func (c ChatRequest) Validate() error {
	if c.Model == "" {
		return fmt.Errorf("model is required")
	}
	return nil
}
