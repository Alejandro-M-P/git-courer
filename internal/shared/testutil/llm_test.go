package testutil_test

import (
	"os"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/shared/testutil"
)

func TestRequireLLM_CompilesAndReturnsAdapter(t *testing.T) {
	t.Parallel()
	// This test verifies RequireLLM compiles and returns a non-nil adapter
	// when an LLM service is available. It will skip if no service is running.
	llm := testutil.RequireLLM(t)
	if llm == nil {
		t.Fatal("expected llm to be not nil")
	}
}

func TestRequireOllama_DeprecatedAlias(t *testing.T) {
	t.Parallel()
	// Verify that the deprecated alias still works
	llm := testutil.RequireOllama(t)
	if llm == nil {
		t.Fatal("expected llm to be not nil via deprecated alias")
	}
}

func TestEnvOr2_ModuleVarsInitialized(t *testing.T) {
	t.Parallel()
	// Verify that the new exported vars exist and have sensible defaults.
	// These are set at package init from env vars, so default values
	// should reflect the fallback chain: LLM_* → OLLAMA_* → default.
	if testutil.LLMBaseURL == "" {
		t.Error("LLMBaseURL should not be empty")
	}
	if testutil.LLMModel == "" {
		t.Error("LLMModel should not be empty")
	}
	// LLMAPIKey can legitimately be empty (no API key for local LLM)
}

func TestEnvOr2_PrimaryTakesPrecedence(t *testing.T) {
	t.Parallel()
	// envOr2 should return default when neither primary nor fallback is set
	// (using unique test keys that aren't set in the environment)
	got := testutil.EnvOr2("TEST_ENVOR2_NEITHER_A", "TEST_ENVOR2_NEITHER_B", "http://default:9999")
	if got != "http://default:9999" {
		t.Errorf("envOr2 with neither set = %q, want default", got)
	}
}

func TestEnvOr2_WithPrimary(t *testing.T) {
	t.Parallel()
	os.Setenv("TEST_ENVOR2_PRIMARY", "primary_value")
	defer os.Unsetenv("TEST_ENVOR2_PRIMARY")

	got := testutil.EnvOr2("TEST_ENVOR2_PRIMARY", "TEST_ENVOR2_FALLBACK", "default_value")
	if got != "primary_value" {
		t.Errorf("envOr2 with primary set = %q, want primary_value", got)
	}
}

func TestEnvOr2_WithFallbackOnly(t *testing.T) {
	t.Parallel()
	os.Setenv("TEST_ENVOR2_FALLBACK_ONLY", "fallback_value")
	defer os.Unsetenv("TEST_ENVOR2_FALLBACK_ONLY")

	got := testutil.EnvOr2("TEST_ENVOR2_NONEXISTENT", "TEST_ENVOR2_FALLBACK_ONLY", "default_value")
	if got != "fallback_value" {
		t.Errorf("envOr2 with fallback only = %q, want fallback_value", got)
	}
}

func TestEnvOr2_BothSet(t *testing.T) {
	t.Parallel()
	os.Setenv("TEST_ENVOR2_BOTH_PRI", "primary_wins")
	os.Setenv("TEST_ENVOR2_BOTH_FB", "fallback_loses")
	defer os.Unsetenv("TEST_ENVOR2_BOTH_PRI")
	defer os.Unsetenv("TEST_ENVOR2_BOTH_FB")

	got := testutil.EnvOr2("TEST_ENVOR2_BOTH_PRI", "TEST_ENVOR2_BOTH_FB", "default_value")
	if got != "primary_wins" {
		t.Errorf("envOr2 with both set = %q, want primary_wins", got)
	}
}

