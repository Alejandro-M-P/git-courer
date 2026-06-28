# Testing Guide

git-courer utilizes a comprehensive testing strategy combining unit tests and E2E pipeline tests to guarantee correctness, reliability, and security of git mutations.

---

## Test Targets

All tests are configured in the [Makefile](file:///git-courer/git-courer/Makefile).

### 1. Unit Tests (`make test-unit` or `make test`)
Isolated, fast-running tests that verify core logic, config parsing, AST path type resolutions, and security rule matchers.
- **Location**: `internal/**/*_test.go`
- **Focus**: Chunker parsing, classification logic, magic byte detection, config validation, and helpers.
- **Requirement**: No external dependencies or running LLM services required.

### 2. Pipeline E2E Tests (`make test-e2e`)
Runs the full commit pipeline (PREVIEW and APPLY) and release flows using a real LLM instance.
- **Location**: `test/pipeline/` and `test/release/`
- **Focus**: Committing changes, generating changelogs, staging files, and verifying AST annotations with live model feedback.
- **Prerequisite**: An active local Ollama instance (recommended model: `qwen3.5:latest`) or an OpenAI-compatible API endpoint configured via the environment variables `LLM_BASE_URL` and `LLM_MODEL`.

---

## E2E Testing Prerequisites

Before running `make test-e2e`, ensure that:
1. An LLM service is running (e.g., `ollama serve`).
2. The model used for testing is pulled locally (e.g., `ollama pull qwen3.5:latest` or any model you prefer).
3. If using a remote provider, set the endpoint:
   ```bash
   export LLM_BASE_URL="https://api.openai.com/v1"
   export LLM_MODEL="gpt-4o-mini"
   ```

---

## Troubleshooting Tests

### LLM Service Not Available
If `make test-e2e` fails at the validation step, verify that your local service is active:
```bash
curl -sf http://localhost:11434/api/tags
```

### Repo Locked Error
E2E tests create temporary git repositories in `t.TempDir()`. If a test fails in the middle of execution, it might leave an active lock in the confirm store. Clean the temporary paths or rerun the tests; locks in tests are scoped to their respective temporary directories and do not impact the host repository.
