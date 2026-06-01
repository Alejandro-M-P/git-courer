package ollama

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"time"

	"github.com/blak0p/git-courer/internal/core/ports"
)

// Compile-time interface check: OllamaLifecycle satisfies ports.Lifecycle.
var _ ports.Lifecycle = (*OllamaLifecycle)(nil)

// OllamaLifecycle manages the Ollama server process.
// It implements ports.Lifecycle for process startup and shutdown.
// All LLM functionality delegates to OpenAIStandardAdapter.
type OllamaLifecycle struct {
	host        string
	modelsDir   string
	process     *exec.Cmd
	startedByUs bool
	autoStart   bool
	httpClient  *http.Client

	// preWarm delegates to the adapter's PreWarm for model warming.
	// When nil, PreWarm is a no-op (backward compatible default).
	preWarm func() error

	// Test-injectable hooks
	findBinaryFunc func() string
	startCommandFn func(cmd *exec.Cmd) error

	// Test-configurable
	pollInterval    time.Duration
	maxPollAttempts int
}

// LifecycleOption is a functional option for OllamaLifecycle configuration.
type LifecycleOption func(*OllamaLifecycle)

// WithLifecycleHTTPClient sets a custom HTTP client for testing.
func WithLifecycleHTTPClient(client *http.Client) LifecycleOption {
	return func(ol *OllamaLifecycle) { ol.httpClient = client }
}

// WithLifecycleFindBinary overrides the findOllamaBinary function for testing.
func WithLifecycleFindBinary(fn func() string) LifecycleOption {
	return func(ol *OllamaLifecycle) { ol.findBinaryFunc = fn }
}

// WithLifecycleStartCommand overrides the command start function for testing.
func WithLifecycleStartCommand(fn func(cmd *exec.Cmd) error) LifecycleOption {
	return func(ol *OllamaLifecycle) { ol.startCommandFn = fn }
}

// WithPreWarm sets a function that the lifecycle's PreWarm method delegates to.
// This decouples OllamaLifecycle from the adapter package — the factory wires
// adapter.PreWarm into the lifecycle via this option.
func WithPreWarm(fn func() error) LifecycleOption {
	return func(ol *OllamaLifecycle) { ol.preWarm = fn }
}

// NewOllamaLifecycle creates a new OllamaLifecycle for the given host.
// When autoStart is true, EnsureRunning will attempt to start Ollama if it's not available.
func NewOllamaLifecycle(host, modelsDir string, autoStart bool, opts ...LifecycleOption) *OllamaLifecycle {
	ol := &OllamaLifecycle{
		host:            host,
		modelsDir:       modelsDir,
		autoStart:       autoStart,
		httpClient:      &http.Client{Timeout: 10 * time.Second},
		pollInterval:    1 * time.Second,
		maxPollAttempts: 30,
	}
	for _, opt := range opts {
		opt(ol)
	}
	return ol
}

// EnsureRunning checks if Ollama is available and starts it if needed.
// Returns true if the provider was started by this call.
func (ol *OllamaLifecycle) EnsureRunning() (bool, error) {
	if ol.isAvailable() {
		return false, nil
	}

	if !ol.autoStart {
		return false, fmt.Errorf("Ollama not available and auto_start is disabled")
	}

	fmt.Println("🚀 Ollama está apagado. Arrancando motor local...")

	ollamaPath := ol.findBinary()
	if ollamaPath == "" {
		return false, fmt.Errorf("ollama binary not found. Install from https://ollama.com")
	}
	fmt.Printf("  Using ollama at: %s\n", ollamaPath)

	currentUser, err := user.Current()
	if err != nil {
		return false, fmt.Errorf("could not determine current user: %v", err)
	}

	cmd := exec.Command(ollamaPath, "serve")
	cmd.Env = append(os.Environ(), "HOME="+currentUser.HomeDir)
	if ol.modelsDir != "" {
		cmd.Env = append(cmd.Env, "OLLAMA_MODELS="+ol.modelsDir)
	}

	if ol.startCommandFn != nil {
		if err := ol.startCommandFn(cmd); err != nil {
			return false, fmt.Errorf("error starting Ollama: %v", err)
		}
	} else {
		if err := cmd.Start(); err != nil {
			return false, fmt.Errorf("error starting Ollama: %v", err)
		}
	}
	ol.process = cmd
	ol.startedByUs = true

	// Poll until Ollama is available
	for i := 0; i < ol.maxPollAttempts; i++ {
		time.Sleep(ol.pollInterval)
		if ol.isAvailable() {
			fmt.Println("✓ Ollama ready!")
			return true, nil
		}
	}
	return false, fmt.Errorf("Ollama took too long to start")
}

// Stop gracefully shuts down the provider if it was started by EnsureRunning.
// Per current behavior, it does NOT kill the Ollama process — just clears
// the startedByUs flag so the process keeps running for next use.
func (ol *OllamaLifecycle) Stop() {
	if ol.startedByUs {
		fmt.Println("🛑 Leaving Ollama running for next use...")
		ol.process = nil
		ol.startedByUs = false
	}
}

// PreWarm delegates to the adapter's PreWarm when wired via WithPreWarm.
// This lifecycle only manages the server process itself; model warming
// is handled by the OpenAIStandardAdapter.
func (ol *OllamaLifecycle) PreWarm() error {
	if ol.preWarm != nil {
		return ol.preWarm()
	}
	return nil
}

// IsWarmed returns true if the server process is running.
func (ol *OllamaLifecycle) IsWarmed() bool {
	return ol.startedByUs || ol.isAvailable()
}

// isAvailable checks if Ollama is reachable via the OpenAI Standard /v1/models endpoint.
func (ol *OllamaLifecycle) isAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", ol.host+"/v1/models", nil)
	if err != nil {
		return false
	}
	resp, err := ol.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// findBinary locates the ollama executable.
func (ol *OllamaLifecycle) findBinary() string {
	if ol.findBinaryFunc != nil {
		return ol.findBinaryFunc()
	}
	paths := []string{
		"/usr/local/bin/ollama",
		"/usr/bin/ollama",
		"/opt/ollama/bin/ollama",
		"/snap/bin/ollama",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
