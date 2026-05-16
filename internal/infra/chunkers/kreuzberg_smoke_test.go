package chunkers

import (
	"testing"
)

func TestKreuzbergParseSmoke(t *testing.T) {
	lang := "Go"
	src := []byte("package main\nfunc Hello() {}")
	
	tree, err := parseSource(lang, src)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer tree.Release()
	
	root := tree.RootNode()
	if root == nil {
		t.Fatal("RootNode() returned nil")
	}
	
	if root.Type() != "source_file" {
		t.Errorf("Root type = %s, want source_file", root.Type())
	}
}
