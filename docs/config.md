# Configuration Reference

## File locations

| File | Purpose |
|------|---------|
| `~/.config/git-courer/config.yaml` | Global defaults for all projects |
| `.gcourer/config.yaml` | Project-specific (overrides global) |

Run `git-courer setup` to create the project config.

## Config options you can change

```yaml
# Ollama (AI)
ollama:
  host: http://localhost:11434   # Ollama server URL
  model: qwen3.5:latest          # Model to use

# Git behavior
git:
  auto_add_secrets: true      # Stage detected secrets (true) or block (false)
  require_clean_repo: false   # Require clean working tree before operations

# Safety
validation:
  require_confirmation: true   # Ask before executing

# Preview dialogs
preview:
  enabled: true              # Show preview before executing

# Which commands are enabled
commands:
  enabled_operations:
    - commit
    - release
    - push
    - pull
    - branch_create
    - branch_delete
    - merge
    # - tag_create         # create local tag
    # - tag_delete        # delete local tag
    # - tag_push          # push tag to remote
    # - tag_delete_remote # delete tag from remote ⚠️

# Preview for specific commands (false by default)
preview:
  operations:
    tag_create: false
    tag_delete: false
    tag_push: false
    tag_delete_remote: false

# Auto-backup before destructive ops
backup:
  enabled: true
```

## Common configs

### No confirmations (faster)

```yaml
validation:
  require_confirmation: false
preview:
  enabled: false
```

### Different model

```yaml
ollama:
  model: llama3.2:latest
```

### Only basic commands

```yaml
commands:
  enabled_operations:
    - commit
    - push
    - pull
```

### Disable backup

```yaml
backup:
  enabled: false
```

## Version bump (automatic)

| Commit | Bump |
|--------|------|
| `feat!:` `fix!:` | major |
| `feat:` | minor |
| rest | patch |

Tell git-courer *"release as minor"* to override.

## Tag operations

### Create tag
```bash
git-courer tag_create       # create local tag
git-courer tag_create_start  # with LLM interpretation
```

### Delete tag
```bash
git-courer tag_delete        # delete local tag
git-courer tag_delete_start  # with LLM interpretation
```

### Push tag
```bash
git-courer tag_push           # push tag to remote
git-courer tag_push_start    # with LLM interpretation
```

### Delete remote tag ⚠️
```bash
git-courer tag_delete_remote     # delete from remote
git-courer tag_delete_remote_start # with LLM interpretation
```

## Error handling

| Error | Solution |
|-------|----------|
| "tag already exists" | Use `tag_delete_remote` first |
| "tag not found" | Check tag name with `READ_TAGS` |
| "invalid tag name" | Use semver (v1.0.0) |