# Configuration Reference



## Quick start

Edit one of:
- `~/.config/git-courer/config.yaml` (global)
- `.gcourer/config.yaml` (project — overrides global)

```yaml
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

### ollama
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
| `LLM_HOST` | `http://localhost:11434` | Ollama host for integration tests |

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

### Different model
```yaml
ollama:
  model: qwen2.5:latest
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
