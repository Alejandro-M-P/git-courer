package classifier

import (
	"strings"
	"unicode"
)

// Category represents the type of git operation detected
type Category int

const (
	Unknown Category = iota
	ReadOnly            // status, log, diff — zero Ollama
	SimpleWrite         // branch, checkout, stash — zero Ollama
	ComplexWrite        // commit, merge, rebase — needs Ollama
)

func (c Category) String() string {
	switch c {
	case ReadOnly:
		return "read_only"
	case SimpleWrite:
		return "simple_write"
	case ComplexWrite:
		return "complex_write"
	default:
		return "unknown"
	}
}

// Result is the classifier output
type Result struct {
	Category   Category
	Operation  string // detected operation, e.g. "commit", "status"
	Target     string // extracted argument, e.g. branch name
	Confidence float64 // 0.0–1.0
}

// NeedsOllama indicates if this category requires calling Ollama
func (r Result) NeedsOllama() bool {
	return r.Category == ComplexWrite || r.Category == Unknown
}

// --- Lexical signals ---

// Read verbs: words that indicate intent to view/query
var readVerbs = map[string]bool{
	"show": true, "muestra": true, "mostrar": true, "ver": true, "dame": true,
	"dime": true, "list": true, "lista": true, "check": true, "get": true,
	"display": true, "print": true, "what": true, "cual": true, "cuales": true,
}

// Read nouns: objects that only make sense in read operations
var readNouns = map[string]struct {
	op string
}{
	"status":    {op: "status"},
	"estado":    {op: "status"},
	"log":       {op: "log"},
	"history":   {op: "log"},
	"historial": {op: "log"},
	"commits":   {op: "log"},
	"diff":      {op: "diff"},
	"cambios":   {op: "diff"},
	"changes":   {op: "diff"},
	"blame":     {op: "blame"},
}

// Simple write operations: don't require generating content with AI
var simpleWriteOps = map[string]struct {
	op      string
	argNext bool // if true, next token is the argument
}{
	"branch":   {op: "branch", argNext: true},
	"rama":     {op: "branch", argNext: true},
	"checkout": {op: "checkout", argNext: true},
	"switch":   {op: "checkout", argNext: true},
	"cambiar":  {op: "checkout", argNext: true},
	"stash":    {op: "stash"},
	"guardar":  {op: "stash"},
	"fetch":    {op: "fetch"},
	"reset":    {op: "reset"},
	"revert":   {op: "revert", argNext: true},
	"restore":  {op: "restore"},
	"clean":    {op: "clean"},
	"tag":      {op: "tag", argNext: true},
}

// Creation verbs that combine with simpleWriteOps
var createVerbs = map[string]bool{
	"create": true, "crea": true, "crear": true, "new": true, "nueva": true, "nuevo": true,
	"make": true, "add": true, "init": true,
}

// Complex operations: need semantic context / generated message
var complexOps = map[string]struct {
	op string
}{
	"commit":    {op: "commit"},
	"merge":     {op: "merge"},
	"rebase":    {op: "rebase"},
	"push":      {op: "push"},
	"pull":      {op: "pull"},
	"fixup":     {op: "fixup"},
	"amend":     {op: "amend"},
	"squash":    {op: "squash"},
	"release":   {op: "release"},
	"publish":   {op: "push"},
	"subir":     {op: "push"},
	"publicar":  {op: "push"},
	"bajar":     {op: "pull"},
	"traer":     {op: "pull"},
	"fusionar":  {op: "merge"},
	"combinar":  {op: "merge"},
	"save":      {op: "commit"},
	"guardar":   {op: "commit"},
}

// Signals that elevate to ComplexWrite even if main op seems simple
var complexModifiers = map[string]bool{
	"and push":       true, "y push": true, "y subir": true,
	"and commit":     true, "y commit": true,
	"with message":   true, "con mensaje": true,
	"everything":     true, "todo": true, "all": true,
	"all changes":    true, "todos los cambios": true,
}

// --- Classifier ---

