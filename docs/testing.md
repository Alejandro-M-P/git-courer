# Testing Guide

git-courer uses a comprehensive, multi-layered testing strategy to ensure reliability, security, and AI response quality.

## Test Targets

### 1. Unit Tests (`make test-unit`)
Fast, isolated tests for core logic.
- **Location**: `internal/**/*_test.go`
- **Focus**: Chunker logic, config parsing, magic bytes detection, and domain models.
- **Requirement**: None (no LLM needed).

### 2. Integration Tests (`make test-integration`)
Tests that run with a real Ollama instance.
- **Location**: `internal/integration/*_test.go`
- **Focus**: End-to-end workflows with actual LLM responses.
- **Requirement**: Ollama running.

### 3. E2E Tests (`make test-e2e`)
Full workflow tests simulating real user scenarios.
- **Location**: `test/e2e/*_test.go`
- **Focus**: Commit, release, branch, and tag operations.
- **Requirement**: Ollama running.

### 4. Torture Tests (`make test-torture`)
Extreme stress tests designed to break the system.
- **Location**: `test/e2e/torture_llm_test.go`, `test/e2e/torture_chunker_test.go`
- **Scenarios**:
    - **Shell Injection**: Malicious branch names (e.g., `feat/x; rm -rf /`).
    - **Disguised Binaries**: Executables hidden as `.txt` files.
    - **Split Secrets**: Credentials fragmented across multiple lines.
    - **Massive Diffs**: 5000+ line changes.
    - **High Concurrency**: Parallel commit operations.
- **Requirement**: Ollama running.

### 5. Full Suite (`make test` or `make test-full`)
Runs all test layers sequentially. **Green here means production-ready.**
- **Combines**: test-unit + test-integration + test-e2e + test-torture
- **Requirement**: Ollama running.

## Troubleshooting Tests

### "Nothing to commit after staging"
This usually happens during integration tests if the LLM decides not to include untracked files. Try using a larger/smarter model like `qwen3.5:latest` or `gemma4:26b`.

### Model Not Available
If a model is missing, the test will skip it. Pull it first:
```bash
ollama pull qwen3.5:0.8b
```

### Repo Locked Error
If a test fails mid-way, it might leave a `lock` in the temporary directory. `git-courer` automatically handles this in the E2E suite using `confirm.ReleaseLock()`.
