package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/llm"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/shared/prompts"
)

func main() {
	// Create Ollama adapter
	llmAdapter := llm.New("http://localhost:11434", "qwen3.5:latest", "")

	// Test data
	instruction := "sacar versión"
	releases := "v0.1.0-alpha\nv0.1.0-beta\nv0.1.1-beta\nv0.1.2-beta\nv0.1.3-beta\nv0.1.4-beta"
	branches := "* feat/smart-tag-release\n  master\n  remotes/origin/HEAD -> origin/main\n  remotes/origin/bugfix\n  remotes/origin/develop\n  remotes/origin/feat/smart-tag-release\n  remotes/origin/feature/commit-analysis\n  remotes/origin/feature/secure-commit-preview\n  remotes/origin/main\n  remotes/origin/mcp_ai_functions_test\n  remotes/origin/refactor/architecture"
	currentBranch := "feat/smart-tag-release"

	fmt.Println("Calling InterpretReleaseIntent...")
	intent, err := llmAdapter.InterpretReleaseIntent(instruction, releases, branches, currentBranch)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("SUCCESS: %+v\n", intent)
}