// Classify takes a natural language instruction and returns a Result
// without calling any external API. O(n) over input tokens.
func Classify(instruction string) Result {
	lower := strings.ToLower(strings.TrimSpace(instruction))

	// Fast check: if non-Latin script detected, fall back to Ollama
	if !isLatinScript(lower) {
		return Result{Category: Unknown, Confidence: 0.0}
	}

	// Quick check: does it contain complex write modifiers?
	for modifier := range complexModifiers {
		if strings.Contains(lower, modifier) {
			op := detectComplexOp(strings.Fields(lower))
			return Result{
				Category:   ComplexWrite,
				Operation:  op,
				Confidence: 0.85,
			}
		}
	}

	tokens := tokenize(lower)

	// Pre-compute: hasReadVerb and hasCreateVerb for all passes
	hasReadVerb := false
	for _, tok := range tokens {
		if readVerbs[tok] {
			hasReadVerb = true
			break
		}
	}

	hasCreateVerb := false
	for _, tok := range tokens {
		if createVerbs[tok] {
			hasCreateVerb = true
			break
		}
	}

	// Pass 1: search for complex operations (highest priority)
	for _, tok := range tokens {
		if entry, ok := complexOps[tok]; ok {
			return Result{
				Category:   ComplexWrite,
				Operation:  entry.op,
				Confidence: 0.95,
			}
		}
	}

	// Special case: read-only instruction without explicit noun
	// e.g. "¿en qué rama estoy?" → ReadOnly/branch
	if containsAny(lower, "branch", "rama", "ramas", "branches") && hasReadVerb {
		return Result{Category: ReadOnly, Operation: "branches", Confidence: 0.85}
	}
	if containsAny(lower, "remote", "remoto") && hasReadVerb {
		return Result{Category: ReadOnly, Operation: "remotes", Confidence: 0.80}
	}

	// Pass 2: simple write (MUST check before readNouns to avoid "stash changes" → read)
	for i, tok := range tokens {
		if entry, ok := simpleWriteOps[tok]; ok {
			target := ""
			if entry.argNext && i+1 < len(tokens) {
				target = tokens[i+1]
			}
			confidence := 0.80
			if hasCreateVerb {
				confidence = 0.90
			}
			return Result{
				Category:   SimpleWrite,
				Operation:  entry.op,
				Target:     target,
				Confidence: confidence,
			}
		}
	}

	// Pass 3: search for explicit read (verb + noun) - AFTER simpleWrite to not conflict
	for _, tok := range tokens {
		if entry, ok := readNouns[tok]; ok {
			confidence := 0.80
			if hasReadVerb {
				confidence = 0.95
			}
			return Result{
				Category:   ReadOnly,
				Operation:  entry.op,
				Confidence: confidence,
			}
		}
	}

	// No clear signal found → escalate to Ollama
	return Result{
		Category:   Unknown,
		Operation:  "",
		Confidence: 0.0,
	}
}

// --- helpers ---

func tokenize(s string) []string {
	replacer := strings.NewReplacer(",", " ", ".", " ", "!", " ", "?", " ", ":", " ", ";", " ", "'", " ", "\"", " ")
	cleaned := replacer.Replace(s)
	parts := strings.Fields(cleaned)
	return parts
}

func detectComplexOp(tokens []string) string {
	for _, tok := range tokens {
		if entry, ok := complexOps[tok]; ok {
			return entry.op
		}
	}
	return "commit" // reasonable fallback
}

func containsAny(s string, words ...string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

func isHexHash(s string) bool {
	if len(s) < 6 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// isLatinScript returns false if text contains non-Latin scripts
// In that case the classifier can't help → escalate to Ollama
func isLatinScript(s string) bool {
	for _, r := range s {
		if r > 127 {
			// Allow extended Latin characters (accents, ñ, etc.)
			if unicode.Is(unicode.Latin, r) {
				continue
			}
			// Any other script (Han, Cyrillic, Arabic, Hangul, etc.) → non-Latin
			return false
		}
	}
	return true
}
