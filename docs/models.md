# Model Guide

git-courer works with any Ollama model. Quality varies significantly by model size.

## Tested models

| Model | Pull command | Commit quality | Breaking change detection | Speed |
|-------|-------------|----------------|--------------------------|-------|
| `qwen3.5:0.8b` | `ollama pull qwen3.5:0.8b` | Basic | ❌ No | Very fast |
| `qwen3.5:latest` (7b) | `ollama pull qwen3.5:latest` | Good | ⚠ Unreliable | Fast |
| `gemma3:12b` | `ollama pull gemma3:12b` | Very good | ⚠ Sometimes | Medium |
| `gemma4:26b` | `ollama pull gemma4:26b` | Excellent | ✓ Usually | Slow |
| `qwen3:32b` | `ollama pull qwen3:32b` | Excellent | ✓ Reliable | Slow |

**Recommended for most setups:** `qwen3.5:latest` (7b) — good balance of quality and speed.

## Breaking change detection

This is the most important limitation to understand.

**The problem:** When a diff removes a public function or changes an API, small models may write `chore: remove X` instead of `feat!: remove X`. This means the version bump and changelog won't reflect the breaking change.

**Why it happens:** Detecting that something is a "breaking change" requires understanding the context of the code — what callers depend on it, what contract it formed. Models under ~13b parameters don't do this reliably.

**Workaround for small models:** Tell git-courer explicitly.

```
"commit this change — the New() constructor was removed, it's a breaking change"
```

The model will then write the correct `feat!:` format.

**For automatic detection:** Use a model with at least 13b parameters (`gemma3:12b` or larger).

## Known quirks

**`gemma4:26b`** — Tends to get stuck when there are many untracked files. It sometimes asks for clarification instead of deciding. Pre-staging files with `git add` before asking git-courer to commit avoids this.

**`qwen3.5:latest`** — Can return comma-separated paths as a filter (e.g., `"a.go, b.go"`). This causes git to fail looking for a file literally named `"a.go, b.go"`. Pre-staging files resolves it.

**All models under 7b** — Non-deterministic on untracked files. For reliable results, stage files before asking git-courer to commit.

## Changing the model

In `.gcourer/config.yaml`:

```yaml
ollama:
  model: qwen3.5:latest
```

Or globally at `~/.config/git-courer/config.yaml`.

## Using without Ollama

git-courer works without Ollama. Commit messages will be generic (`chore: update files`). All git operations, security checks, and version management still work.
