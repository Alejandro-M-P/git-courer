package chunkers

import (
	"testing"

	gotreesitter "github.com/odvcencio/gotreesitter"
	tsgrammars "github.com/odvcencio/gotreesitter/grammars"
)

func TestGotreesitterImports(t *testing.T) {
	lang := tsgrammars.GoLanguage()
	if lang == nil {
		t.Fatal("GoLanguage() returned nil")
	}
	parser := gotreesitter.NewParser(lang)
	src := []byte("package main\nfunc Hello() {}")
	tree, err := parser.Parse(src)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	defer tree.Release()
	root := tree.RootNode()
	if root == nil {
		t.Fatal("RootNode() returned nil")
	}
	t.Logf("Root type: %s", root.Type(lang))
}
