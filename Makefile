.PHONY: build test test-unit test-integration test-e2e test-torture test-full lint clean help

# ─── Build ────────────────────────────────────────────────────────────────────

build:
	@echo "Building git-courer..."
	@go build -o git-courer ./cmd/main.go
	@echo "✓ Build complete"

# ─── Tests ────────────────────────────────────────────────────────────────────

GOTESTSUM_FORMAT ?= pkgname-and-test-fails
GOTESTSUM := $(shell which gotestsum 2>/dev/null || echo "$(HOME)/go/bin/gotestsum")

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

# Full E2E suite (requires Ollama) — commit, branch, gitops, workflow, connection
test-e2e:
	@echo "Running e2e tests..."
	$(call run_test,./test/e2e/... -tags e2e)
	@echo "✓ E2E tests passed"

# Torture/Stress tests only
test-torture:
	@echo "Running torture tests..."
	$(call run_test,./test/e2e/... -tags e2e -run "TestTorture")
	@echo "✓ Torture tests passed"

# Run all suites, always generate the telemetry report at the end
test-full: build
	@failed=0; \
	echo "Running unit tests..."; \
	$(MAKE) --no-print-directory test-unit      || failed=1; \
	echo "Running integration tests..."; \
	$(MAKE) --no-print-directory test-integration || failed=1; \
	echo "Running e2e tests..."; \
	$(MAKE) --no-print-directory test-e2e        || failed=1; \
	echo ""; \
	echo "Generating Telemetry Report..."; \
	go run ./cmd/gcourer-report/main.go; \
	echo ""; \
	if [ $$failed -ne 0 ]; then \
		echo "✖ Some test suites failed (see above)"; \
		exit 1; \
	else \
		echo "✓ Full test cycle complete"; \
	fi

# Alias
test: test-full

# ─── Quality ──────────────────────────────────────────────────────────────────

lint:
	@echo "Running lint..."
	@go vet ./...
	@echo "✓ Lint passed"

# ─── Utilities ────────────────────────────────────────────────────────────────

clean:
	@rm -f git-courer
	@go clean
	@echo "✓ Clean complete"

help:
	@echo "Git Courer Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  build              Build binary"
	@echo "  test-unit          Unit tests (no Ollama)"
	@echo "  test-integration   Integration tests (requires Ollama)"
	@echo "  test-e2e           Full E2E suite (requires Ollama)"
	@echo "  test-torture       Stress/torture tests only"
	@echo "  test-full          Run everything + telemetry report (always)"
	@echo "  lint               Static analysis"
	@echo "  clean              Remove artifacts"
