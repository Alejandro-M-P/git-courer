package components

import (
	"testing"

	"github.com/blak0p/git-courer/internal/config"
)

func TestNewFormModel(t *testing.T) {
	cfg := config.Default()
	cfg.LLM.Enabled = true

	m := NewFormModel(cfg, 60)
	if len(m.fields) == 0 {
		t.Fatalf("Expected fields, got 0")
	}

	// First field should be LLM Enabled
	enabledField := m.fields[0]
	if enabledField.ID != "enabled" {
		t.Errorf("Expected first field to be 'enabled', got %q", enabledField.ID)
	}

	if *enabledField.Value != "true" {
		t.Errorf("Expected 'enabled' field value to be 'true', got %q", *enabledField.Value)
	}

	// Cycle option to "false"
	m.cursor = 0
	m.cycleOption(1)
	if *enabledField.Value != "false" {
		t.Errorf("Expected cycled value to be 'false', got %q", *enabledField.Value)
	}

	// Set temporary config path to save safely
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	err := m.cfg.SaveGlobal()
	if err != nil {
		t.Fatalf("SaveGlobal failed: %v", err)
	}

	if m.cfg.LLM.Enabled {
		t.Error("cfg.LLM.Enabled should be false after saving")
	}
}
