package debugprompt

import (
	"fmt"
	"os"

	"github.com/Alejandro-M-P/git-courer/internal/shared/prompts"
)

func GenerateDebugPrompt() {
	// Test data
	instruction := "sacar versión"
	releases := "v0.1.0-alpha\nv0.1.0-beta\nv0.1.1-beta\nv0.1.2-beta\nv0.1.3-beta\nv0.1.4-beta"
	branches := "* feat/smart-tag-release\n  master\n  remotes/origin/HEAD -> origin/main\n  remotes/origin/bugfix\n  remotes/origin/develop\n  remotes/origin/feat/smart-tag-release\n  remotes/origin/feature/commit-analysis\n  remotes/origin/feature/secure-commit-preview\n  remotes/origin/main\n  remotes/origin/mcp_ai_functions_test\n  remotes/origin/refactor/architecture"
	currentBranch := "feat/smart-tag-release"

	// Get the prompt template
	promptTemplate := prompts.Get("release_interpret")
	fmt.Println("=== PROMPT TEMPLATE ===")
	fmt.Println(promptTemplate)
	fmt.Println()

	// Render the prompt
	prompt, err := prompts.Render(promptTemplate, map[string]string{
		"instruction":    instruction,
		"releases":       releases,
		"branches":       branches,
		"current_branch": currentBranch,
	})
	if err != nil {
		fmt.Printf("Error rendering prompt: %v\n", err)
		return
	}

	fmt.Println("=== RENDERED PROMPT ===")
	fmt.Println(prompt)
	fmt.Println()

	// Save to file for inspection
	err = os.WriteFile("/git-courer/debug_prompt.txt", []byte(prompt), 0644)
	if err != nil {
		fmt.Printf("Error writing prompt to file: %v\n", err)
	}
	fmt.Println("Prompt saved to /git-courer/debug_prompt.txt")
}
