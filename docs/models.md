# Model Guide

> **Without a configured LLM backend, git-courer cannot generate commit message bodies, changelogs, or run the security AI auditor.** Basic read operations (status, diff, log) still work, and commit messages can still be written manually by the agent (when `llm.enabled` is `false`).

git-courer integrates with multiple LLM backends through a unified OpenAI-compatible completions interface.

---

## The Role of the LLM vs. Go AST Analysis

A core architectural principle of git-courer is that **the commit type (e.g., `feat`, `fix`, `refactor`, `test`) is determined deterministically in Go by parsing the AST**, not by the LLM. 

- **Go AST Analysis**: Deterministically classifies the commit type, detects breaking changes in signature contracts, and groups changes by dependency graph.
- **LLM Role**: Only responsible for writing the human-readable explanation (the *WHY* and *WHAT* of the commit message), generating changelogs during a release, and acting as the final paranoid security auditor layer.

---

## Context Window Resolution (3-Tier Cascade)

At startup or install time, git-courer dynamically resolves the target model's context window size to ensure prompt payloads fit within the LLM limits. It follows a 3-tier cascade strategy:

1. **Ollama Lookup**: If using Ollama, git-courer calls the `/api/show` endpoint to dynamically read `<architecture>.context_length` or `context_length` from the active local model.
2. **User Config Override**: If the Ollama lookup fails or a remote provider is used, it reads the `llm.context_window` setting from `~/.config/git-courer/config.yaml`.
3. **Default Fallback**: Falls back to a safe default of `8192` tokens if no other context size is resolved.

---

## Model Recommendations

While git-courer prompts are optimized to work on any model size, performance and description quality scale with model capability:

| Model | Recommended Use Case | Commit Description Quality | Security Auditor Capability |
|-------|----------------------|---------------------------|----------------------------|
| **gemma4:26b** (or similar) | High-performance workstation | Elite (Excellent context & details) | Highly Paranoid |
| **qwen3.5:latest** (7b) | Standard Developer Laptop | Very Good (Accurate, concise) | Reliable |
| **qwen3.5:0.8b** (or similar) | Modest laptops / No GPU | Basic (Requires offline toggle verification) | Limited (Fallback to regex is safer) |

---

## Configuration Examples

### 1. Ollama (Default Local Provider)
Ollama is auto-discovered and managed by git-courer.
```yaml
llm:
  enabled: true
  provider: ollama
  model: qwen3.5:latest
  base_url: http://localhost:11434/v1
```

### 2. LM Studio
1. Start LM Studio and load your model.
2. Enable the local server (default: `http://localhost:1234/v1`).
3. Set the config:
```yaml
llm:
  provider: lmstudio
  base_url: http://localhost:1234/v1
  model: my-model
```

### 3. vLLM
1. Start the vLLM OpenAI-compatible server:
   ```bash
   python -m vllm.entrypoints.openai.api_server --model my-model
   ```
2. Configure git-courer:
```yaml
llm:
  provider: vllm
  base_url: http://localhost:8000/v1
  model: my-model
```

### 4. LocalAI
1. Start LocalAI with your models.
2. Configure git-courer:
```yaml
llm:
  provider: localai
  base_url: http://localhost:8080/v1
  model: my-model
```

### 5. Standard OpenAI-Compatible Server
Any endpoint exposing the standard `/v1/chat/completions` API is supported:
```yaml
llm:
  provider: openai
  base_url: https://api.openai.com/v1
  model: gpt-4o-mini
  api_key: sk-proj-...  # Optional API key if required by your endpoint
  num_parallel: 1
```
