# Testing Guide

git-courer utilizes a comprehensive testing strategy combining unit tests and E2E pipeline tests to guarantee correctness, reliability, and security of git mutations.

---

## Build Requirement: `CGO_ENABLED=1`

git-courer uses cgo bindings, so the binary and all tests **must** be built with `CGO_ENABLED=1`. The Makefile sets this for you on every target; if you invoke `go build` or `go test` directly, export it yourself:

```bash
CGO_ENABLED=1 go build -o git-courer ./cmd/main.go
CGO_ENABLED=1 go test ./...
```

---

## Make Targets

All targets are defined in the [Makefile](file:///git-courer/git-courer/Makefile):

| Target | Purpose |
|--------|---------|
| `make build` | Build the self-contained `git-courer` binary (`CGO_ENABLED=1 go build`). |
| `make test-unit` (alias: `make test`) | Run `go vet` then unit tests with the `-short` flag. No external services needed. |
| `make test-e2e` | Run E2E pipeline tests with `-tags e2e`. Requires a live LLM service. |
| `make lint` | `go vet ./...`. |
| `make clean` | Remove the built binary and clear the Go build cache. |
| `make help` | Print the target list. |

---

## `gotestsum` (Optional)

The Makefile auto-detects [gotestsum](https://github.com/gotestyourself/gotestsum) on your `PATH` (or at `~/go/bin/gotestsum`). If found, tests are run through `gotestsum` with the `pkgname-and-test-fails` format for readable output; otherwise it falls back to plain `go test`. No configuration is needed — install gotestsum to get the nicer output:

```bash
go install gotest.tools/gotestsum@latest
```

---

## Test Suite

### 1. Unit Tests (`make test-unit`)

Isolated, fast-running tests that verify core logic, config parsing, AST path type resolutions, and security rule matchers.

- **Location**: `internal/**/*_test.go`
- **Flag**: `-short` — skips long-running or integration-style tests, keeping the unit run fast.
- **Focus**: Chunker parsing, classification logic, magic byte detection, config validation, and helpers.
- **Requirement**: No external dependencies or running LLM services required.

### 2. Pipeline E2E Tests (`make test-e2e`)

Runs the full commit pipeline (PREVIEW and APPLY) and release flows using a real LLM instance.

- **Location**: `test/pipeline/` and `test/release/`
- **Flag**: `-tags e2e` — the build tag that enables E2E test files.
- **Focus**: Committing changes, generating changelogs, staging files, and verifying AST annotations with live model feedback.
- **Prerequisite**: An active local Ollama instance (recommended model: `qwen3.5:latest`) or an OpenAI-compatible API endpoint configured via the environment variables `LLM_BASE_URL` and `LLM_MODEL`.

#### Test Directory Structure

| Directory | Contents |
|-----------|---------|
| `test/pipeline/` | End-to-end commit pipeline tests (PREVIEW → APPLY, AST annotations, security blocks). |
| `test/release/` | Release-flow tests (changelog generation, version bumping, tagging). |

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