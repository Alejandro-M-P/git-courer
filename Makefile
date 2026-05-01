.PHONY: build test test-unit test-integration test-torture test-full lint clean help

# ─── Build ────────────────────────────────────────────────────────────────────

# Build the git-courer binary
build:
	@echo "Building git-courer..."
	@go build -o git-courer ./cmd/main.go
	@echo "✓ Build complete"

# ─── Tests ────────────────────────────────────────────────────────────────────

GOTESTSUM_FORMAT ?= pkgname-and-test-fails
GOTESTSUM := $(shell which gotestsum 2>/dev/null || echo "$(HOME)/go/bin/gotestsum")

# Internal helper to run tests
define run_test
	@if [ -x "$(GOTESTSUM)" ]; then \
		TELEMETRY=1 $(GOTESTSUM) --format $(GOTESTSUM_FORMAT) -- $(1) -count=1; \
	else \
		TELEMETRY=1 go test $(1) -count=1; \
	fi
endef

# Unit tests (no Ollama needed)
test-unit:
	@echo "Running unit tests..."
	$(call run_test,./...)
	@echo "✓ Unit tests passed"

# Integration tests (requires Ollama)
test-integration:
	@echo "Running integration tests..."
	$(call run_test,./internal/integration/... -tags integration)
	@echo "✓ Integration tests passed"

# Torture/Stress tests
test-torture:
	@echo "Running torture tests..."
	$(call run_test,./test/e2e/... -tags e2e -run "TestTorture")
	@echo "✓ Torture tests passed"

# The Ultimate Test: Sequential run of all suites with final telemetry report
test-full: build test-unit test-integration test-torture
	@echo "\nGenerating Telemetry Report..."
	@go run ./cmd/gcourer-report/main.go
	@echo "\n✓ Full test cycle complete"

# Alias for full test suite
test: test-full

# ─── Quality ──────────────────────────────────────────────────────────────────

# Static analysis and linting
lint:
	@echo "Running lint..."
	@go vet ./...
	@echo "✓ Lint passed"

# ─── Utilities ────────────────────────────────────────────────────────────────

# Remove build artifacts
clean:
	@rm -f git-courer
	@go clean
	@echo "✓ Clean complete"

# Show help
help:
	@echo "Git Courer Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  build            Build binary"
	@echo "  test-unit        Run unit tests"
	@echo "  test-integration Run integration tests (requires Ollama)"
	@echo "  test-torture     Run stress/torture tests"
	@echo "  test-full        Run everything and show report"
	@echo "  lint             Run static analysis"
	@echo "  clean            Remove artifacts"
