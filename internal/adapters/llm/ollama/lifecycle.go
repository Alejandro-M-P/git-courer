package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// OllamaLifecycle manages Ollama-specific startup, model pre-warming, and shutdown.
// It implements ports.Lifecycle for the Ollama provider.
type OllamaLifecycle struct {
	host        string
	model       string
	modelsDir   string
	process     *exec.Cmd
	startedByUs bool
	resolver    *ModelResolver
	autoStart   bool
	httpClient  *http.Client

	// Test-injectable hooks
	findBinaryFunc  func() string
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

// NewOllamaLifecycle creates a new OllamaLifecycle for the given host, model, and modelsDir.
// When autoStart is true, EnsureRunning will attempt to start Ollama if it's not available.
func NewOllamaLifecycle(host, model, modelsDir string, autoStart bool, opts ...LifecycleOption) *OllamaLifecycle {
	ol := &OllamaLifecycle{
		host:            host,
		model:           model,
		modelsDir:       modelsDir,
		autoStart:       autoStart,
		httpClient:      &http.Client{Timeout: 10 * time.Second},
		pollInterval:    1 * time.Second,
		maxPollAttempts: 30,
	}
	for _, opt := range opts {
		opt(ol)
	}
	ol.resolver = NewModelResolver(host, model, WithResolverHTTPClient(ol.httpClient))
	return ol
}

// EnsureRunning checks if Ollama is available and starts it if needed.
// Returns true if the provider was started by this call.
// After ensuring the server is running, resolves the model.
func (ol *OllamaLifecycle) EnsureRunning() (bool, error) {
	if ol.IsAvailable() {
		if err := ol.resolver.Resolve(); err != nil {
			return false, err
		}
		ol.model = ol.resolver.Model()
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
		if ol.IsAvailable() {
			fmt.Println("✓ Ollama ready!")
			if err := ol.resolver.Resolve(); err != nil {
				return true, err
			}
			ol.model = ol.resolver.Model()
			return true, nil
		}
	}
	return false, fmt.Errorf("Ollama took too long to start")
}

// PreWarm loads the model into memory before the first request.
// Sends a minimal request to /api/generate with a 120s timeout.
func (ol *OllamaLifecycle) PreWarm() error {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	reqBody := map[string]interface{}{
		"model": ol.model, "prompt": ".", "stream": false,
		"options": map[string]interface{}{"temperature": 0.0, "num_predict": 1},
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", ol.host+"/api/generate", bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := ol.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("model %q failed to load: %w", ol.model, err)
	}
	defer resp.Body.Close()
	fmt.Printf("✓ Model %q loaded in memory\n", ol.model)
	return nil
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

// IsAvailable checks if Ollama is reachable via /api/tags.
func (ol *OllamaLifecycle) IsAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", ol.host+"/api/tags", nil)
	if err != nil {
		return false
	}
	resp, err := ol.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// findBinary returns the path to the Ollama binary, using the injectable
// findBinaryFunc for testing, or the default findOllamaBinary function.
func (ol *OllamaLifecycle) findBinary() string {
	if ol.findBinaryFunc != nil {
		return ol.findBinaryFunc()
	}
	return findOllamaBinary()
}

// findOllamaBinary searches standard paths and PATH for the Ollama binary.
func findOllamaBinary() string {
	locations := []string{
		"/usr/local/bin/ollama", "/usr/bin/ollama",
		"/opt/homebrew/bin/ollama", "/snap/bin/ollama",
	}
	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}
	if path, err := exec.LookPath("ollama"); err == nil {
		return path
	}
	return ""
}

// timeoutForAttempt returns the timeout duration for the given attempt index,
// matching the current Ollama adapter behavior.
func timeoutForAttempt(attempt int) time.Duration {
	switch {
	case attempt >= 4:
		return 180 * time.Second
	case attempt >= 2:
		return 120 * time.Second
	default:
		return 60 * time.Second
	}
}

// Compile-time interface check: OllamaLifecycle satisfies ports.Lifecycle.
var _ ports.Lifecycle = (*OllamaLifecycle)(nil)