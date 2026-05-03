# Configuration Reference

## Quick start

Edit one of:
- `~/.config/git-courer/config.yaml` (global)
- `.gcourer/config.yaml` (project — overrides global)

```yaml
llm:
  provider: ollama
  model: gemma4:26b

preview:
  enabled: true

git:
  workdir: .

context:
  project: ""  # mandatory — short project description
  style: concise_technical
```

## All options

### llm
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| provider | string | **REQUIRED** | LLM backend: `ollama`, `openai-compatible`, `lmstudio`, `vllm`, `localai` |
| model | string | **REQUIRED** | Model name/identifier |
| base_url | string | http://localhost:11434/v1 | API endpoint URL |
| num_parallel | int | 1 | Max concurrent LLM calls |

### preview
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| enabled | bool | true | Enable preview dialogs before executing operations |

### git
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| workdir | string | . | Default working directory |

### context
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| project | string | **REQUIRED** | Short project description for LLM context |
| style | string | concise_technical | Communication style conventions |

## Examples

### Minimal config
```yaml
llm:
  provider: ollama
  model: qwen3.5:latest

context:
  project: My awesome project
```

### LM Studio
```yaml
llm:
  provider: lmstudio
  base_url: http://localhost:1234/v1
  model: my-model

context:
  project: My project
  style: verbose
```

### vLLM
```yaml
llm:
  provider: vllm
  base_url: http://localhost:8000/v1
  model: my-model

context:
  project: My project
```

### OpenAI-compatible with API key
```yaml
llm:
  provider: openai-compatible
  base_url: https://my-llm-server.example.com/v1
  model: my-model
  num_parallel: 2

context:
  project: My project
```

### No preview (execute immediately)
```yaml
llm:
  provider: ollama
  model: gemma4:26b

preview:
  enabled: false

context:
  project: My project
```
