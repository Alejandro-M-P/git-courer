package data

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTestPattern_MarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name string
		pattern TestPattern
		jsonStr string
	}{
		{
			name: "suffix pattern",
			pattern: TestPattern{
				Type: "suffix",
				Value: "_test.go",
				SamePackage: true,
			},
			jsonStr: `{"type":"suffix","value":"_test.go","same_package":true}`,
		},
		{
			name: "prefix pattern",
			pattern: TestPattern{
				Type: "prefix",
				Value: "test_",
			},
			jsonStr: `{"type":"prefix","value":"test_"}`,
		},
		{
			name: "import_match pattern",
			pattern: TestPattern{
				Type: "import_match",
				Value: "",
				Fallback: true,
			},
			jsonStr: `{"type":"import_match","value":"","fallback":true}`,
		},
		{
			name: "pattern with in_dir",
			pattern: TestPattern{
				Type: "prefix",
				Value: "test_",
				InDir: "tests/",
			},
			jsonStr: `{"type":"prefix","value":"test_","in_dir":"tests/"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			data, err := json.Marshal(tt.pattern)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}
			if string(data) != tt.jsonStr {
				t.Errorf("Marshal got %s, want %s", string(data), tt.jsonStr)
			}

			// Test unmarshaling
			var parsed TestPattern
			err = json.Unmarshal([]byte(tt.jsonStr), &parsed)
			if err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if parsed != tt.pattern {
				t.Errorf("Unmarshal got %+v, want %+v", parsed, tt.pattern)
			}
		})
	}
}

func TestLanguageNodes_WithTestPatterns(t *testing.T) {
	nodes := LanguageNodes{
		Functions: []string{"function_declaration"},
		Types:     []string{"type_declaration"},
		TestPatterns: []TestPattern{
			{
				Type: "suffix",
				Value: "_test.go",
				SamePackage: true,
			},
		},
	}

	data, err := json.Marshal(nodes)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var parsed LanguageNodes
	err = json.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(parsed.TestPatterns) != 1 {
		t.Errorf("Expected 1 test pattern, got %d", len(parsed.TestPatterns))
	}
	if parsed.TestPatterns[0].Type != "suffix" {
		t.Errorf("Expected suffix type, got %s", parsed.TestPatterns[0].Type)
	}
}

func TestGetAllLanguageNames(t *testing.T) {
	names := GetAllLanguageNames()
	
	if len(names) == 0 {
		t.Fatal("Expected some language names")
	}
	
	// Check that some common languages are present
	expectedLanguages := []string{"Go", "Python", "JavaScript", "TypeScript", "Java", "Ruby"}
	for _, lang := range expectedLanguages {
		found := false
		for _, name := range names {
			if name == lang {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected language %s to be in the list", lang)
		}
	}
}

func TestControlFlowCategory_RoundTrip(t *testing.T) {
	original := ControlFlowCategory{
		Branch: []string{"if_statement", "else_clause"},
		Loop:   []string{"for_statement", "while_statement"},
		Return: []string{"return_statement"},
		Error:  []string{"try_statement", "catch_clause"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var parsed ControlFlowCategory
	err = json.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(parsed.Branch) != 2 || parsed.Branch[0] != "if_statement" {
		t.Errorf("Branch = %v, want [if_statement else_clause]", parsed.Branch)
	}
	if len(parsed.Loop) != 2 || parsed.Loop[0] != "for_statement" {
		t.Errorf("Loop = %v, want [for_statement while_statement]", parsed.Loop)
	}
	if len(parsed.Return) != 1 || parsed.Return[0] != "return_statement" {
		t.Errorf("Return = %v, want [return_statement]", parsed.Return)
	}
	if len(parsed.Error) != 2 || parsed.Error[0] != "try_statement" {
		t.Errorf("Error = %v, want [try_statement catch_clause]", parsed.Error)
	}
}

func TestControlFlowCategory_OmitEmpty(t *testing.T) {
	// LanguageNodes without control_flow key → zero-value ControlFlowCategory
	jsonStr := `{"functions":["fn"],"types":["tp"]}`

	var nodes LanguageNodes
	err := json.Unmarshal([]byte(jsonStr), &nodes)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(nodes.ControlFlow.Branch) != 0 {
		t.Errorf("ControlFlow.Branch = %v, want empty", nodes.ControlFlow.Branch)
	}
	if len(nodes.ControlFlow.Loop) != 0 {
		t.Errorf("ControlFlow.Loop = %v, want empty", nodes.ControlFlow.Loop)
	}
	if len(nodes.ControlFlow.Return) != 0 {
		t.Errorf("ControlFlow.Return = %v, want empty", nodes.ControlFlow.Return)
	}
	if len(nodes.ControlFlow.Error) != 0 {
		t.Errorf("ControlFlow.Error = %v, want empty", nodes.ControlFlow.Error)
	}

	// Marshal a ControlFlowCategory with only Branch → "loop","return","error" keys omitted
	partial := ControlFlowCategory{Branch: []string{"if_statement"}}
	data, err := json.Marshal(partial)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	result := string(data)
	if !strings.Contains(result, `"branch"`) {
		t.Errorf("partial marshal should contain branch key, got: %s", result)
	}
	if strings.Contains(result, `"loop"`) {
		t.Errorf("partial marshal should omit loop key, got: %s", result)
	}
	if strings.Contains(result, `"return"`) {
		t.Errorf("partial marshal should omit return key, got: %s", result)
	}
	if strings.Contains(result, `"error"`) {
		t.Errorf("partial marshal should omit error key, got: %s", result)
	}
}

func TestLoadModels(t *testing.T) {
	// models.json loads on init; verify data is present via lookups
	window, ok := GetModelContext("azure_ai/Meta-Llama-3-70B-Instruct")
	if !ok {
		t.Fatal("Expected azure_ai/Meta-Llama-3-70B-Instruct to be in models catalog")
	}
	if window != 8192 {
		t.Errorf("Expected context_window 8192 for model, got %d", window)
	}
}

func TestGetModelContext(t *testing.T) {
	tests := []struct {
		name          string
		contextWindow int
	}{
		{"azure_ai/Meta-Llama-3-70B-Instruct", 8192},
		{"deepseek-chat", 131072},
		{"gemini/gemma-3-27b-it", 131072},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			window, ok := GetModelContext(tt.name)
			if !ok {
				t.Fatalf("Expected model %s to exist", tt.name)
			}
			if window != tt.contextWindow {
				t.Errorf("Expected context_window %d for %s, got %d", tt.contextWindow, tt.name, window)
			}
		})
	}
}

func TestGetModelContextUnknown(t *testing.T) {
	_, ok := GetModelContext("nonexistent-model:999b")
	if ok {
		t.Error("Expected unknown model to return false")
	}
}

func TestGetAllModelNames(t *testing.T) {
	names := GetAllModelNames()

	if len(names) == 0 {
		t.Fatal("Expected some model names")
	}

	// Verify a few known models are present
	expectedModels := []string{"azure_ai/Meta-Llama-3-70B-Instruct", "deepseek-chat", "gemini/gemma-3-27b-it"}
	for _, model := range expectedModels {
		found := false
		for _, name := range names {
			if name == model {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected model %s to be in the list", model)
		}
	}
}

func TestLoadModelsFromBytes(t *testing.T) {
	customJSON := `{
		"models": {
			"test-model:7b": {"context_window": 4096},
			"test-model:70b": {"context_window": 32768}
		}
	}`

	err := LoadModelsFromBytes([]byte(customJSON))
	if err != nil {
		t.Fatalf("LoadModelsFromBytes failed: %v", err)
	}

	window, ok := GetModelContext("test-model:7b")
	if !ok {
		t.Fatal("Expected test-model:7b to be loaded")
	}
	if window != 4096 {
		t.Errorf("Expected context_window 4096, got %d", window)
	}

	window, ok = GetModelContext("test-model:70b")
	if !ok {
		t.Fatal("Expected test-model:70b to be loaded")
	}
	if window != 32768 {
		t.Errorf("Expected context_window 32768, got %d", window)
	}

	// Restore original data from embedded JSON
	if err := LoadModels(); err != nil {
		t.Fatalf("Failed to restore original models data: %v", err)
	}
}