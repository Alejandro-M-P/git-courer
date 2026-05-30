package chunkers

import (
	"log"
	"testing"
)

func TestKreuzbergParseSmoke(t *testing.T) {
	// kreuzberg uses lowercase language names
	lang := "go"

	src := []byte("package main\nfunc Hello() {}")

	result, err := AnalyzeSource(lang, src)
	if err != nil {
		// Grammar may not be downloaded; log and skip
		t.Skipf("AnalyzeSource(%q) failed (grammar may not be cached): %v", lang, err)
	}
	if result == nil {
		t.Fatal("AnalyzeSource returned nil")
	}
	if result.Language == "" {
		t.Error("Language field is empty, expected a detected language name")
	}
	_ = log.Default
}
