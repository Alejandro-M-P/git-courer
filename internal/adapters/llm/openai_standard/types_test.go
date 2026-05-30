package openai_standard

import (
	"encoding/json"
	"testing"
)

func TestChatMessage_Marshal(t *testing.T) {
	msg := ChatMessage{Role: "user", Content: "Hello, world!"}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("ChatMessage marshal failed: %v", err)
	}
	expected := `{"role":"user","content":"Hello, world!"}`
	if string(data) != expected {
		t.Errorf("ChatMessage marshal: got %q, want %q", string(data), expected)
	}
}

func TestChatMessage_Unmarshal(t *testing.T) {
	raw := `{"role":"assistant","content":"Hi there!"}`
	var msg ChatMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("ChatMessage unmarshal failed: %v", err)
	}
	if msg.Role != "assistant" {
		t.Errorf("ChatMessage role: got %q, want %q", msg.Role, "assistant")
	}
	if msg.Content != "Hi there!" {
		t.Errorf("ChatMessage content: got %q, want %q", msg.Content, "Hi there!")
	}
}

func TestChatMessage_Roles(t *testing.T) {
	roles := []string{"system", "user", "assistant"}
	for _, role := range roles {
		msg := ChatMessage{Role: role, Content: "test"}
		if msg.Role != role {
			t.Errorf("ChatMessage role: got %q, want %q", msg.Role, role)
		}
	}
}

func TestChatRequest_Marshal(t *testing.T) {
	req := ChatRequest{
		Model:       "gpt-4",
		Messages:    []ChatMessage{{Role: "user", Content: "commit this"}},
		Temperature: floatPtr(0.3),
		MaxTokens:   1024,
		Stream:      false,
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("ChatRequest marshal failed: %v", err)
	}

	// Verify roundtrip
	var parsed ChatRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("ChatRequest roundtrip unmarshal failed: %v", err)
	}
	if parsed.Model != "gpt-4" {
		t.Errorf("ChatRequest model: got %q, want %q", parsed.Model, "gpt-4")
	}
	if len(parsed.Messages) != 1 {
		t.Fatalf("ChatRequest messages: got %d, want 1", len(parsed.Messages))
	}
	if parsed.Messages[0].Role != "user" {
		t.Errorf("ChatRequest message role: got %q, want %q", parsed.Messages[0].Role, "user")
	}
	if parsed.Temperature == nil || *parsed.Temperature != 0.3 {
		t.Errorf("ChatRequest temperature: got %v, want 0.3", parsed.Temperature)
	}
	if parsed.MaxTokens != 1024 {
		t.Errorf("ChatRequest max_tokens: got %d, want 1024", parsed.MaxTokens)
	}
	if parsed.Stream != false {
		t.Errorf("ChatRequest stream: got %v, want false", parsed.Stream)
	}
}

func TestChatRequest_WithFormat(t *testing.T) {
	req := ChatRequest{
		Model:    "gpt-4",
		Messages: []ChatMessage{{Role: "user", Content: "test"}},
		Format:   "json",
		Stream:   false,
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("ChatRequest with format marshal failed: %v", err)
	}
	// Format should be present when set
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("ChatRequest format check unmarshal failed: %v", err)
	}
	if fmt, ok := parsed["format"]; !ok || fmt != "json" {
		t.Errorf("ChatRequest format: got %v, want %q", fmt, "json")
	}
}

func TestChatResponse_Unmarshal(t *testing.T) {
	raw := `{
		"choices": [
			{"message": {"role": "assistant", "content": "feat: add new feature"}}
		],
		"usage": {
			"prompt_tokens": 50,
			"total_tokens": 100
		}
	}`
	var resp ChatResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("ChatResponse unmarshal failed: %v", err)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("ChatResponse choices: got %d, want 1", len(resp.Choices))
	}
	if resp.Choices[0].Message.Role != "assistant" {
		t.Errorf("ChatResponse message role: got %q, want %q", resp.Choices[0].Message.Role, "assistant")
	}
	if resp.Choices[0].Message.Content != "feat: add new feature" {
		t.Errorf("ChatResponse message content: got %q, want %q", resp.Choices[0].Message.Content, "feat: add new feature")
	}
	if resp.Usage.PromptTokens != 50 {
		t.Errorf("ChatResponse prompt_tokens: got %d, want 50", resp.Usage.PromptTokens)
	}
	if resp.Usage.TotalTokens != 100 {
		t.Errorf("ChatResponse total_tokens: got %d, want 100", resp.Usage.TotalTokens)
	}
}

func TestChatRequest_OmitEmpty(t *testing.T) {
	// Verify that Temperature and MaxTokens are omitted when nil/zero
	req := ChatRequest{
		Model:    "test-model",
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
		Stream:   false,
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("ChatRequest omit empty marshal failed: %v", err)
	}
	// Temperature should NOT appear when nil (omitempty skips nil pointers)
	// MaxTokens should NOT appear when zero
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("ChatRequest omit check unmarshal failed: %v", err)
	}
	if _, ok := parsed["temperature"]; ok {
		t.Error("ChatRequest: temperature should be omitted when nil")
	}
	if _, ok := parsed["max_tokens"]; ok {
		t.Error("ChatRequest: max_tokens should be omitted when zero")
	}
	// Model and Messages and Stream should always be present
	if _, ok := parsed["model"]; !ok {
		t.Error("ChatRequest: model should always be present")
	}
	if _, ok := parsed["messages"]; !ok {
		t.Error("ChatRequest: messages should always be present")
	}
	if _, ok := parsed["stream"]; !ok {
		t.Error("ChatRequest: stream should always be present")
	}
}
