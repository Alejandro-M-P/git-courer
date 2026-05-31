package chunkers

import (
	"testing"
)

func TestNewLanguageCatalog(t *testing.T) {
	catalog := NewLanguageCatalog()
	
	if catalog == nil {
		t.Fatal("Catalog should not be nil")
	}
	
	// Test that we can get language nodes for known languages
	if nodes, ok := catalog.ByName("Go"); !ok {
		t.Error("Should find Go language")
	} else if len(nodes.TestPatterns) == 0 {
		t.Error("Go should have test patterns")
	}
	
	if nodes, ok := catalog.ByName("Python"); !ok {
		t.Error("Should find Python language")
	} else if len(nodes.TestPatterns) == 0 {
		t.Error("Python should have test patterns")
	}
}

func TestFindTestPattern(t *testing.T) {
	catalog := NewLanguageCatalog()
	
	// Test Go test file detection
	pattern, lang := catalog.FindTestPattern("main_test.go")
	if pattern == nil {
		t.Error("Should find pattern for main_test.go")
	}
	if lang != "Go" {
		t.Errorf("Expected language Go, got %s", lang)
	}
	
	// Test Python test file detection  
	pattern, lang = catalog.FindTestPattern("test_main.py")
	if pattern == nil {
		t.Error("Should find pattern for test_main.py")
	}
	if lang != "Python" {
		t.Errorf("Expected language Python, got %s", lang)
	}
}

func TestIsTestFile(t *testing.T) {
	catalog := NewLanguageCatalog()
	
	if !catalog.IsTestFile("main_test.go") {
		t.Error("main_test.go should be detected as test file")
	}
	
	if !catalog.IsTestFile("test_main.py") {
		t.Error("test_main.py should be detected as test file")
	}
	
	if catalog.IsTestFile("main.go") {
		t.Error("main.go should NOT be detected as test file")
	}
}