#!/bin/bash
# Update model catalog from LiteLLM model_prices_and_context_window.json
# Fetches open-weight and local models, extracts context windows
# Usage: scripts/update-models.sh [--dry-run]
set -e

DRY_RUN=false
if [ "$1" = "--dry-run" ]; then
  DRY_RUN=true
fi

OUTPUT_FILE="internal/data/models.json"
TMP_JSON=$(mktemp)
trap "rm -f $TMP_JSON" EXIT

echo "Fetching model database from LiteLLM..." >&2

if ! curl -sf --max-time 60 "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json" -o "$TMP_JSON"; then
  echo "Failed to fetch from LiteLLM" >&2
  exit 1
fi

echo "Parsing and filtering local/open-weight models..." >&2

python3 - "$TMP_JSON" "$DRY_RUN" << 'PYTHON_SCRIPT'
import json
import sys

json_path = sys.argv[1]
dry_run = sys.argv[2] == "true"

with open(json_path, "r") as f:
    data = json.load(f)

# Exclude these litellm_providers (cloud services) - but NOT ollama since we want local ollama models
EXCLUDE_PROVIDERS = {
    "openai", "anthropic", "google", "cohere", "mistral", "azure",
    "bedrock", "vertex", "replicate", "together", "anyscale",
    "deepinfra", "fireworks", "groq", "perplexity", "sambanova",
    "sagemaker", "predibase", "custom", "hosted",
    "cloudflare", "inference", "voyage", "upstage", "reka"
}

# Include only these model families (open-weight / local-capable)
INCLUDE_FAMILIES = [
    # Ollama-style names
    "llama", "mistral", "qwen", "deepseek", "gemma", "phi", "codellama",
    "mixtral", "flamethrower", "command", "starling", "wizard", "vicuna",
    "longllama", "manticore", "orca", "nous", "airo",
    # HuggingFace model families
    "meta-", "meta_", "/Llama", "/Mistral", "/Qwen", "/Gemma", "/Phi",
    "microsoft/", "deepseek-ai/", "bigcode/", "01-ai/", "tiiuae/",
    "EleutherAI/", "facebook/", "google/", "stabilityai/",
    "mistralai/", "codellama/", "Qwen/", "Deepseek/",
    # Special models
    "Yi-", "Yi/", "Mistral/", "Mixtral/", "FLAN-", "flan-",
    "Wizard", "zephyr", "Starling", "orin", "Mathstral",
    # Alternative names
    "Llama-3", "Llama-4", "Llama2", "LLaMA", "MISTRAL",
    "Qwen2", "Qwen3", "qwen2", "qwen3",
    # Vision models
    "Vision", "vision", "-Instruct", "-Chat",
    # Reasoning models
    "Reasoning", "Think", "R1", "-r1",
    # Ollama models
    "ollama/"
]

# Model name patterns to exclude (definitely cloud-only)
EXCLUDE_PATTERNS = [
    "gpt-4", "gpt-5", "gpt-3.5",
    "claude-3", "claude-4", "claude-haiku", "claude-opus", "claude-sonnet",
    "gemini-2", "gemini-1.5", "gemini-pro",
    "command-", "command.r",
    "amazon/", "aws/", "bedrock/",
    "azure_openai/", "openai/",
    "together/", "replicate/",
    "perplexity/", "cohere/",
    "voyage-", "reka-", "upstage/",
    "writer.", "palmyra", "synthia",
    "text-to-sql", "snowflake",
    "jamba", "jurrasic"
]

results = {}
excluded_provider = 0
excluded_pattern = 0
no_context = 0

for model_key, model_data in data.items():
    if model_key == "sample_spec" or not isinstance(model_data, dict):
        continue

    litellm_provider = model_data.get("litellm_provider", "")
    mode = model_data.get("mode", "")

    if mode not in ("chat", "completion", ""):
        continue

    if litellm_provider in EXCLUDE_PROVIDERS:
        excluded_provider += 1
        continue

    model_lower = model_key.lower()
    if any(pattern in model_lower for pattern in EXCLUDE_PATTERNS):
        excluded_pattern += 1
        continue

    # Must match at least one include family
    if not any(family in model_key for family in INCLUDE_FAMILIES):
        excluded_pattern += 1
        continue

    max_input = model_data.get("max_input_tokens", 0)
    max_tokens = model_data.get("max_tokens", 0)
    max_output = model_data.get("max_output_tokens", 0)

    context_window = max_input or max_tokens or max_output or 0

    if context_window <= 0:
        no_context += 1
        continue

    # Handle ollama/* models - strip prefix for Ollama compatibility
    if model_key.startswith("ollama/"):
        clean_key = model_key.replace("ollama/", "")
        results[clean_key] = context_window
        continue

    results[model_key] = context_window

print(f"Total models in LiteLLM: {len(data)}", file=sys.stderr)
print(f"Excluded (provider): {excluded_provider}", file=sys.stderr)
print(f"Excluded (pattern): {excluded_pattern}", file=sys.stderr)
print(f"No context window: {no_context}", file=sys.stderr)
print(f"Open-weight models found: {len(results)}", file=sys.stderr)

if dry_run:
    sample = dict(list(results.items())[:40])
    print(json.dumps({"models": sample}, indent=2))
else:
    with open("internal/data/models.json", "w") as f:
        json.dump({"models": results}, f, indent=2)
    print(f"Updated internal/data/models.json with {len(results)} models", file=sys.stderr)
PYTHON_SCRIPT