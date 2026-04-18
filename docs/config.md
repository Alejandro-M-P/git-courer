# Configuration Reference

## File locations

| File | Purpose |
|------|---------|
| `~/.config/git-courer/config.yaml` | Global defaults for all projects |
| `.gcourer/config.yaml` | Project-specific (overrides global) |

Run `git-courer setup` to create the project config with sensible defaults.

## Full config reference

```yaml
ollama:
  host: http://localhost:11434    # Ollama API URL
  model: qwen3.5:latest           # Model to use (see docs/models.md)
  context_window: 0               # 0 = auto-detect from model
  auto_start: false               # Start Ollama automatically if not running

git:
  workdir: .                      # Working directory
  auto_add_secrets: true          # Stage detected secrets anyway (true) or block (false)
  require_clean_repo: false       # Require clean working tree before operations

secrets:
  detection_mode: regex+ai        # regex | regex+ai | disabled
  use_llm_security_scan: auto     # auto | true | false
  patterns:                       # Additional file patterns to flag
    - "*.key"
    - "*.pem"
    - ".env*"
    - "credentials.json"

validation:
  require_confirmation: true      # Ask before executing operations
  max_commit_length: 72           # Max subject line length

preview:
  enabled: true                   # Show preview before executing
  operations:
    commit: true
    branch_create: true
    branch_delete: true
    release: true

commit:
  ttl: 10m                        # How long a pending commit plan stays valid
  max_plan_retries: 3
  background_threshold: 10000     # Diff size (chars) above which commit runs async

release:
  max_commits_per_chunk: 20       # Commits sent per changelog generation call

commands:
  enabled_operations:             # Which operations are allowed
    - commit
    - release
    - push
    - pull
    - branch_create
    - branch_delete
    - merge

backup:
  enabled: true                   # Auto-backup before destructive operations (reset, force push)
```

## Common configurations

### Disable confirmation for all operations

```yaml
validation:
  require_confirmation: false
preview:
  enabled: false
```

### Use a faster/smaller model

```yaml
ollama:
  model: qwen3.5:0.8b
```

### Disable secret detection (not recommended)

```yaml
secrets:
  detection_mode: disabled
```

### Restrict to specific operations

```yaml
commands:
  enabled_operations:
    - commit
    - push
```

## Version bump rules

These are calculated from commit messages — no configuration needed.

| Commit type | Version bump |
|-------------|-------------|
| `feat!:`, `fix!:` or `BREAKING CHANGE:` in body | major (1.x → 2.0.0) |
| `feat:` | minor (1.2.x → 1.3.0) |
| Everything else | patch (1.2.3 → 1.2.4) |

You can always override: tell git-courer *"release as minor"* and it uses your bump regardless of what the commits say.
