# Model Guide

git-courer works with any Ollama model. Quality varies significantly by model size.

## Tested models

| Model | Pull command | Commit quality | Breaking change detection | Speed |
|-------|-------------|----------------|--------------------------|-------|
| `qwen3.5:0.8b` | `ollama pull qwen3.5:0.8b` | Good (High accuracy) | ⚠ Improved | Very fast |
| `qwen3.5:latest` (7b) | `ollama pull qwen3.5:latest` | Very Good | ✓ Reliable | Fast |
| `gemma4:26b` | `ollama pull gemma4:26b` | Elite | ✓ Reliable | Slow |

**Recommended for performance:** `qwen3.5:latest` (7b) — excellent precision with our refined prompts.
**Recommended for budget laptops:** `qwen3.5:0.8b` (1GB) — surprisingly accurate for basic commits.

## Accuracy-First Prompts

As of v1.1.0, git-courer uses a refined prompt engine that:
- **Prioritizes Grounding**: Models are forbidden from "inventing" impacts; they must stick to the diff facts.
- **Context-Aware**: Prompts include explicit file lists before the diff to anchor the model's attention.
- **Model Agnostic**: Optimized to work reliably even on <1B parameter models.
- **Bilingual**: Responds in the same language as the user instruction.

## Breaking change detection

With the new prompt engine, even smaller models like `qwen3.5:0.8b` can detect breaking changes if the instruction implies it, although larger models (>7b) remain more reliable for automatic detection without explicit instructions.

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
