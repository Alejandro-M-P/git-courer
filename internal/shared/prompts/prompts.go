// Package prompts provides LLM prompt templates for git-courer operations.
// Each operation has its own focused prompt — no generic one-size-fits-all.
// Templates are loaded from .md files in the md/ directory.
package prompts

import (
	"bytes"
	"embed"
	"fmt"
	"path"
	"strings"
	"text/template"
)

//go:embed md/*.md
var templatesFS embed.FS

var templateCache = make(map[string]string)

// Load all templates into memory on first use
func init() {
	loadTemplates()
}

func loadTemplates() {
	entries, err := templatesFS.ReadDir("md")
	if err != nil {
		return
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		// strip extension for the key (e.g., "commit_message.md" -> "commit_message")
		key := strings.TrimSuffix(name, ".md")

		data, err := templatesFS.ReadFile(path.Join("md", name))
		if err != nil {
			continue
		}
		templateCache[key] = string(data)
	}
}

// Get returns the prompt template for the given operation.
// Returns an error if the operation template is not found.
func Get(op string) (string, error) {
	if tmpl, ok := templateCache[op]; ok {
		return tmpl, nil
	}
	return "", fmt.Errorf("prompt template for operation '%s' not found", op)
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

// GetCommitMessage returns the commit message template
func GetCommitMessage() string {
	tmpl, _ := Get("commit_message")
	return tmpl
}

// GetCommitSynthesis returns the commit synthesis template
func GetCommitSynthesis() string {
	tmpl, _ := Get("commit_synthesis")
	return tmpl
}

// GetCredentialAudit returns the credential_audit template
func GetCredentialAudit() string {
	tmpl, _ := Get("credential_audit")
	return tmpl
}

// GetChangelogGenerate returns the changelog_generate template
func GetChangelogGenerate() string {
	tmpl, _ := Get("changelog_generate")
	return tmpl
}

// GetClassifyBinary returns the classify_binary template
func GetClassifyBinary() string {
	tmpl, _ := Get("classify_binary")
	return tmpl
}

// GetBranchCreate returns the branch_create template
func GetBranchCreate() string {
	tmpl, _ := Get("branch_create")
	return tmpl
}

// GetChangelogAreas returns the changelog_areas template
func GetChangelogAreas() string {
	tmpl, _ := Get("changelog_areas")
	return tmpl
}

// --- Params structs ---

// ClassifyBinaryParams for the classify_binary prompt.
type ClassifyBinaryParams struct {
	Diff             string
	AnnotatedSummary string
}

// BuildClassifyBinaryParams creates ClassifyBinaryParams from a diff string.
func BuildClassifyBinaryParams(diff, annotatedSummary string) ClassifyBinaryParams {
	return ClassifyBinaryParams{Diff: diff, AnnotatedSummary: annotatedSummary}
}

// SynthesisParams for commit message synthesis
type SynthesisParams struct {
	Context      string
	Why          string
	CommitType   string
	Scope        string
	Breaking     bool
	FileMessages []string
}

// BuildSynthesisParams creates SynthesisParams for commit synthesis
func BuildSynthesisParams(fileMessages []string, context, commitType, scope string, breaking bool, why string) SynthesisParams {
	return SynthesisParams{
		Context:      context,
		Why:          why,
		CommitType:   commitType,
		Scope:        scope,
		Breaking:     breaking,
		FileMessages: fileMessages,
	}
}

// MessageParams for commit message generation
type MessageParams struct {
	Files           string
	RejectedMessage string
	Context         string
	// AnnotatedDiff contains AST-based semantic annotations (optional)
	AnnotatedDiff string
	// Diff is the raw diff fallback when AnnotatedDiff is empty
	Diff string
	// Pre-classified by Go — LLM should NOT generate these
	CommitType string
	Scope      string
	Breaking   bool
	// Why is the user's reason for this change — flows into the LLM prompt
	Why string
}

// BuildMessageParams creates MessageParams for commit message
// BuildMessageParams creates MessageParams for commit message
func BuildMessageParams(files []string, annotatedDiff, rawDiff, context, commitType, scope string, breaking bool, why string) MessageParams {
	return MessageParams{
		Files:         joinFiles(files),
		AnnotatedDiff: annotatedDiff,
		Diff:          rawDiff,
		Context:       context,
		CommitType:    commitType,
		Scope:         scope,
		Breaking:      breaking,
		Why:           why,
	}
}

// BuildMessageParamsWithRetry creates MessageParams with rejection context
func BuildMessageParamsWithRetry(files []string, annotatedDiff, rawDiff, rejected, context, commitType, scope string, breaking bool, why string) MessageParams {
	return MessageParams{
		Files:           joinFiles(files),
		RejectedMessage: rejected,
		AnnotatedDiff:   annotatedDiff,
		Diff:            rawDiff,
		Context:         context,
		CommitType:      commitType,
		Scope:           scope,
		Breaking:        breaking,
		Why:             why,
	}
}

// ProjectDescriptionParams for the project_description prompt.
type ProjectDescriptionParams struct {
	DocContents string
}

// BuildProjectDescriptionParams creates ProjectDescriptionParams from doc content.
func BuildProjectDescriptionParams(docContents string) ProjectDescriptionParams {
	return ProjectDescriptionParams{DocContents: docContents}
}

// GetProjectDescription returns the project_description template.
func GetProjectDescription() string {
	tmpl, _ := Get("project_description")
	return tmpl
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