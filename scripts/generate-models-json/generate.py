#!/usr/bin/env python3
"""
Generate models.json from the Ollama library + registry.
  1. Fetches ollama.com/library (HTML) → extracts all model names
  2. For each model: fetches registry.ollama.com (CDN) → extracts num_ctx

Usage:
  python3 scripts/generate-models-json/generate.py
  python3 scripts/generate-models-json/generate.py > internal/data/models.json
"""

import json
import os
import re
import sys
import urllib.request
import urllib.error
import html.parser

REGISTRY = "https://registry.ollama.com"
LIBRARY_URL = "https://ollama.com/library"

# ── HTML Parser ──────────────────────────────────────────────────────
class LibraryParser(html.parser.HTMLParser):
    def __init__(self):
        super().__init__()
        self.models = set()
        self._in_pre = False

    def handle_starttag(self, tag, attrs):
        if tag != "a":
            return
        attrs_dict = dict(attrs)
        href = attrs_dict.get("href", "")
        m = re.match(r"^/library/([a-zA-Z0-9][a-zA-Z0-9._-]+)$", href)
        if m:
            self.models.add(m.group(1))

# ── HTTP helpers ─────────────────────────────────────────────────────
def fetch_text(url):
    req = urllib.request.Request(url, headers={
        "User-Agent": "git-courer-model-catalog/1.0"
    })
    with urllib.request.urlopen(req, timeout=15) as resp:
        return resp.read().decode("utf-8", errors="replace")

def fetch_json(url):
    req = urllib.request.Request(url, headers={
        "Accept": "application/json",
        "User-Agent": "git-courer-model-catalog/1.0"
    })
    with urllib.request.urlopen(req, timeout=15) as resp:
        return json.loads(resp.read())

def main():
    # ── Step 1: get model list from library page ─────────────────────
    print("Fetching model list from ollama.com/library...", file=sys.stderr)
    html = fetch_text(LIBRARY_URL)
    parser = LibraryParser()
    parser.feed(html)
    model_names = sorted(parser.models)
    print(f"  Found {len(model_names)} models", file=sys.stderr)

    if not model_names:
        print("ERROR: no models found, aborting", file=sys.stderr)
        sys.exit(1)

    # ── Step 2: fetch context window for each model ──────────────────
    models = {}
    errors = []
    skipped = []

    for i, name in enumerate(model_names, 1):
        print(f"  [{i}/{len(model_names)}] {name} ...", file=sys.stderr, end="")

        manifest_url = f"{REGISTRY}/v2/library/{name}/manifests/latest"
        try:
            manifest = fetch_json(manifest_url)
        except urllib.error.HTTPError as e:
            print(f" HTTP {e.code}", file=sys.stderr)
            skipped.append(name)
            continue
        except Exception as e:
            print(f" ERROR {e}", file=sys.stderr)
            errors.append(f"{name}: {e}")
            continue

        # Find the params blob
        layers = manifest.get("layers", [])
        params_digest = None
        for layer in layers:
            if layer.get("mediaType") == "application/vnd.ollama.image.params":
                params_digest = layer["digest"]
                break

        if not params_digest:
            print(" (no params blob)", file=sys.stderr)
            skipped.append(name)
            continue

        params_url = f"{REGISTRY}/v2/library/{name}/blobs/{params_digest}"
        try:
            params = fetch_json(params_url)
        except Exception as e:
            print(f" params ERROR {e}", file=sys.stderr)
            errors.append(f"{name} params: {e}")
            continue

        ctx = params.get("num_ctx")
        if ctx is None:
            print(" (no num_ctx)", file=sys.stderr)
            skipped.append(name)
            continue

        models[name] = ctx
        print(f" {ctx}", file=sys.stderr)

    output = {"models": models}
    json.dump(output, sys.stdout, indent=2, ensure_ascii=False)

    print(file=sys.stderr)
    print(f"Done: {len(models)} models, {len(skipped)} skipped, {len(errors)} errors", file=sys.stderr)

if __name__ == "__main__":
    main()
