//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/llm/openai_standard"
	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/shared/prompts"
)

func main() {
	ctx := context.Background()

	// Cargar config
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	llmCfg, err := cfg.ResolveLLMConfig()
	if err != nil {
		fmt.Printf("Error resolving LLM config: %v\n", err)
		os.Exit(1)
	}

	// Crear adapter
	adapter, err := openai_standard.NewAdapter(llmCfg)
	if err != nil {
		fmt.Printf("Error creating adapter: %v\n", err)
		os.Exit(1)
	}

	// Datos de prueba
	files := []string{"main.go", "auth.go"}
	diff := `diff --git a/main.go b/main.go
index 1234567..abcdefg 100644
--- a/main.go
+++ b/main.go
@@ -10,6 +10,8 @@ func main() {
+   fmt.Println("Hello World")
+   log.Println("Application started")
 }`
	contextStr := "" // Context is now per-project (ProjectConfig), not global

	fmt.Println("=== CONTEXTO INYECTADO ===")
	fmt.Printf("Context: %s\n\n", contextStr)

	// Probar con el NUEVO prompt (usando el sistema real)
	fmt.Println("=== PRUEBA CON PROMPT NUEVO ===")
	params := prompts.MessageParams{
		CurrentBranch:   "main",
		Files:           strings.Join(files, ", "),
		Diff:            diff,
		RejectedMessage: "",
		Context:         contextStr,
	}

	// Renderizar el prompt para verlo
	newPrompt, err := prompts.RenderOp("commit_message", params)
	if err != nil {
		fmt.Printf("Error rendering prompt: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Prompt que se envía:\n%s\n\n", newPrompt)

	// Enviar al LLM
	fmt.Println("Enviando al LLM...")
	chunk := struct {
		Files []string
		Diff  string
	}{
		Files: files,
		Diff:  diff,
	}

	// Necesitamos usar el adapter con contexto
	adapter.SetContext(contextStr)
	result, err := adapter.GenerateChunkMessage(domainChunk{Files: files, Diff: diff})
	if err != nil {
		fmt.Printf("Error from LLM: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== RESPUESTA DEL LLM (PROMPT NUEVO) ===")
	jsonResult, _ := json.MarshalIndent(result, "", "  ")
	fmt.Printf("%s\n", jsonResult)

	_ = ctx
}

type domainChunk struct {
	Files []string
	Diff  string
}
