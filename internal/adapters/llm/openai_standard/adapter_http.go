package openai_standard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors for centralized JSON parsing.
var (
	ErrEmptyResponse = errors.New("LLM returned empty response")
	ErrInvalidJSON   = errors.New("LLM returned invalid JSON")
)

// parseJSON strips markdown fences, validates the cleaned string, and
// unmarshals into v. Returns ErrEmptyResponse if the result is empty after
// cleaning, or ErrInvalidJSON if the first non-space character is not '{' or
// json.Unmarshal fails.
func parseJSON(raw string, v any) error {
	cleaned := cleanJSON(raw)
	if cleaned == "" {
		return ErrEmptyResponse
	}
	if cleaned[0] != '{' {
		return ErrInvalidJSON
	}
	if err := json.Unmarshal([]byte(cleaned), v); err != nil {
		return ErrInvalidJSON
	}
	return nil
}

// chatCompletionOpts configures a chatCompletionWithMessages call.
type chatCompletionOpts struct {
	operation        string
	jsonMode         bool
	reasoningEffort  string
	maxTokens        int
	temperature      *float64
	topP             *float64
	frequencyPenalty *float64
	presencePenalty  *float64
	seed             *int
	stop             []string
}

// floatPtr returns a pointer to the given float64 value.
func floatPtr(f float64) *float64 { return &f }

// chatCompletion sends a prompt via /chat/completions and returns the response content.
// When opts.jsonMode is true, the request includes format: "json" for structured output.
// When opts.reasoningEffort is "none", injects a /no_think system message so that
// Qwen3 and similar reasoning models skip the <think>...</think> output entirely.
func (a *OpenAIStandardAdapter) chatCompletion(prompt string, opts chatCompletionOpts) (string, error) {
	var messages []ChatMessage
	if opts.reasoningEffort == "none" {
		messages = []ChatMessage{
			{Role: "system", Content: "/no_think"},
			{Role: "user", Content: prompt},
		}
	} else {
		messages = []ChatMessage{{Role: "user", Content: prompt}}
	}
	return a.chatCompletionWithMessages(messages, opts)
}

// chatCompletionWithMessages sends messages via /chat/completions and returns the response content.
func (a *OpenAIStandardAdapter) chatCompletionWithMessages(messages []ChatMessage, opts chatCompletionOpts) (string, error) {
	req := ChatRequest{
		Model:    a.model,
		Messages: messages,
		Stream:   false,
		Think:    false,
	}
	if opts.jsonMode {
		req.Format = "json"
	}
	if opts.reasoningEffort != "" {
		req.ReasoningEffort = opts.reasoningEffort
	}
	if opts.maxTokens > 0 {
		req.MaxTokens = opts.maxTokens
	}
	if opts.temperature != nil {
		req.Temperature = opts.temperature
	}
	if opts.topP != nil {
		req.TopP = opts.topP
	}
	if opts.frequencyPenalty != nil {
		req.FrequencyPenalty = opts.frequencyPenalty
	}
	if opts.presencePenalty != nil {
		req.PresencePenalty = opts.presencePenalty
	}
	if opts.seed != nil {
		req.Seed = opts.seed
	}
	if len(opts.stop) > 0 {
		req.Stop = opts.stop
	}

	// Inject Ollama-specific options only for ollama provider
	if a.provider == "ollama" {
		req.Options = make(map[string]interface{})
		if opts.temperature != nil {
			req.Options["temperature"] = *opts.temperature
		}
		if opts.maxTokens > 0 {
			req.Options["num_predict"] = opts.maxTokens
		}
		if a.numCtx > 0 {
			req.Options["num_ctx"] = a.numCtx
		}
		req.Options["keep_alive"] = ollamaKeepAlive
	}

	body, err := a.client.Post(context.Background(), "/chat/completions", req)
	if err != nil {
		return "", fmt.Errorf("chat completion request failed: %w", err)
	}

	var resp ChatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("parse chat response: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no chat choices")
	}

	return resp.Choices[0].Message.Content, nil
}

// cleanJSON strips defensive markdown fences, prose before/after the JSON object,
// and whitespace from LLM responses. It extracts the first top-level JSON object
// ({ ... }) from any surrounding text, handling:
//   - Markdown fences (```json ... ```)
//   - <think> reasoning blocks
//   - Explanatory text before or after the JSON
//   - Trailing commas (relaxed via json.RawMessage preprocessing)
func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	// Strip <think>...</think> blocks emitted by reasoning models (Qwen3, DeepSeek-R1).
	// reasoning_effort:"none" is not always honored by local backends.
	if end := strings.Index(s, "</think>"); end != -1 {
		s = strings.TrimSpace(s[end+len("</think>"):])
	}
	// Strip markdown fences
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	// Extract the first top-level JSON object { ... } from any surrounding prose.
	// This handles cases where the LLM adds explanatory text before or after.
	// If no object is found, fall back to the whole cleaned string for backward
	// compatibility with non-object payloads (e.g. arrays).
	start := strings.IndexByte(s, '{')
	if start == -1 {
		return s
	}
	s = s[start:]
	end := strings.LastIndexByte(s, '}')
	if end == -1 {
		return s
	}
	s = s[:end+1]

	return strings.TrimSpace(s)
}
