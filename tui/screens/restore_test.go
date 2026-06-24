// Package screens provides the TUI wizard screens for git-courer.
package screens

import (
	"strings"
	"testing"
)

// TestRestoreScreen_Init verifies a freshly created RestoreScreen starts at
// step 0 (confirmation), not yet done.
func TestRestoreScreen_Init(t *testing.T) {
	m := NewRestoreScreen(80)

	if m.step != 0 {
		t.Errorf("initial step: got %d, want 0", m.step)
	}
	if m.Done() {
		t.Error("fresh RestoreScreen should not be Done()")
	}
	if m.Init() != nil {
		t.Error("Init should return nil cmd")
	}
}

// TestRestoreScreen_Done verifies Done returns true only after the screen
// reaches step 2 (done).
func TestRestoreScreen_Done(t *testing.T) {
	m := NewRestoreScreen(80)

	if m.Done() {
		t.Error("step 0 should not be Done()")
	}

	m.step = 1
	if m.Done() {
		t.Error("step 1 should not be Done()")
	}

	m.step = 2
	if !m.Done() {
		t.Error("step 2 should be Done()")
	}
}

// TestRestoreScreen_View renders confirmation text at step 0.
func TestRestoreScreen_View(t *testing.T) {
	m := NewRestoreScreen(80)
	view := m.View()

	// Per the spec, the confirmation text must mention restoring configs and
	// removing hooks.
	if !strings.Contains(view, "restore") {
		t.Errorf("confirmation view missing 'restore'\nview:\n%s", view)
	}
	if !strings.Contains(view, "hooks") {
		t.Errorf("confirmation view missing 'hooks'\nview:\n%s", view)
	}
}