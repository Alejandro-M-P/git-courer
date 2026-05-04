# Configuration Reference

> **`llm.provider` and `llm.model` are mandatory.** Without them, all AI-powered operations fail. Basic git reads (status, diff, log) still work.

## Quick start

Edit the global config file:
- `~/.config/git-courer/config.yaml`

Or run `git-courer` (no arguments) to configure everything interactively via the TUI.

> **Note**: Per-project `.gcourer/config.yaml` was removed in v1.5+. All settings are now in the global config only.

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
| provider | string | **REQUIRED** | `ollama` for Ollama (auto-start included). Anything else is treated as OpenAI-compatible — use any string that makes sense to you (e.g. `lmstudio`, `vllm`, `localai`, `myserver`). Requires `base_url`. |
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
