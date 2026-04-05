package config

import (
	"testing"
)

// TestDefaultConfig tests default configuration values
func TestDefaultConfig(t *testing.T) {
	cfg := Default()

	// Ollama tests
	if cfg.Ollama.Host != "http://localhost:11434" {
		t.Errorf("Ollama.Host = %q, want %q", cfg.Ollama.Host, "http://localhost:11434")
	}
	if cfg.Ollama.Model != "llama3.2" {
		t.Errorf("Ollama.Model = %q, want %q", cfg.Ollama.Model, "llama3.2")
	}
	if cfg.Ollama.AutoStart != false {
		t.Errorf("Ollama.AutoStart = %v, want %v", cfg.Ollama.AutoStart, false)
	}

	// Git tests
	if cfg.Git.WorkDir != "." {
		t.Errorf("Git.WorkDir = %q, want %q", cfg.Git.WorkDir, ".")
	}
	if cfg.Git.AutoAddSecrets != true {
		t.Errorf("Git.AutoAddSecrets = %v, want %v", cfg.Git.AutoAddSecrets, true)
	}
	if cfg.Git.RequireCleanRepo != false {
		t.Errorf("Git.RequireCleanRepo = %v, want %v", cfg.Git.RequireCleanRepo, false)
	}

	// Validation tests
	if cfg.Validation.RequireConfirmation != true {
		t.Errorf("Validation.RequireConfirmation = %v, want %v", cfg.Validation.RequireConfirmation, true)
	}
	if cfg.Validation.MaxCommitLength != 72 {
		t.Errorf("Validation.MaxCommitLength = %v, want %v", cfg.Validation.MaxCommitLength, 72)
	}

	// UI tests
	if cfg.UI.Theme != "dark" {
		t.Errorf("UI.Theme = %q, want %q", cfg.UI.Theme, "dark")
	}
	if cfg.UI.ShowIcons != true {
		t.Errorf("UI.ShowIcons = %v, want %v", cfg.UI.ShowIcons, true)
	}

	// MCP tests
	if cfg.MCP.Name != "git-courer" {
		t.Errorf("MCP.Name = %q, want %q", cfg.MCP.Name, "git-courer")
	}
	if cfg.MCP.Version != "1.0.0" {
		t.Errorf("MCP.Version = %q, want %q", cfg.MCP.Version, "1.0.0")
	}
}

// TestConfigLoad tests loading config from file
func TestConfigLoad(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Errorf("Load() error = %v", err)
		return
	}

	// Should return default config if no file exists
	if cfg.Ollama.Host != "http://localhost:11434" {
		t.Errorf("Ollama.Host = %q, want default", cfg.Ollama.Host)
	}
}
