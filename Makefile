.PHONY: build test test-unit test-ollama test-all lint clean smoke

# ─── Build ────────────────────────────────────────────────────────────────────

build:
	go build -o git-courer ./cmd/main.go

# ─── Tests ────────────────────────────────────────────────────────────────────

# Unit tests (no Ollama needed)
# Runs quickly, used in CI
test-unit:
	go test ./internal/adapters/... ./internal/config/... ./internal/core/... ./internal/delivery/... ./internal/infra/... ./internal/installer/... ./internal/security/... ./internal/shared/... ./internal/workflow/... ./test/e2e/... -v -count=1

# Integration tests (requires Ollama running with qwen3.5:latest or similar)
# Must have: ollama serve
# To install model: ollama pull qwen3.5:latest
test-ollama:
	go test ./internal/integration/... -v -tags integration -count=1

# Quality benchmark: runs comprehensive prompt tests across models
# Usage: GC_TEST_MODELS="gemma4:26b,qwen3.5:0.8b" make test-quality
test-quality:
	go test ./internal/adapters/llm/ -v -tags integration -run "TestPromptQuality|TestPromptMatrix" -count=1

# Torture/Stress tests: runs Armageddon E2E, Extreme Stress, and Torture scenarios
# Warning: slow and resource intensive. Requires Ollama running.
test-torture:
	go test ./test/e2e/ -v -tags e2e -run "TestDiff5000Lines|TestShellInjection|TestFragmentedSecrets|TestMassiveFileCount|TestConcurrentOps" -count=1
	go test ./internal/adapters/llm/ -v -run "TestLarge|TestEmpty|TestMany|TestConcurrent|TestModel|TestMalformed" -count=1

# Torture tests only (requires Ollama running)
test-torture-ollama:
	OLLAMA_HOST=http://localhost:11434 OLLAMA_MODEL=qwen3.5:0.8b go test ./test/e2e/ -v -tags e2e -run "TestOllama" -count=1

# The Ultimate Test: runs EVERYTHING sequentially. 
# From quick build check to extreme E2E torture.
test-full: build test-unit test-ollama test-quality test-torture

# CI Test: Recommended for PRs and Merges. 
# Runs all unit tests (no Ollama required).
test-ci: test-unit

# All tests: unit + integration + quality + torture (requires Ollama)
# Usage: make test-all
# Requires: Ollama running with qwen3.5:latest
test-all: test-unit test-ollama

# Alias for test-all
test: test-all

# ─── Code Quality ─────────────────────────────────────────────────────────────

lint:
	go vet ./...

# ─── Utilities ────────────────────────────────────────────────────────────────

smoke: build
	@echo "✓ Build OK"

clean:
	rm -f git-courer
	go clean