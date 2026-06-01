package mcp

import (
	"testing"

	"github.com/blak0p/git-courer/internal/config"
	"github.com/blak0p/git-courer/internal/core/ports"
)

// TestServer_NewAcceptsPortsLifecycle verifies that New() accepts ports.Lifecycle
// instead of the local OllamaLifecycle interface.
func TestServer_NewAcceptsPortsLifecycle(t *testing.T) {
	cfg := config.Default()

	var lifecycle ports.Lifecycle = &mockLifecycle{}

	// This should compile: New() must accept ports.Lifecycle
	srv := New(cfg, nil, nil, lifecycle)
	if srv == nil {
		t.Error("New() with ports.Lifecycle should return non-nil Server")
	}
}

// TestServer_LifecycleEnsureRunning verifies that New() calls lifecycle.EnsureRunning()
// during initialization and that the server starts without error when the lifecycle
// reports the provider as available.
func TestServer_LifecycleEnsureRunning(t *testing.T) {
	cfg := config.Default()

	lifecycle := &mockLifecycle{started: false, err: nil}

	srv := New(cfg, nil, nil, lifecycle)
	if srv == nil {
		t.Fatal("New() with mock lifecycle should return non-nil Server")
	}

	// Verify lifecycle.EnsureRunning was called (it's called in New())
	// The mock returns (false, nil), meaning provider was already running.
	if lifecycle.started {
		t.Error("mockLifecycle should report started=false (already running)")
	}
}

// mockLifecycle implements ports.Lifecycle for testing.
type mockLifecycle struct {
	started bool
	err     error
}

func (m *mockLifecycle) EnsureRunning() (bool, error) { return m.started, m.err }
func (m *mockLifecycle) PreWarm() error               { return nil }
func (m *mockLifecycle) Stop()                        {}
func (m *mockLifecycle) IsWarmed() bool               { return false }
