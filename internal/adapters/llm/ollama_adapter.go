package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/domain/models"
)

// Adapter implements the LLM Port interface using Ollama
type Adapter struct {
	host  string
	model string
}

// NewAdapter creates a new Ollama adapter
func NewAdapter(host string, model string) *Adapter {
	if host == "" {
		host = "http://localhost:11434"
	}
	if model == "" {
		model = "llama3.2"
	}
	return &Adapter{
		host:  host,
		model: model,
	}
}

// GenerateCommitMessage generates a commit message from diff
func (o *Adapter) GenerateCommitMessage(diff string) (models.CommitMessage, error) {
	if diff == "" {
		return models.CommitMessage{}, fmt.Errorf("empty diff")
	}

	// Truncate diff if too long (simple approach)
	maxLen := 8000
	if len(diff) > maxLen {
		diff = diff[:maxLen] + "\n... (truncated)"
	}

	prompt := fmt.Sprintf(`You are a git commit message generator. Generate a concise commit message following Conventional Commits format.

Rules:
- Start with type: feat, fix, chore, docs, style, refactor, test, perf, ci, build, revert
- Use imperative mood: "add" not "added" or "adds"
- Keep subject under 72 characters
- No period at end of subject

Diff to analyze:
%s

Generate only the commit message in this exact format:
type: subject`, diff)

	result, err := o.generate(prompt)
	if err != nil {
		return models.CommitMessage{}, err
	}

	// Parse result
	lines := strings.Split(strings.TrimSpace(result), ":")
	if len(lines) < 2 {
		return models.CommitMessage{
			Type:    "chore",
			Subject: strings.TrimSpace(result),
			Full:    strings.TrimSpace(result),
		}, nil
	}

	msg := models.CommitMessage{
		Type:    strings.TrimSpace(lines[0]),
		Subject: strings.TrimSpace(strings.Join(lines[1:], ":")),
		Full:    strings.TrimSpace(result),
	}

	// Default to chore if invalid type
	if msg.Type == "" {
		msg.Type = "chore"
	}

	return msg, nil
}

// GenerateSummary generates a human-readable summary of changes
func (o *Adapter) GenerateSummary(diff string) (string, error) {
	if diff == "" {
		return "No changes", nil
	}

	maxLen := 4000
	if len(diff) > maxLen {
		diff = diff[:maxLen] + "\n... (truncated)"
	}

	prompt := fmt.Sprintf(`Summarize these git changes in 2-3 sentences for a user:

%s

Keep it human-readable and concise.`, diff)

	return o.generate(prompt)
}

// GenerateBranchName suggests a branch name based on task
func (o *Adapter) GenerateBranchName(task string) (string, error) {
	if task == "" {
		task = "changes"
	}

	prompt := fmt.Sprintf(`Generate a short branch name (kebab-case) for this task:

%s

Rules:
- Use prefixes: feature/, fix/, chore/, docs/
- Keep under 50 characters
- Use kebab-case
- Be descriptive but concise

Only output the branch name, nothing else.`, task)

	result, err := o.generate(prompt)
	if err != nil {
		return "", err
	}

	// Clean up result
	result = strings.TrimSpace(result)
	result = strings.ToLower(result)

	// Ensure it starts with a valid prefix
	validPrefixes := []string{"feature/", "fix/", "chore/", "docs/", "refactor/", "test/"}
	hasPrefix := false
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(result, prefix) {
			hasPrefix = true
			break
		}
	}
	if !hasPrefix {
		result = "feature/" + result
	}

	return result, nil
}

// DetectSecrets checks if files contain secrets
func (o *Adapter) DetectSecrets(files []string) ([]models.SecretDetection, error) {
	if len(files) == 0 {
		return nil, nil
	}

	// This is a simplified version - in production you'd read files and analyze
	// For now, just return empty - will be enhanced later
	return []models.SecretDetection{}, nil
}

// IsAvailable checks if Ollama is running
func (o *Adapter) IsAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", o.host+"/api/tags", nil)
	if err != nil {
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

// generate sends a prompt to Ollama and returns the response
func (o *Adapter) generate(prompt string) (string, error) {
	reqBody := map[string]interface{}{
		"model":  o.model,
		"prompt": prompt,
		"stream": false,
		"options": map[string]interface{}{
			"temperature": 0.3,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", o.host+"/api/generate", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to connect to Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Ollama returned status %d", resp.StatusCode)
	}

	var response struct {
		Response string `json:"response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", err
	}

	return response.Response, nil
}

// Ensure Adapter implements the LLMPort interface
var _ models.LLMPort = (*Adapter)(nil)
