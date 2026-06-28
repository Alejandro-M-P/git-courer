# Configuration Reference

> **`llm.provider` and `llm.model` are mandatory** unless `llm.enabled` is set to `false`. When `llm.enabled` is `false`, git-courer runs in offline mode, bypassing LLM connectivity requirements. Basic git features still work fully offline.

## Quick start

Edit the global config file:
- `~/.config/git-courer/config.yaml`

Or run `git-courer` (no arguments) to configure everything interactively via the TUI.

## Global config (`~/.config/git-courer/config.yaml`)

```yaml
llm:
  enabled: true
  provider: ollama
  model: gemma4:26b

preview:
  enabled: true

git:
  workdir: .
```

## Per-project config (`.git/git-courer/config.json`)

This file is **committable** and **shared by your team**. It lives in your repo and travels with it. Editing it per project gives significantly better results.

```json
{
  "description": "Payment service API",
  "areas": {
    "internal/payments/": "payments",
    "internal/auth/": "auth",
    "internal/infra/": "infra"
  },
  "test_command": "go test ./...",
  "excluded": ["*.pb.go", "vendor/", "*.gen.go"]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `description` | string | Short project description for LLM context |
| `areas` | object | Maps directory prefixes to area names for commit pipeline grouping and changelog generation |
| `test_command` | string | Command for pre-PR validation (used by `pr-review` and `git-courer release`) |
| `excluded` | array | Glob patterns to exclude from diff analysis and commit chunking |

### Best practices

- **Better results = edit this file per project.** Areas, test command, and exclusions are project-specific. One global config cannot know your codebase structure.
- Define `areas` before your first release. Without them, changelog grouping falls back to file paths.
- Set `test_command` to the fastest command that proves correctness (`go test ./...`, `make test-ci`, `npm test`).
- Use `excluded` for generated files, vendored code, and lockfiles you never want in commit analysis.

## All options

### llm
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| enabled | bool | true | Enable or disable AI features. When set to `false`, git-courer runs offline without an LLM. |
| provider | string | **REQUIRED** | `ollama` for Ollama (auto-start included). Anything else is treated as OpenAI-compatible — use any string that makes sense to you (e.g. `lmstudio`, `vllm`, `localai`, `myserver`). Requires `base_url`. (Ignored if `enabled: false`) |
| model | string | **REQUIRED** | Model name/identifier (Ignored if `enabled: false`) |
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

### release
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| type | string | tag | Release workflow type. Options: `tag` (creates a local annotated git tag and pushes it to remote) or `github` (creates a release on GitHub via the `gh` CLI). |

## Examples

### Minimal config
```yaml
llm:
  provider: ollama
  model: qwen3.5:latest
```

### LM Studio
```yaml
llm:
  provider: lmstudio
  base_url: http://localhost:1234/v1
  model: my-model
```

### vLLM
```yaml
llm:
  provider: vllm
  base_url: http://localhost:8000/v1
  model: my-model
```

### OpenAI-compatible with API key
```yaml
llm:
  provider: openai-compatible
  base_url: https://my-llm-server.example.com/v1
  model: my-model
  num_parallel: 2
```

### No preview (execute immediately)
```yaml
llm:
  provider: ollama
  model: gemma4:26b

preview:
  enabled: false
```

### Per-project with areas
```json
{
  "description": "Microservice mesh control plane",
  "areas": {
    "pkg/api/": "api",
    "internal/control/": "control",
    "internal/storage/": "storage",
    "deploy/": "deploy"
  },
  "test_command": "make test-ci",
  "excluded": ["*.pb.go", "vendor/"]
}
```
