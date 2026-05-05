package domain

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectConfig_LoadSave(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	config := &ProjectConfig{
		Description: "A git commit helper",
		Areas: map[string][]string{
			"security": {"internal/auth", "internal/crypto"},
		},
	}

	if err := config.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error = %v", err)
	}

	if loaded.Description != config.Description {
		t.Errorf("Description = %q, want %q", loaded.Description, config.Description)
	}
	if len(loaded.Areas["security"]) != 2 {
		t.Errorf("Areas[security] len = %d, want 2", len(loaded.Areas["security"]))
	}
}

func TestProjectConfig_MissingConfig(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	loaded, err := LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error = %v", err)
	}
	if loaded.Description != "" {
		t.Errorf("Description = %q, want empty string", loaded.Description)
	}
	if loaded.Areas == nil {
		t.Error("Areas should be empty map, not nil")
	}
	if len(loaded.Areas) != 0 {
		t.Errorf("len(Areas) = %d, want 0", len(loaded.Areas))
	}
}

func TestProjectConfig_MalformedJSON(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, ".git-courer")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "config.json"), []byte("not json"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := LoadProjectConfig(tmpDir)
	if err == nil {
		t.Fatal("LoadProjectConfig() expected error for malformed JSON, got nil")
	}
}

func TestProjectConfig_ResolveScope_SingleMatch(t *testing.T) {
	t.Parallel()
	config := &ProjectConfig{
		Areas: map[string][]string{
			"security": {"internal/auth", "internal/crypto"},
		},
	}
	scope := config.ResolveScope([]string{"internal/auth/login.go", "internal/auth/tokens.go"})
	if scope != "security" {
		t.Errorf("ResolveScope = %q, want security", scope)
	}
}

func TestProjectConfig_ResolveScope_MultipleAreasTie(t *testing.T) {
	t.Parallel()
	config := &ProjectConfig{
		Areas: map[string][]string{
			"core":  {"internal/core"},
			"tui":   {"internal/ui"},
		},
	}
	scope := config.ResolveScope([]string{"internal/core/domain.go", "internal/ui/screen.go"})
	if scope != "core" {
		t.Errorf("ResolveScope = %q, want core (first in config wins)", scope)
	}
}

func TestProjectConfig_ResolveScope_NoMatch(t *testing.T) {
	t.Parallel()
	config := &ProjectConfig{
		Areas: map[string][]string{
			"security": {"internal/auth"},
		},
	}
	scope := config.ResolveScope([]string{"pkg/utils/helpers.go"})
	if scope != "" {
		t.Errorf("ResolveScope = %q, want empty string", scope)
	}
}

func TestProjectConfig_ResolveScope_NoAreasConfigured(t *testing.T) {
	t.Parallel()
	config := &ProjectConfig{}
	scope := config.ResolveScope([]string{"internal/auth/login.go"})
	if scope != "" {
		t.Errorf("ResolveScope = %q, want empty string", scope)
	}
}

func TestProjectConfig_ResolveScope_MostFilesWins(t *testing.T) {
	t.Parallel()
	config := &ProjectConfig{
		Areas: map[string][]string{
			"security": {"internal/auth"},
			"core":     {"internal/core"},
		},
	}
	scope := config.ResolveScope([]string{
		"internal/core/domain.go",
		"internal/core/ports.go",
		"internal/auth/login.go",
	})
	if scope != "core" {
		t.Errorf("ResolveScope = %q, want core (most files wins)", scope)
	}
}