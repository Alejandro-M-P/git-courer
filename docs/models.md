# Model Guide

> **Without a configured LLM backend, git-courer cannot generate commit message bodies, changelogs, or run the security AI auditor.** Basic read operations (status, diff, log) still work, and commit messages can still be written manually by the agent (when `llm.enabled` is `false`).

git-courer integrates with multiple LLM backends through a unified OpenAI-compatible completions interface.

---

## The Role of the LLM vs. Go AST Analysis

A core architectural principle of git-courer is that **the commit type (e.g., `feat`, `fix`, `refactor`, `test`) is determined deterministically in Go by parsing the AST**, not by the LLM. 

- **Go AST Analysis**: Deterministically classifies the commit type, detects breaking changes in signature contracts, and groups changes by dependency graph.
- **LLM Role**: Only responsible for writing the human-readable explanation (the *WHY* and *WHAT* of the commit message), generating changelogs during a release, and acting as the final paranoid security auditor layer.

---

## `llm.enabled: false` — Operating Without an LLM

Setting `llm.enabled: false` disables all AI features. In this mode:

- **Commit type detection still works** — it is Go AST-based, not LLM-based.
- **Commit message bodies are not generated** — the agent/user writes them manually.
- **Changelog generation is unavailable** — releases fall back to a tag-only flow.
- **Security audit falls back to the non-LLM layers** — binary, blacklist, static analysis, and regex still run, but there is no AI override of regex false positives (see `docs/security-architecture.md`).
- **`Validate()` skips mandatory-field checks** — when `enabled` is false, `provider` and `model` are not required.

This is useful for air-gapped environments or when you want the deterministic guarantees of the Go classifier without an LLM dependency.

---

## Context Window Resolution (3-Tier Cascade)

At startup or install time, git-courer dynamically resolves the target model's context window size to ensure prompt payloads fit within the LLM limits. It follows a 3-tier cascade strategy:

1. **Ollama Lookup**: If using Ollama, git-courer calls the `/api/show` endpoint to dynamically read `<architecture>.context_length` or `context_length` from the active local model.
2. **User Config Override**: If the Ollama lookup fails or a remote provider is used, it reads the `llm.context_window` setting from `~/.config/git-courer/config.yaml`. A value of `0` (the default) means "resolve automatically"; any value `> 0` is used verbatim.
3. **Default Fallback**: Falls back to a safe default of `8192` tokens if no other context size is resolved.

---

## Choosing a Model

git-courer's prompts are tuned to work across model sizes, but quality scales with model capability. **There is no single recommended model** — performance depends on your hardware, quantization, and the kind of changes you commit. Instead of trusting a generic table, **benchmark on your own setup**:

1. Pull a candidate model (`ollama pull <model>`).
2. Run `make test-e2e` against it.
3. Inspect the generated commit messages and changelog entries for accuracy.
4. Compare against another model on the same diff.

Smaller models (`< 7B`) tend to need the strict-classification prompt path and may produce terser messages; larger models (`14B+`) handle deeper semantic context. The security auditor's `ParseModelSize` uses these same thresholds to select its prompt strategy (see `docs/security-architecture.md`).

---

## Configuration Examples

### 1. Ollama (Default Local Provider)
Ollama is auto-discovered and managed by git-courer.
```yaml
llm:
  enabled: true
  provider: ollama
  model: llama3.2:latest
  base_url: http://localhost:11434/v1
  num_parallel: 1
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
  num_parallel: 1
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
  num_parallel: 1
```

### 4. LocalAI
1. Start LocalAI with your models.
2. Configure git-courer:
```yaml
llm:
  provider: localai
  base_url: http://localhost:8080/v1
  model: my-model
  num_parallel: 1
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

---

## Field Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `llm.enabled` | bool | `true` | Master switch for all AI features. `false` disables LLM generation, changelog, and the AI security auditor (deterministic Go classification still works). |
| `llm.provider` | string | *(mandatory)* | Provider key: `ollama`, `lmstudio`, `vllm`, `localai`, `openai`, or any OpenAI-compatible key. |
| `llm.model` | string | *(mandatory)* | Model name as the provider expects it (e.g. `llama3.2:latest`, `gpt-4o-mini`). Parsed by `ParseModelSize` for prompt-strategy selection. |
| `llm.base_url` | string | `http://localhost:11434/v1` | OpenAI-compatible completions endpoint. Required for non-Ollama providers. |
| `llm.context_window` | int | `0` | `0` = resolve automatically (Ollama API → fallback `8192`). `>0` = use this value verbatim. |
| `llm.num_parallel` | int | `1` | Number of concurrent LLM requests the adapter may issue (e.g. for parallel commit-message generation). Raise this on providers that accept higher concurrency; `1` is the safe default for local single-model setups. |
| `llm.api_key` | string | *(none)* | **Adapter-level, not a config struct field.** Shown in the OpenAI example above for convenience; it is passed at adapter creation (`FactoryConfig.APIKey`), not read from `LLMConfig`. Omit for local providers that do not require auth. |

> **`api_key` note**: `LLMConfig` does not have an `api_key` YAML field. The key is supplied at adapter-creation time via `FactoryConfig.APIKey`. The `api_key:` line in the OpenAI example above is consumed during setup wiring, not parsed from the global config file.