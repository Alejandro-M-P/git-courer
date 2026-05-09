package chunkers

import (
	"os"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/data"
	tsgrammars "github.com/odvcencio/gotreesitter/grammars"
)

func TestMain(m *testing.M) {
	if err := data.LoadLanguagesFromBytes([]byte(domain.FixtureJSON)); err != nil {
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"foo.go", "go"},
		{"foo.py", "python"},
		{"foo.js", "javascript"},
		{"foo.ts", "typescript"},
		{"foo.rs", "rust"},
		{"foo.java", "java"},
		{"foo.cs", "c_sharp"},
		{"foo.cpp", "cpp"},
		{"foo.php", "php"},
		{"foo.rb", "ruby"},
		{"foo.swift", "swift"},
		{"foo.kt", "kotlin"},
		{"foo.dart", "dart"},
		{"unknown.xyz", ""},
	}
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := detectLanguage(tt.filename)
			if got != tt.want {
				t.Errorf("detectLanguage(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestParseAndExtract_Go(t *testing.T) {
	src := []byte(`package main

func Hello(name string) string {
	return "Hello, " + name
}

func Add(a, b int) int {
	return a + b
}

type Config struct {
	Port int
	Host string
}
`)
	lang := tsgrammars.GoLanguage()
	nodes, _ := domain.GetLanguageNodes("Go")

	ents, err := parseAndExtract(lang, src, nodes)
	if err != nil {
		t.Fatalf("parseAndExtract: %v", err)
	}

	names := entityNames(ents)
	expected := map[string]bool{"Hello": true, "Add": true, "Config": true}
	for _, name := range names {
		if !expected[name] {
			t.Errorf("unexpected entity %q", name)
		}
	}
	for exp := range expected {
		if !contains(names, exp) {
			t.Errorf("missing entity %q in %v", exp, names)
		}
	}

	if ents[0].Line == 0 {
		t.Errorf("line should be >= 1, got %d", ents[0].Line)
	}
}

func TestParseAndExtract_Python(t *testing.T) {
	src := []byte(`def hello(name):
    return "Hello, " + name

def calculate(a, b):
    return a + b

class Person:
    def __init__(self, name):
        self.name = name
`)
	lang := tsgrammars.PythonLanguage()
	nodes, _ := domain.GetLanguageNodes("Python")

	ents, err := parseAndExtract(lang, src, nodes)
	if err != nil {
		t.Fatalf("parseAndExtract: %v", err)
	}

	// hello, calculate, __init__ (3 functions) + Person (1 class) = 4
	if len(ents) < 3 {
		t.Errorf("expected at least 3 entities, got %d: %v", len(ents), entityNames(ents))
	}

	hasFunc := false
	hasClass := false
	for _, e := range ents {
		if e.Kind == "func" {
			hasFunc = true
		}
		if e.Kind == "type" {
			hasClass = true
		}
	}
	if !hasFunc {
		t.Error("no function entities extracted")
	}
	if !hasClass {
		t.Error("no type entities extracted")
	}
}

func TestParseAndExtract_Empty(t *testing.T) {
	lang := tsgrammars.GoLanguage()
	nodes, _ := domain.GetLanguageNodes("Go")

	ents, err := parseAndExtract(lang, []byte{}, nodes)
	if err != nil {
		t.Fatalf("empty source should not error: %v", err)
	}
	if len(ents) != 0 {
		t.Errorf("expected 0 entities for empty source, got %d", len(ents))
	}
}

func TestParseAndExtract_InvalidSource(t *testing.T) {
	lang := tsgrammars.GoLanguage()
	nodes, _ := domain.GetLanguageNodes("Go")

	// Tree-sitter handles some invalid syntax gracefully (error recovery).
	// We just verify it doesn't panic.
	ents, err := parseAndExtract(lang, []byte("not valid go code at all"), nodes)
	if err != nil {
		// This is acceptable — tree-sitter may fail or may recover.
		t.Logf("parse error (expected, soft failure): %v", err)
	}
	// If it recovered and returned entities, that's also fine.
	t.Logf("entities from invalid source: %v", entityNames(ents))
}

func TestMatchEntities_NewFunc(t *testing.T) {
	nodes, _ := domain.GetLanguageNodes("Go")
	before := []entity{}
	after := []entity{
		{Name: "NewHandler", Signature: "func NewHandler() {}", Line: 10, Kind: "func"},
	}

	labels := matchEntities(before, after, nodes, "Go", "handler.go")
	if len(labels) != 1 {
		t.Fatalf("expected 1 label, got %d", len(labels))
	}
	if labels[0].Type != domain.NEW_FUNC {
		t.Errorf("expected NEW_FUNC, got %s", labels[0].Type)
	}
	if labels[0].Name != "NewHandler" {
		t.Errorf("expected NewHandler, got %s", labels[0].Name)
	}
}

func TestMatchEntities_ModBody(t *testing.T) {
	nodes, _ := domain.GetLanguageNodes("Go")
	sig := "func Process(x int) error { return nil }"
	before := []entity{
		{Name: "Process", Signature: sig, Line: 5, Kind: "func"},
	}
	after := []entity{
		{Name: "Process", Signature: sig, Line: 8, Kind: "func"},
	}

	labels := matchEntities(before, after, nodes, "Go", "service.go")
	if len(labels) != 1 {
		t.Fatalf("expected 1 label, got %d", len(labels))
	}
	if labels[0].Type != domain.MOD_BODY {
		t.Errorf("expected MOD_BODY, got %s", labels[0].Type)
	}
}

func TestMatchEntities_ModSig(t *testing.T) {
	nodes, _ := domain.GetLanguageNodes("Go")
	before := []entity{
		{Name: "Validate", Signature: "func Validate(x int) {}", Line: 5, Kind: "func"},
	}
	after := []entity{
		{Name: "Validate", Signature: "func Validate(x, y int) {}", Line: 5, Kind: "func"},
	}

	labels := matchEntities(before, after, nodes, "Go", "validate.go")
	if len(labels) != 1 {
		t.Fatalf("expected 1 label, got %d", len(labels))
	}
	if labels[0].Type != domain.MOD_SIG {
		t.Errorf("expected MOD_SIG, got %s", labels[0].Type)
	}
}

func TestMatchEntities_DeletedFunc(t *testing.T) {
	nodes, _ := domain.GetLanguageNodes("Go")
	before := []entity{
		{Name: "OldHelper", Signature: "func OldHelper() {}", Line: 3, Kind: "func"},
	}
	after := []entity{}

	labels := matchEntities(before, after, nodes, "Go", "helper.go")
	if len(labels) != 1 {
		t.Fatalf("expected 1 label, got %d", len(labels))
	}
	if labels[0].Type != domain.DELETED_FUNC {
		t.Errorf("expected DELETED_FUNC, got %s", labels[0].Type)
	}
}

func TestMatchEntities_NewType(t *testing.T) {
	nodes, _ := domain.GetLanguageNodes("Go")
	before := []entity{}
	after := []entity{
		{Name: "Config", Signature: "type Config struct { Port int }", Line: 10, Kind: "type"},
	}

	labels := matchEntities(before, after, nodes, "Go", "config.go")
	if len(labels) != 1 {
		t.Fatalf("expected 1 label, got %d", len(labels))
	}
	if labels[0].Type != domain.NEW_TYPE {
		t.Errorf("expected NEW_TYPE, got %s", labels[0].Type)
	}
}

func TestMatchEntities_ModType(t *testing.T) {
	nodes, _ := domain.GetLanguageNodes("Go")
	before := []entity{
		{Name: "User", Signature: "type User struct { Name string }", Line: 4, Kind: "type"},
	}
	after := []entity{
		{Name: "User", Signature: "type User struct { Name string; Email string }", Line: 4, Kind: "type"},
	}

	labels := matchEntities(before, after, nodes, "Go", "user.go")
	if len(labels) != 1 {
		t.Fatalf("expected 1 label, got %d", len(labels))
	}
	if labels[0].Type != domain.MOD_TYPE {
		t.Errorf("expected MOD_TYPE, got %s", labels[0].Type)
	}
}

func TestMatchEntities_DeletedType(t *testing.T) {
	nodes, _ := domain.GetLanguageNodes("Go")
	before := []entity{
		{Name: "LegacyItem", Signature: "type LegacyItem struct{}", Line: 2, Kind: "type"},
	}
	after := []entity{}

	labels := matchEntities(before, after, nodes, "Go", "legacy.go")
	if len(labels) != 1 {
		t.Fatalf("expected 1 label, got %d", len(labels))
	}
	if labels[0].Type != domain.DELETED_TYPE {
		t.Errorf("expected DELETED_TYPE, got %s", labels[0].Type)
	}
}

func TestMatchEntities_Mixed(t *testing.T) {
	nodes, _ := domain.GetLanguageNodes("Go")
	before := []entity{
		{Name: "Process", Signature: "func Process() {}", Line: 1, Kind: "func"},
		{Name: "OldFunc", Signature: "func OldFunc() {}", Line: 10, Kind: "func"},
		{Name: "OldType", Signature: "type OldType struct{}", Line: 15, Kind: "type"},
	}
	after := []entity{
		{Name: "Process", Signature: "func Process(x int) {}", Line: 1, Kind: "func"},
		{Name: "NewFunc", Signature: "func NewFunc() {}", Line: 20, Kind: "func"},
	}

	labels := matchEntities(before, after, nodes, "Go", "mixed.go")
	if len(labels) != 4 {
		t.Fatalf("expected 4 labels, got %d", len(labels))
	}

	types := labelTypes(labels)
	if !types[domain.MOD_SIG] {
		t.Error("missing MOD_SIG for Process")
	}
	if !types[domain.NEW_FUNC] {
		t.Error("missing NEW_FUNC for NewFunc")
	}
	if !types[domain.DELETED_FUNC] {
		t.Error("missing DELETED_FUNC for OldFunc")
	}
	if !types[domain.DELETED_TYPE] {
		t.Error("missing DELETED_TYPE for OldType")
	}
}

func TestIsPublicEntity_Go(t *testing.T) {
	tests := []struct {
		name     string
		sig      string
		want     bool
	}{
		{"HandleRequest", "func HandleRequest() {}", true},
		{"internalHelper", "func internalHelper() {}", false},
		{"_private", "func _private() {}", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ent := entity{Name: tt.name, Signature: tt.sig, Kind: "func"}
			got := isPublicEntity(ent, "Go")
			if got != tt.want {
				t.Errorf("isPublicEntity(%q, Go) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestIsPublicEntity_Python(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"process_data", true},
		{"_internal_helper", false},
		{"__dunder__", false}, // starts with underscore
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ent := entity{Name: tt.name, Signature: "def " + tt.name + "():"}
			got := isPublicEntity(ent, "Python")
			if got != tt.want {
				t.Errorf("isPublicEntity(%q, Python) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestIsPublicEntity_Rust(t *testing.T) {
	entPub := entity{Name: "public_fn", Signature: "pub fn public_fn() {}", Kind: "func"}
	entPriv := entity{Name: "private_fn", Signature: "fn private_fn() {}", Kind: "func"}

	if !isPublicEntity(entPub, "Rust") {
		t.Error("pub fn should be public")
	}
	if isPublicEntity(entPriv, "Rust") {
		t.Error("non-pub fn should not be public")
	}
}

func TestIsPublicEntity_Java(t *testing.T) {
	entPub := entity{Name: "doSomething", Signature: "public void doSomething() {}", Kind: "func"}
	entPriv := entity{Name: "helper", Signature: "private void helper() {}", Kind: "func"}

	if !isPublicEntity(entPub, "Java") {
		t.Error("public method should be public")
	}
	if isPublicEntity(entPriv, "Java") {
		t.Error("private method should not be public")
	}
}

func TestBreakingDetection(t *testing.T) {
	nodes, _ := domain.GetLanguageNodes("Go")
	before := []entity{
		{Name: "HandleRequest", Signature: "func HandleRequest(x int) {}", Line: 5, Kind: "func"},
		{Name: "internalHelper", Signature: "func internalHelper(x int) {}", Line: 10, Kind: "func"},
	}
	after := []entity{
		{Name: "HandleRequest", Signature: "func HandleRequest(x, y int) {}", Line: 5, Kind: "func"},
		{Name: "internalHelper", Signature: "func internalHelper(x, y int) {}", Line: 10, Kind: "func"},
	}

	labels := matchEntities(before, after, nodes, "Go", "breaking.go")

	for _, l := range labels {
		switch l.Name {
		case "HandleRequest":
			if !l.Breaking {
				t.Errorf("HandleRequest MOD_SIG should be breaking")
			}
		case "internalHelper":
			if l.Breaking {
				t.Errorf("internalHelper MOD_SIG should NOT be breaking (private)")
			}
		}
	}
}

func TestBreakingDetection_Deleted(t *testing.T) {
	nodes, _ := domain.GetLanguageNodes("Go")
	before := []entity{
		{Name: "HandleRequest", Signature: "func HandleRequest() {}", Line: 5, Kind: "func"},
		{Name: "internalHelper", Signature: "func internalHelper() {}", Line: 10, Kind: "func"},
	}
	after := []entity{}

	labels := matchEntities(before, after, nodes, "Go", "deleted.go")

	for _, l := range labels {
		switch l.Name {
		case "HandleRequest":
			if !l.Breaking {
				t.Errorf("HandleRequest DELETED should be breaking (public)")
			}
		case "internalHelper":
			if l.Breaking {
				t.Errorf("internalHelper DELETED should NOT be breaking (private)")
			}
		}
	}
}

func TestFormatLabels_Single(t *testing.T) {
	labels := []domain.Label{
		{Type: domain.NEW_FUNC, Name: "NewFunc", File: "file.go", Line: 10, Breaking: false},
		{Type: domain.MOD_BODY, Name: "Changed", File: "file.go", Line: 5, Breaking: false},
	}

	formatted := formatLabels("file.go", labels)

	if !containsStr(formatted, "📄 file.go") {
		t.Error("missing 📄 header")
	}
	if !containsStr(formatted, "NewFunc [NEW_FUNC] file.go:10") {
		t.Error("missing NewFunc annotation")
	}
	if !containsStr(formatted, "Changed [MOD_BODY] file.go:5") {
		t.Error("missing Changed annotation")
	}
}

func TestFormatLabels_Breaking(t *testing.T) {
	labels := []domain.Label{
		{Type: domain.MOD_SIG, Name: "Handle", File: "api.go", Line: 10, Breaking: true},
	}

	formatted := formatLabels("api.go", labels)

	if !containsStr(formatted, "⚠ BREAKING") {
		t.Errorf("missing breaking indicator in: %q", formatted)
	}
}

func TestFormatLabels_Empty(t *testing.T) {
	formatted := formatLabels("empty.go", nil)
	if !containsStr(formatted, "📄 empty.go") {
		t.Error("empty labels should still produce header")
	}
	// Should have header and a trailing newline, but no label lines.
	if containsStr(formatted, "[") {
		t.Error("should not contain label markers")
	}
}

func TestCategoryLabel(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"config.json", "CONFIG"},
		{"settings.yaml", "CONFIG"},
		{"app.toml", "CONFIG"},
		{"README.md", "DOCS"},
		{"docs/api.rst", "DOCS"},
		{"go.mod", "DEPS"},
		{"package.json", "DEPS"},
		{"Cargo.toml", "DEPS"},
		{"Cargo.lock", "DEPS"},
		{"Makefile", "CI"},
		{"Dockerfile", "CI"},
		{".github/workflows/ci.yml", "CI"},
		{".gitlab-ci.yml", "CI"},
		{"src/main.go", ""}, // not a non-code file
	}
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := categoryLabel(tt.filename)
			if got != tt.want {
				t.Errorf("categoryLabel(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestAnnotate_FullPipeline_Go(t *testing.T) {
	a := NewASTAnnotator()

	before := []byte(`package main

func Process(x int) error {
	return nil
}

func OldHelper() {}
`)
	after := []byte(`package main

func Process(x int) error {
	return fmt.Errorf("updated")
}

func NewFeature() {}
`)
	chunk := &domain.DiffChunk{
		Files: []string{"service.go"},
	}

	err := a.Annotate(chunk, before, after)
	if err != nil {
		t.Fatalf("Annotate: %v", err)
	}
	if chunk.AnnotatedDiff == "" {
		t.Fatal("AnnotatedDiff should not be empty")
	}

	t.Logf("AnnotatedDiff:\n%s", chunk.AnnotatedDiff)

	// Process should be MOD_BODY (same sig, different body).
	if !containsStr(chunk.AnnotatedDiff, "Process [MOD_BODY]") {
		t.Errorf("expected MOD_BODY for Process, got:\n%s", chunk.AnnotatedDiff)
	}
	// NewFeature should be NEW_FUNC.
	if !containsStr(chunk.AnnotatedDiff, "NewFeature [NEW_FUNC]") {
		t.Error("missing NEW_FUNC for NewFeature")
	}
	// OldHelper should be DELETED_FUNC.
	if !containsStr(chunk.AnnotatedDiff, "OldHelper [DELETED_FUNC") {
		t.Error("missing DELETED_FUNC for OldHelper")
	}
}

func TestAnnotate_NewFile(t *testing.T) {
	a := NewASTAnnotator()
	chunk := &domain.DiffChunk{
		Files: []string{"new.go"},
	}
	after := []byte(`package newpkg
func NewFunc() string { return "hello" }
`)

	err := a.Annotate(chunk, nil, after)
	if err != nil {
		t.Fatalf("Annotate new file: %v", err)
	}
	if !containsStr(chunk.AnnotatedDiff, "NewFunc [NEW_FUNC]") {
		t.Error("new file: should have NEW_FUNC label")
	}
}

func TestAnnotate_DeletedFile(t *testing.T) {
	a := NewASTAnnotator()
	chunk := &domain.DiffChunk{
		Files: []string{"old.go"},
	}
	before := []byte(`package oldpkg
func OldFunc() string { return "bye" }
`)

	err := a.Annotate(chunk, before, nil)
	if err != nil {
		t.Fatalf("Annotate deleted file: %v", err)
	}
	t.Logf("AnnotatedDiff=%q", chunk.AnnotatedDiff)
	if !containsStr(chunk.AnnotatedDiff, "OldFunc [DELETED_FUNC") {
		t.Error("deleted file: should have DELETED_FUNC label")
	}
}

func TestAnnotate_UnknownLanguage(t *testing.T) {
	a := NewASTAnnotator()
	chunk := &domain.DiffChunk{
		Files: []string{"script.unknown"},
	}
	before := []byte("some content")
	after := []byte("some different content")

	err := a.Annotate(chunk, before, after)
	if err != nil {
		t.Fatalf("Annotate unknown language: %v", err)
	}
	// Should not crash and AnnotatedDiff should remain empty.
	if chunk.AnnotatedDiff != "" {
		t.Errorf("unknown language should produce empty AnnotatedDiff, got %q", chunk.AnnotatedDiff)
	}
}

func TestAnnotate_ConfigFile(t *testing.T) {
	a := NewASTAnnotator()
	chunk := &domain.DiffChunk{
		Files: []string{"config.json"},
	}
	before := []byte(`{"port": 8080}`)
	after := []byte(`{"port": 9090}`)

	err := a.Annotate(chunk, before, after)
	if err != nil {
		t.Fatalf("Annotate config file: %v", err)
	}
	if !containsStr(chunk.AnnotatedDiff, "[CONFIG]") {
		t.Errorf("config file should have CONFIG label, got %q", chunk.AnnotatedDiff)
	}
}

func TestAnnotate_DepsFile(t *testing.T) {
	a := NewASTAnnotator()
	chunk := &domain.DiffChunk{
		Files: []string{"go.mod"},
	}
	before := []byte("module foo")
	after := []byte("module bar")

	err := a.Annotate(chunk, before, after)
	if err != nil {
		t.Fatalf("Annotate deps file: %v", err)
	}
	if !containsStr(chunk.AnnotatedDiff, "[DEPS]") {
		t.Errorf("deps file should have DEPS label, got %q", chunk.AnnotatedDiff)
	}
}

func TestAnnotate_EmptyContent(t *testing.T) {
	a := NewASTAnnotator()
	chunk := &domain.DiffChunk{
		Files: []string{"empty.go"},
	}

	err := a.Annotate(chunk, nil, nil)
	if err != nil {
		t.Fatalf("Annotate empty: %v", err)
	}
	// Empty before and after: no AST annotations possible, AnnotatedDiff stays empty.
	if chunk.AnnotatedDiff != "" {
		t.Errorf("empty content should produce empty AnnotatedDiff, got %q", chunk.AnnotatedDiff)
	}
}

func TestAnnotate_AppendsToExistingDiff(t *testing.T) {
	a := NewASTAnnotator()

	chunk := &domain.DiffChunk{
		Files:      []string{"first.go"},
		AnnotatedDiff: "📄 existing.md\nlegacy [DOCS] existing.md\n",
	}
	before := []byte("package first\nfunc First() {}")
	after := []byte("package first\nfunc First() {}\nfunc Second() {}")

	err := a.Annotate(chunk, before, after)
	if err != nil {
		t.Fatalf("Annotate append: %v", err)
	}

	if !containsStr(chunk.AnnotatedDiff, "📄 existing.md") {
		t.Error("should preserve existing AnnotatedDiff content")
	}
	if !containsStr(chunk.AnnotatedDiff, "📄 first.go") {
		t.Error("should append new annotation")
	}
	if !containsStr(chunk.AnnotatedDiff, "Second [NEW_FUNC]") {
		t.Error("should have NEW_FUNC for Second")
	}
}

func TestAnnotate_NoFiles(t *testing.T) {
	a := NewASTAnnotator()
	chunk := &domain.DiffChunk{
		Files: nil,
	}
	before := []byte("package main\nfunc F() {}")
	after := []byte("package main\nfunc F() {}")

	err := a.Annotate(chunk, before, after)
	if err != nil {
		t.Fatalf("Annotate no files: %v", err)
	}
	if chunk.AnnotatedDiff != "" {
		t.Errorf("no files should produce empty AnnotatedDiff, got %q", chunk.AnnotatedDiff)
	}
}

func TestAnnotate_ParseError(t *testing.T) {
	a := NewASTAnnotator()
	chunk := &domain.DiffChunk{
		Files: []string{"broken.go"},
	}
	before := []byte("func Broken( { missing paren") // syntax error
	after := []byte("func Broken( { fixed")          // still broken

	err := a.Annotate(chunk, before, after)
	if err != nil {
		t.Fatalf("parse error should not fail Annotate: %v", err)
	}
	// Soft failure: AnnotatedDiff stays empty but no error returned.
	if chunk.AnnotatedDiff != "" {
		t.Logf("parse error recovery produced: %q (acceptable)", chunk.AnnotatedDiff)
	}
}

func TestGrammarsNameToDomain_Completeness(t *testing.T) {
	// Verify all languages in langFactories have a domain mapping.
	for gName := range langFactories {
		if _, ok := grammarsNameToDomain[gName]; !ok {
			t.Errorf("langFactories has %q but grammarsNameToDomain is missing it", gName)
		}
		domainName := grammarsNameToDomain[gName]
		if _, ok := domain.GetLanguageNodes(domainName); !ok {
			t.Errorf("language %q → domain %q not in LanguageNodes", gName, domainName)
		}
	}
}

func TestNewASTAnnotator_ImplementsPort(t *testing.T) {
	a := NewASTAnnotator()
	// Compile-time check: already done by var _ declaration.
	// Runtime: verify it's not nil.
	if a == nil {
		t.Fatal("NewASTAnnotator returned nil")
	}
	chunk := &domain.DiffChunk{Files: []string{"test.go"}}
	if err := a.Annotate(chunk, nil, nil); err != nil {
		t.Errorf("nil content should not error: %v", err)
	}
}

func TestAnnotate_SameSignature_DifferentBody(t *testing.T) {
	a := NewASTAnnotator()

	before := []byte(`package main
func Calculate(x int) int {
	return x * 2
}
`)
	after := []byte(`package main
func Calculate(x int) int {
	return x * 3
}
`)
	chunk := &domain.DiffChunk{Files: []string{"calc.go"}}

	err := a.Annotate(chunk, before, after)
	if err != nil {
		t.Fatalf("Annotate: %v", err)
	}

	if !containsStr(chunk.AnnotatedDiff, "Calculate [MOD_BODY]") {
		t.Errorf("same sig, diff body should be MOD_BODY, got:\n%s", chunk.AnnotatedDiff)
	}
}

// --- helpers ---

func entityNames(ents []entity) []string {
	names := make([]string, len(ents))
	for i, e := range ents {
		names[i] = e.Name
	}
	return names
}

func contains(names []string, target string) bool {
	for _, n := range names {
		if n == target {
			return true
		}
	}
	return false
}

func labelTypes(labels []domain.Label) map[domain.LabelType]bool {
	m := make(map[domain.LabelType]bool)
	for _, l := range labels {
		m[l.Type] = true
	}
	return m
}

func containsStr(haystack, needle string) bool {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
