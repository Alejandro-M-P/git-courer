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

# All tests: unit + integration (requires Ollama)
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