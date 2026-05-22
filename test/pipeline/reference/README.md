# Pipeline Reference Diffs

Real diffs from git-courer's own history, used for pipeline auditing and testing.

You can run the pipeline against any of these diffs to see what the classifier produces
for different commit types (feat, fix, refactor, chore, etc.).

## How to Audit

```bash
cd /git-courer
LLM_MODEL=qwen3.5:latest go test -tags e2e -v -count=1 ./test/pipeline/

# Check the audit directory:
cat /tmp/pipeline-audit/simple_fix/README.md
cat /tmp/pipeline-audit/simple_fix/05_classified.json
cat /tmp/pipeline-audit/simple_fix/04_annotated.json
cat /tmp/pipeline-audit/simple_fix/06_message.txt
```

## Reference Diffs

| File | Commit Type | Description | Files Changed |
|------|-------------|-------------|---------------|
| `refactor_21_files.diff` | refactor | Dependency injection refactor | 21 files |
| `feat_annotated_diff.diff` | feat | AnnotateDiffForRead feature | ~10 files |
| `feat_handler_wiring.diff` | feat | Handler wiring for ContentProvider | ~6 files |
| `fix_python_classifier.diff` | fix | Python signature detection fix | 5 files |
| `chore_preserve_best_type.diff` | chore | Preserve best CommitType | ~4 files |

## What Each Stage Produces

| Stage | File | Content |
|-------|------|---------|
| 00 | `00_request.json` | MCP request (instruction + preview flag) |
| 01 | `01_diff.txt` | Raw unified diff from git |
| 02 | `02_security.json` | Security check (blocked files) |
| 03 | `03_chunks.json` | Chunked diff with `commit_type`="" (not yet classified) |
| 04 | `04_annotated.json` | Annotated chunks with AST labels inline in `@@` headers |
| 05 | `05_classified.json` | Classified chunks with `commit_type` and `confidence_score` |
| 06 | `06_message.txt` | Plain text commit message from LLM |
| 07 | `07_result.json` | Final pipeline result |
| – | `annotated_diff.txt` | Human-readable extracted annotated diff |

## Data Flow (Not Sequential)

```
Request (00) → Diff (01) → Security (02)   ← both read diff from 01
                 Diff (01) → Chunks (03) → Annotation (04) → Classification (05) → LLM (06) → Result (07)
```

## Field Semantics in 05_classified.json

Each chunk in the classified output has:

- `files`: List of files in this chunk
- `diff`: **Empty string** when `annotated_diff` is populated (the annotated version replaces it)
- `annotated_diff`: Inline labels with diff lines (e.g. `@@ -1,5 +1,7 @@ [MOD_BODY_LOGIC: main]`)
- `commit_type`: Pre-classified type from AST analysis (`feat`, `fix`, `refactor`, `chore`, etc.)
- `confidence_score`: Classifier confidence (0.0-1.0)
- `scope`: Functional area (e.g. "security", "core"), empty if not configured