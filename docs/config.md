# Configuration Reference

> **`llm.provider` and `llm.model` are mandatory** unless `llm.enabled` is set to `false`. When `llm.enabled` is `false`, git-courer runs in offline mode, bypassing LLM connectivity requirements. Basic git features still work fully offline.

## Quick Start

Edit the global config file:
- `~/.config/git-courer/config.yaml`

Or run `git-courer` (no arguments) to configure everything interactively via the TUI.

## Global Config (`~/.config/git-courer/config.yaml`)

```yaml
llm:
  enabled: true
  provider: ollama
  model: gemma4:26b
  base_url: http://localhost:11434/v1
  num_parallel: 1
  context_window: 0          # 0 = auto-resolved at install (8192 fallback)

release:
  type: tag
```

### llm options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable or disable AI features. When set to `false`, git-courer runs offline without an LLM. |
| `provider` | string | **REQUIRED** | `ollama` for Ollama (auto-start included). Anything else is treated as OpenAI-compatible (e.g. `lmstudio`, `vllm`, `localai`, `openai`). Requires `base_url`. (Ignored if `enabled` is `false`) |
| `model` | string | **REQUIRED** | Model name/identifier (e.g., `gemma4:26b`, `qwen3.5:latest`). (Ignored if `enabled` is `false`) |
| `base_url` | string | `http://localhost:11434/v1` | API endpoint URL (include `/v1` suffix for OpenAI-compatible providers). |
| `num_parallel` | int | `1` | Max concurrent LLM calls. |
| `context_window` | int | `0` | Context window size. Automatically resolved at install time. If set to `0` or omitted, defaults to `8192`. |

### release options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `type` | string | `tag` | Release workflow type. Options: `tag` (creates a local annotated git tag and pushes it to remote) or `github` (creates a release on GitHub via the `gh` CLI). |

---

## Per-Project Config (`.git/git-courer/config.json`)

This file is **committable** and **shared by your team**. It lives in your repository and travels with it. Editing it per project gives significantly better results.

```json
{
  "description": "Payment service API",
  "path_types": {
    "test": ["test/"],
    "ci": [".github/workflows/"]
  },
  "test_command": "go test ./...",
  "base_branch": "main",
  "user_name": "Alice",
  "user_email": "alice@example.com",
  "signing_key": "~/.ssh/id_ed25519",
  "excluded": ["*.pb.go", "vendor/", "*.gen.go"]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `description` | string | Short project description for LLM context. Used to scope commit messages. |
| `path_types` | object | Maps commit types (like `test`, `ci`, `docs`) to file path prefixes. If all changed files match the prefixes under a type, that type is automatically assigned to the commit. Defaults (when unset): `test` → `["test/"]`, `ci` → `[".github/workflows/", "ci/"]`, `docs` → `["docs/"]`. |
| `test_command` | string | Command for pre-PR validation (used by `pr-review` and `git-courer release`). |
| `base_branch` | string | Base branch used for PR validation and comparison (e.g., `main` or `develop`). |
| `user_name` | string | Per-project commit author name. Overrides the global `user.name` for commits made through git-courer. |
| `user_email` | string | Per-project commit author email. Overrides the global `user.email` for commits made through git-courer. |
| `signing_key` | string | Path to a GPG/SSH signing key used to sign commits made through git-courer. |
| `excluded` | array | Path prefixes to exclude from diff analysis and commit classification. |

### Best Practices

- **Trailing Slash Rule**: All prefixes in `path_types` and `excluded` **must end with a trailing slash (`/`)** to prevent partial matches (e.g., `"docs/"` ensures it won't match `"docsify/"`). Missing trailing slashes are normalized at load time.
- **Define Custom Path Types**: By default, path types are mapped for `test` (`test/`), `ci` (`.github/workflows/`, `ci/`), and `docs` (`docs/`). Define custom `path_types` in `config.json` to override or extend this mapping.
- **Set Test Command**: Set `test_command` to the fastest command that proves correctness (e.g., `go test ./...`, `npm test`).
- **Use Excluded**: Use `excluded` for generated files, vendored code, and lockfiles you never want in commit analysis.

---

## Examples

### Minimal Global Config
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
  provider: openai
  base_url: https://api.openai.com/v1
  model: gpt-4o-mini
  num_parallel: 2
```

### Per-Project with Custom Path Types
```json
{
  "description": "Microservice mesh control plane",
  "path_types": {
    "api": ["pkg/api/", "internal/api/"],
    "storage": ["internal/storage/"],
    "deploy": ["deploy/"]
  },
  "test_command": "make test-unit",
  "excluded": ["*.pb.go", "vendor/"]
}
```
