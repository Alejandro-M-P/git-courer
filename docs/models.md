# Model Guide

git-courer works with multiple LLM backends: Ollama, LM Studio, vLLM, LocalAI, and any OpenAI-compatible server. Quality varies significantly by model size and backend.

## Tested models

The following models have been tested with Ollama. Other backends can use the same models if they support them.

| Model | Pull command | Commit quality | Breaking change detection | Speed |
|-------|-------------|----------------|--------------------------|-------|
| `qwen3.5:0.8b` | `ollama pull qwen3.5:0.8b` | Good (High accuracy) | ⚠ Improved | Very fast |
| `qwen3.5:latest` (7b) | `ollama pull qwen3.5:latest` | Very Good | ✓ Reliable | Fast |
| `gemma4:26b` | `ollama pull gemma4:26b` | Elite | ✓ Reliable | Slow |

**Recommended for performance:** `qwen3.5:latest` (7b) — excellent precision with our refined prompts.
**Recommended for budget laptops:** `qwen3.5:0.8b` (1GB) — surprisingly accurate for basic commits.

> **Note:** The models above were tested on Ollama. LM Studio, vLLM, and LocalAI can serve the same GGUF/Safetensors models with similar quality — the prompts are provider-agnostic.

## Accuracy-First Prompts

As of v1.1.0, git-courer uses a refined prompt engine that:
- **Prioritizes Grounding**: Models are forbidden from "inventing" impacts; they must stick to the diff facts.
- **Context-Aware**: Prompts include explicit file lists before the diff to anchor the model's attention.
- **Model Agnostic**: Optimized to work reliably even on <1B parameter models.
- **Bilingual**: Responds in the same language as the user instruction.

## Breaking change detection

With the new prompt engine, even smaller models like `qwen3.5:0.8b` can detect breaking changes if the instruction implies it, although larger models (>7b) remain more reliable for automatic detection without explicit instructions.

## Reasoning Models (DeepSeek/R1)

git-courer ahora soporta modelos de razonamiento (como DeepSeek-R1 u otros modelos con capacidades Chain-of-Thought). 

Si usas un modelo de razonamiento, git-courer detecta automáticamente si el modelo soporta inyección de parámetros para controlar el proceso de razonamiento. Si el backend es compatible, se puede habilitar la supresión de `no_think` para que el modelo responda directamente sin incluir sus pensamientos internos en el diff o la respuesta final.

### Configuración
Solo especifica el modelo en tu `config.yaml`. La detección de capacidades de razonamiento es automática.

```yaml
llm:
  model: deepseek-r1:7b
```

## Changing the model

In `.gcourer/config.yaml`:

```yaml
# Recommended (unified config)
llm:
  model: qwen3.5:latest
```

Or using legacy config:

```yaml
ollama:
  model: qwen3.5:latest
```

Or globally at `~/.config/git-courer/config.yaml`.

## Using without Ollama

git-courer works without any LLM backend. Commit messages will be generic (`chore: update files`). All git operations, security checks, and version management still work.

## Using with LM Studio

1. Start LM Studio and load a model
2. Enable the local server (default: `http://localhost:1234/v1`)
3. Configure git-courer:

```yaml
llm:
  provider: lmstudio
  base_url: http://localhost:1234/v1
  model: my-model
```

## Using with vLLM

1. Start vLLM with your model:
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

## Using with LocalAI

1. Start LocalAI with your model
2. Configure git-courer:

```yaml
llm:
  provider: localai
  base_url: http://localhost:8080/v1
  model: my-model
```

## Using with any OpenAI-compatible server

Any server that exposes the `/v1/chat/completions` endpoint works:

```yaml
llm:
  provider: openai-compatible
  base_url: https://my-llm-server.example.com/v1
  model: my-model
  api_key: sk-my-key   # optional
```
