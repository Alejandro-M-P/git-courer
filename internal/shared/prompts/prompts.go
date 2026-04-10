// Package prompts provides LLM prompt templates for git-courer operations.
// Each operation has its own focused prompt — no generic one-size-fits-all.
// Templates are loaded from .txt files in the txt/ directory.
package prompts

import (
	"bytes"
	"embed"
	"fmt"
	"path"
	"strings"
	"text/template"
)

//go:embed txt/*.txt
var templatesFS embed.FS

var templateCache = make(map[string]string)

// Load all templates into memory on first use
func init() {
	loadTemplates()
}

func loadTemplates() {
	entries, err := templatesFS.ReadDir("txt")
	if err != nil {
		return
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// strip .txt extension for the key
		key := name[:len(name)-4]

		data, err := templatesFS.ReadFile(path.Join("txt", name))
		if err != nil {
			continue
		}
		templateCache[key] = string(data)
	}
}

// Get returns the prompt template for the given operation.
// Falls back to a minimal generic prompt for unknown operations.
func Get(op string) string {
	if tmpl, ok := templateCache[op]; ok {
		return tmpl
	}
	return `Interpret this git instruction: "{{.Instruction}}"
Respond with ONLY a JSON object with the relevant arguments.`
}

// GetAll returns all available templates (for debugging/listing)
func GetAll() map[string]string {
	return templateCache
}

// HasTemplate checks if a template exists for the given operation
func HasTemplate(op string) bool {
	_, ok := templateCache[op]
	return ok
}

// Render processes a template with the given data
func Render(tmpl string, data interface{}) (string, error) {
	t, err := template.New("prompt").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderOp renders a template for an operation with the provided params
func RenderOp(op string, params interface{}) (string, error) {
	tmpl := Get(op)
	return Render(tmpl, params)
}

// --- Legacy compatibility functions ---

// TemplateFor returns the prompt template for the given operation.
// Kept for backward compatibility.
func TemplateFor(op string) string {
	return Get(op)
}

// Render renders a template string with the given data.
// Kept for backward compatibility.
func RenderWithData(tmpl string, data interface{}) (string, error) {
	return Render(tmpl, data)
}

// GetCommitMessage returns the commit message template
func GetCommitMessage() string {
	return Get("commit_message")
}

// GetDecideCommit returns the decide commit template
func GetDecideCommit() string {
	return Get("decide_commit")
}

// --- Generic interpreter template ---

// InterpretGitOp is a generic template that wraps operation-specific prompts.
// It takes the operation name and delegates to the specific template.
const InterpretGitOp = `You are a Git expert. Your task is to interpret a natural language instruction and extract the required parameters.

**User instruction:** "{{.Instruction}}"

**Operation:** {{.Operation}}

**Context:**
{{.Context}}

**Specific guidance for {{.Operation}}:**
{{.OperationPrompt}}

Now extract the parameters and respond with ONLY a JSON object.`

// InterpretParams holds parameters for the generic interpreter
type InterpretParams struct {
	Operation       string
	Instruction     string
	Context         string
	OperationPrompt string
}

// BuildInterpretParams creates InterpretParams for a given operation
func BuildInterpretParams(op, instruction string, ctx map[string]string) InterpretParams {
	// Build context string from context map
	var ctxStr strings.Builder
	for k, v := range ctx {
		ctxStr.WriteString(fmt.Sprintf("- %s: %s\n", k, v))
	}

	return InterpretParams{
		Operation:       op,
		Instruction:     instruction,
		Context:         ctxStr.String(),
		OperationPrompt: Get(op), // Get the operation-specific prompt
	}
}

// --- New struct-based params ---

// MessageParams for commit message generation
type MessageParams struct {
	CurrentBranch   string
	Files           string
	Diff            string
	RejectedMessage string
}

// DecideParams for deciding what to commit
type DecideParams struct {
	Instruction   string
	CurrentBranch string
	GitStatus     string
	Untracked     string
	Modified      string
	Deleted       string
}

// OpParams for any per-operation prompt
type OpParams struct {
	Instruction    string
	CurrentBranch  string
	Branches       string
	Tags           string
	RecentCommits  string
	UntrackedFiles string
	Remotes        string
	Remote         string
}

// BuildMessageParams creates MessageParams for commit message
func BuildMessageParams(files []string, diff string) MessageParams {
	return MessageParams{Files: joinFiles(files), Diff: diff}
}

// BuildMessageParamsWithRetry creates MessageParams with rejection context
func BuildMessageParamsWithRetry(files []string, diff, rejected string) MessageParams {
	return MessageParams{Files: joinFiles(files), Diff: diff, RejectedMessage: rejected}
}

// BuildDecideParams creates DecideParams
func BuildDecideParams(instruction, gitStatus, untracked, modified, deleted string) DecideParams {
	return DecideParams{
		Instruction: instruction, GitStatus: gitStatus,
		Untracked: untracked, Modified: modified, Deleted: deleted,
	}
}

// BuildOpParams constructs OpParams from a context map
func BuildOpParams(instruction string, ctx map[string]string) OpParams {
	return OpParams{
		Instruction:    instruction,
		CurrentBranch:  ctx["current_branch"],
		Branches:       ctx["branches"],
		Tags:           ctx["tags"],
		RecentCommits:  ctx["recent_commits"],
		UntrackedFiles: ctx["untracked_files"],
		Remotes:        ctx["remotes"],
		Remote:         ctx["remote"],
	}
}

func joinFiles(files []string) string {
	if len(files) == 0 {
		return ""
	}
	result := files[0]
	for _, f := range files[1:] {
		result += ", " + f
	}
	return result
}

// --- Embedded prompt constants (for when embed fails) ---

const FallbackGenerateMessage = `You are an expert Git assistant. Generate a commit message.

Files: {{.Files}}

Diff:
{{.Diff}}

{{- if .RejectedMessage}}
Previous rejected: {{.RejectedMessage}}
{{- end}}

Rules:
- Conventional: type(scope): description
- Max 72 chars
- Be specific`

const FallbackDecideCommit = `Decide what to commit from: "{{.Instruction}}"

Untracked: {{.Untracked}}
Modified: {{.Modified}}
Deleted: {{.Deleted}}

JSON: {"include_untracked": bool, "filter": ""}`

const FallbackBranchCreate = `Create branch name from: "{{.Instruction}}"

Current: {{.CurrentBranch}}
Branches: {{.Branches}}

Format: feat/, fix/, refactor/, docs/, test/, chore/
kebab-case, short, descriptive`

// GetWithFallback returns the template, falling back to embedded if file loading fails
func GetWithFallback(op string) string {
	if tmpl, ok := templateCache[op]; ok && tmpl != "" {
		return tmpl
	}
	switch op {
	case "commit_message":
		return FallbackGenerateMessage
	case "decide_commit":
		return FallbackDecideCommit
	case "branch_create":
		return FallbackBranchCreate
	default:
		return `{"instruction": "{{.Instruction}}"}`
	}
}
