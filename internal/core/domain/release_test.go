package domain

import (
	"encoding/json"
	"testing"
)

func TestChangelog_Fields(t *testing.T) {
	ch := Changelog{
		Features: []string{"add login"},
		Fixes:    []string{"fix crash"},
		Breaking: []string{"remove old API"},
		Docs:     []string{"update readme"},
		Perf:     []string{"faster queries"},
		Internal: []string{"refactor"},
	}

	if len(ch.Features) != 1 || ch.Features[0] != "add login" {
		t.Errorf("Features: got %v", ch.Features)
	}
	if len(ch.Fixes) != 1 || ch.Fixes[0] != "fix crash" {
		t.Errorf("Fixes: got %v", ch.Fixes)
	}
	if len(ch.Breaking) != 1 || ch.Breaking[0] != "remove old API" {
		t.Errorf("Breaking: got %v", ch.Breaking)
	}
	if len(ch.Docs) != 1 || ch.Docs[0] != "update readme" {
		t.Errorf("Docs: got %v", ch.Docs)
	}
	if len(ch.Perf) != 1 || ch.Perf[0] != "faster queries" {
		t.Errorf("Perf: got %v", ch.Perf)
	}
	if len(ch.Internal) != 1 || ch.Internal[0] != "refactor" {
		t.Errorf("Internal: got %v", ch.Internal)
	}
}

func TestChangelog_JSONRoundTrip(t *testing.T) {
	original := Changelog{
		Features: []string{"add login", "add logout"},
		Fixes:    []string{"fix crash"},
		Breaking: []string{"remove API"},
		Docs:     []string{},
		Perf:     nil,
		Internal: []string{"refactor"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Changelog
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(decoded.Features) != 2 {
		t.Errorf("Features: got %d, want 2", len(decoded.Features))
	}
	if decoded.Features[0] != "add login" || decoded.Features[1] != "add logout" {
		t.Errorf("Features values: got %v", decoded.Features)
	}
	if len(decoded.Fixes) != 1 || decoded.Fixes[0] != "fix crash" {
		t.Errorf("Fixes: got %v", decoded.Fixes)
	}
	if len(decoded.Breaking) != 1 || decoded.Breaking[0] != "remove API" {
		t.Errorf("Breaking: got %v", decoded.Breaking)
	}
	if len(decoded.Docs) != 0 {
		t.Errorf("Docs: got %d, want 0", len(decoded.Docs))
	}
	if decoded.Perf != nil {
		t.Errorf("Perf: got %v, want nil", decoded.Perf)
	}
	if len(decoded.Internal) != 1 || decoded.Internal[0] != "refactor" {
		t.Errorf("Internal: got %v", decoded.Internal)
	}
}

func TestChangelog_JSONTags(t *testing.T) {
	// Verify JSON tags match the prompt schema (lowercase keys)
	data := []byte(`{"features":["f1"],"fixes":["f2"],"breaking":["b1"],"docs":["d1"],"perf":["p1"],"internal":["i1"]}`)
	var ch Changelog
	if err := json.Unmarshal(data, &ch); err != nil {
		t.Fatalf("unmarshal with lowercase keys: %v", err)
	}
	if len(ch.Features) != 1 || ch.Features[0] != "f1" {
		t.Errorf("Features from lowercase: got %v", ch.Features)
	}
}