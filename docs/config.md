# Configuration Reference

> git-courer is CLI-only. No UI exists.

## Quick start

Edit one of:
- `~/.config/git-courer/config.yaml` (global)
- `.gcourer/config.yaml` (project)

```yaml
ollama:
  host: http://localhost:11434
  model: gemma4:26b

git:
  auto_add_secrets: true

secrets:
  detection_mode: regex+ai

validation:
  require_confirmation: true

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
| context_window | int | 0 | Context size (0=default) |
| auto_start | bool | false | Auto-start Ollama |
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
| detection_mode | string | regex+ai | Detection mode: regex, ai, regex+ai |
| patterns | []string | *.key, *.pem, .env*, credentials.json, secrets.yaml, *.password, *.token | File patterns to check |
| use_llm_security_scan | string | auto | Use LLM for scan: auto, true, false |

### validation
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| require_confirmation | bool | true | Ask before executing destructive operations |
| max_commit_length | int | 72 | Maximum commit message length |

### preview
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| enabled | bool | true | Enable preview dialogs |

Operations that can require preview (map[string]bool):
- commit
- branch_create
- branch_delete
- release
- tag_create
- tag_delete
- tag_push
- tag_delete_remote

### commands
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| enabled_operations | []string | commit, release, push, pull, branch_create, branch_delete, merge, tag_create, tag_delete, tag_push, tag_delete_remote | Allowed commands |

### backup
| Field | Type | Default | Description |
|-------|------|---------|-------------|
| enabled | bool | true | Enable automatic backup before destructive operations |

## Examples

### Commit only
```yaml
commands:
  enabled_operations:
    - commit
```

### No confirmations
```yaml
validation:
  require_confirmation: false
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