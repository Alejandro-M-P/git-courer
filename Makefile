.PHONY: build test test-unit test-e2e lint clean help

# ─── Build ────────────────────────────────────────────────────────────────────

# Literal dollar sign for shell escaping
S := $$

build:
	@echo "Building git-courer (self-contained)..."
	@CGO_ENABLED=1 CGO_LDFLAGS="-Wl,-Bstatic -L$(shell pwd)/target/release -lts_pack_core_ffi -Wl,-Bdynamic -lpthread -ldl" go build -o git-courer ./cmd/main.go
	@echo "✓ Build complete (single binary created)"

# ─── Tests ────────────────────────────────────────────────────────────────────

GOTESTSUM_FORMAT ?= pkgname-and-test-fails
GOTESTSUM := $(shell which gotestsum 2>/dev/null || echo "$(HOME)/go/bin/gotestsum")

define run_test
	@if [ -x "$(GOTESTSUM)" ]; then \
		TELEMETRY=1 CGO_ENABLED=1 CGO_LDFLAGS="-L$(shell pwd)/target/release -Wl,-rpath,$(shell pwd)/target/release" $(GOTESTSUM) --format $(GOTESTSUM_FORMAT) -- $(1) -count=1; \
	else \
		TELEMETRY=1 CGO_ENABLED=1 CGO_LDFLAGS="-L$(shell pwd)/target/release -Wl,-rpath,$(shell pwd)/target/release" go test $(1) -count=1; \
	fi
endef

# Unit tests (no external dependencies needed)
# Runs go vet first, then all unit tests
test-unit:
	@echo "Running vet..."
	@go vet ./...
	@echo "✓ Vet passed"
	@echo "Running unit tests..."
	$(call run_test,./...)
	@echo "✓ Unit tests passed"

# E2E pipeline tests (requires Ollama running locally)
# Runs stages 0-7 of the commit message pipeline with real LLM
# Prerequisites: Ollama must be running with a model loaded
#   - Install: https://ollama.com
#   - Start: ollama serve
#   - Pull model: ollama pull <model>
test-e2e:
	@echo "Checking Ollama availability..."
	@curl -sf http://localhost:11434/api/tags > /dev/null 2>&1 || \
		{ echo "✖ Ollama is not running. Start it with: ollama serve"; exit 1; }
	@echo "✓ Ollama is running"
	@echo "Running e2e pipeline tests..."
	$(call run_test,-tags e2e ./test/pipeline/...)
	@echo "✓ E2E tests passed"

# Alias: test runs unit tests by default
test: test-unit

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
	@echo "  test-unit          Unit tests + vet (no Ollama needed)"
	@echo "  test-e2e           Pipeline E2E tests (requires Ollama)"
	@echo "  test               Alias for test-unit"
	@echo "  lint               Static analysis"
	@echo "  clean              Remove artifacts"
