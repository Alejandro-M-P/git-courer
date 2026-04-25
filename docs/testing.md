# Testing Guide

git-courer uses a comprehensive, multi-layered testing strategy to ensure reliability, security, and AI response quality.

## Test Layers

### 1. Unit Tests (`make test-unit`)
Fast, isolated tests for core logic.
- **Location**: `internal/**/*_test.go`
- **Focus**: Chunker logic, config parsing, magic bytes detection, and domain models.
- **Requirement**: None (no LLM needed).

### 2. CI Suite (`make test-ci`)
The recommended command for Pull Requests and Merges.
- **Focus**: Runs all unit tests and core infrastructure checks.
- **Requirement**: None.

### 3. Quality Matrix (`make test-quality`)
Benchmarks the AI's "intelligence" and grounding.
- **Location**: `internal/adapters/llm/prompt_matrix_test.go`
- **Focus**:
    - **Grounding**: Ensures the AI doesn't "invent" code changes.
    - **Formatting**: Verifies JSON-strict output without markdown fences.
    - **Bilingualism**: Tests Spanish/English response accuracy.
- **Requirement**: Ollama running. Configure models via `GC_TEST_MODELS="model1,model2"`.

### 4. Armageddon E2E Torture (`make test-torture`)
Extreme stress tests designed to break the system.
- **Location**: `test/e2e/armageddon_test.go`
- **Scenarios**:
    - **Shell Injection**: Malicious branch names (e.g., `feat/x; rm -rf /`).
    - **Disguised Binaries**: Executables hidden as `.txt` files.
    - **Split Secrets**: Credentials fragmented across multiple lines.
    - **Massive Diffs**: 5000+ line changes.
- **Requirement**: Ollama running (uses `gemma4:26b` by default for maximum intelligence).

## The Ultimate Marathon (`make test-full`)
Runs all the above layers sequentially. **Green here means production-ready.**

## Troubleshooting Tests

### "Nothing to commit after staging"
This usually happens during integration tests if the LLM decides not to include untracked files. Try using a larger/smarter model like `qwen3.5:latest` or `gemma4:26b`.

### Model Not Available
If a model in the matrix is missing, the test will skip it. Pull it first:
```bash
ollama pull qwen3.5:0.8b
```

### Repo Locked Error
If a test fails mid-way, it might leave a `lock` in the temporary directory. `git-courer` automatically handles this in the E2E suite using `confirm.ReleaseLock()`.
