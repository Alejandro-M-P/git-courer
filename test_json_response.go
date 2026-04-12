package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/llm"
)

func main() {
	// Create Ollama adapter
	llmAdapter := llm.New("http://localhost:11434", "qwen3.5:latest", "")

	// Test prompt that should return JSON
	prompt := `Responde SOLO con este JSON: {"test": "ok"}`

	// Call generateJSON
	response, _, _, err := llmAdapter.(*llm.Adapter).generateJSON(prompt)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Response: %q\n", response)

	// Try to parse as JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		fmt.Printf("Error parsing JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Parsed JSON: %+v\n", result)
}
