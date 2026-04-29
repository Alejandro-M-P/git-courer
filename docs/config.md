# Configuration Reference



## Quick start

Edit one of:
- `~/.config/git-courer/config.yaml` (global)
- `.gcourer/config.yaml` (project — overrides global)

```yaml
# New unified config (recommended)
llm:
  provider: ollama
  base_url: http://localhost:11434/v1
  model: gemma4:26b
  ollama:
    auto_start: true

# Legacy (still supported, auto-migrated)
ollama:
  host: http://localhost:11434
  model: gemma4:26b

git:
  auto_add_secrets: true

secrets:
  detection_mode: regex+ai

preview:
  enabled: true

commands:
  enabled_operations:
    - commit

backup:
  enabled: true
```

## All editable options

### llm
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| provider | string | ollama | LLM backend: `ollama`, `openai-compatible`, `lmstudio`, `vllm`, `localai` |
| base_url | string | http://localhost:11434/v1 | API endpoint URL |
| model | string | gemma4:26b | Model name/identifier |
| api_key | string | "" | Optional API key for protected servers |
| context_window | int | 0 | Context window size (0 = model default) |

### llm.ollama
Ollama-specific sub-configuration within the `llm:` section:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| models_dir | string | "" | Custom models directory |
| auto_start | bool | false | Auto-start Ollama if not running |

> **Note:** All Ollama connections use `/v1/` endpoints (OpenAI-compatible). This is supported since Ollama v0.1.25 (2023). If you're running an older version, update Ollama.

### ollama ⚠️ Legacy
> ⚠️ The `ollama:` section is legacy. It still works but `llm:` is recommended. If both exist, `llm:` takes precedence. Legacy fields are auto-migrated to `llm:` at runtime.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| host | string | http://localhost:11434 | Ollama server URL |
| model | string | gemma4:26b | Model to use |
| context_window | int | 0 | Context window size (0 = model default) |
| auto_start | bool | false | Auto-start Ollama if not running |
| models_dir | string | "" | Custom models directory |

### git
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| workdir | string | . | Default working directory |
| auto_add_secrets | bool | true | Auto-stage detected secrets |
| require_clean_repo | bool | false | Require clean working tree |

### secrets
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| detection_mode | string | regex+ai | Detection mode: `regex`, `ai`, `regex+ai` |
| patterns | []string | see below | File & content patterns to check |
| use_llm_security_scan | string | auto | Use LLM for scan: `auto` (all models), `true`, `false` |

**Default Patterns**:
`*.key`, `*.pem`, `.env*`, `credentials.json`, `secrets.yaml`, `*.password`, `*.token`, `(?i)DUMMY_AWS[0-9A-Z]{16}` (AWS), `(?i)SECRET_?[A-Z0-9]{16,64}` (Generic).

---

### testing (Environment Variables)
These are not in `config.yaml` but used during development:

| Variable | Default | Description |
|----------|---------|-------------|
| `GC_TEST_MODELS` | `gemma4:26b` | Comma-separated models for the Quality Matrix |
| `LLM_HOST` | `http://localhost:11434` | LLM host for integration tests (applies to any provider) |

### preview
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| enabled | bool | true | Enable preview dialogs |
| operations | map[string]bool | see below | Per-operation preview flag |

Default per-operation values:

| Operation | Preview required |
|-----------|-----------------|
| commit | true |
| branch_create | true |
| branch_delete | true |
| release | true |
| tag_create | false |
| tag_delete | false |
| tag_push | false |
| tag_delete_remote | false |

### commit
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| ttl | duration | 10m | How long a pending commit plan is valid |
| log_path | string | .gcourer/task.log | Path to commit log file |
| max_log_lines | int | 500 | Max log lines to feed the LLM |

### release
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| log_path | string | .gcourer/release.log | Path to release log file |
| max_log_lines | int | 500 | Max log lines to feed the LLM |
| max_commits_per_chunk | int | 20 | Max commits per changelog chunk |

### commands
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| enabled_operations | []string | commit, release, push, pull, branch_create, branch_delete, merge | Allowed operations |

Available operation keys: `commit`, `release`, `push`, `pull`, `branch_create`, `branch_delete`, `merge`, `tag_create`, `tag_delete`, `tag_push`, `tag_delete_remote`

### backup
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| enabled | bool | true | Create a git ref backup before every destructive operation |

### validation
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| require_confirmation | bool | true | Require explicit user confirmation before executing operations when preview.enabled=true (default preview behavior) |

## Examples

### Commit only
```yaml
commands:
  enabled_operations:
    - commit
```

### No confirmations
```yaml
preview:
  enabled: false
```

### Different model (recommended)
```yaml
llm:
  model: qwen3.5:latest
```

### Different model (legacy)
```yaml
ollama:
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

### LocalAI
```yaml
llm:
  provider: localai
  base_url: http://localhost:8080/v1
  model: my-model
```

### OpenAI-compatible with API key
```yaml
llm:
  provider: openai-compatible
  base_url: https://my-llm-server.example.com/v1
  model: my-model
  api_key: sk-my-key
```

### Custom secrets patterns
```yaml
secrets:
  patterns:
    - "*.key"
    - "*.pem"
    - ".env*"
    - "credentials.json"
    - "*.secret"
```

### Longer plan TTL
```yaml
commit:
  ttl: 30m
```

---

## GoReleaser Integration

git-courer ahora crea tags anotados con el changelog como cuerpo del tag. GoReleaser (o cualquier herramienta CI/CD) lee el changelog directamente desde la anotación del tag usando la variable de template `{{ .TagBody }}`. No requiere archivo auxiliar ni configuración adicional.

### Example `.goreleaser.yaml`

```yaml
# .goreleaser.yaml
release:
  prerelease: auto
  header: |
    Changelog

    {{ .TagBody }}
```

### Benefits
- **Universal compatibility**: Works with any release tool (GoReleaser, release-please, manual scripts)
- **No vendor lock-in**: git-courer doesn't assume any specific release toolchain
- **Auto-update compatibility**: Installer works because the release is created by the external tool


