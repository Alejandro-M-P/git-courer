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

	"github.com/Alejandro-M-P/git-courer/internal/config"
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
// Returns an error if the operation template is not found.
func Get(op string) (string, error) {
	if tmpl, ok := templateCache[op]; ok {
		return tmpl, nil
	}
	return "", fmt.Errorf("prompt template for operation '%s' not found", op)
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
	tmpl, err := Get(op)
	if err != nil {
		return "", err
	}
	return Render(tmpl, params)
}

// GetCommitMessage returns the commit message template
func GetCommitMessage() string {
	tmpl, _ := Get("commit_message")
	return tmpl
}

// GetDecideCommit returns the decide commit template
func GetDecideCommit() string {
	tmpl, _ := Get("decide_commit")
	return tmpl
}

// --- Params structs ---

// MessageParams for commit message generation
type MessageParams struct {
    CurrentBranch   string
    Files           string
    Diff            string
    RejectedMessage string
    Context         string
    // AnnotatedDiff contains AST-based semantic annotations (optional)
    AnnotatedDiff   string
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
func BuildMessageParams(files []string, diff, annotatedDiff, context string) MessageParams {
    return MessageParams{
        Files: joinFiles(files), 
        Diff: diff,
        AnnotatedDiff: annotatedDiff,
        Context: context,
    }
}

// BuildMessageParamsWithRetry creates MessageParams with rejection context
func BuildMessageParamsWithRetry(files []string, diff, rejected, context string) MessageParams {
    return MessageParams{
        Files: joinFiles(files), 
        Diff: diff,
        RejectedMessage: rejected,
        Context: context,
        AnnotatedDiff: "", // Empty for retry scenarios
    }
}

// FormatContext renders a non-empty context string from ContextConfig.
// Returns empty string when both fields are empty.
func FormatContext(cfg config.ContextConfig) string {
	project := strings.TrimSpace(cfg.Project)
	style := strings.TrimSpace(cfg.Style)
	parts := make([]string, 0, 2)
	if project != "" {
		parts = append(parts, "Project description: "+project)
	}
	if style != "" {
		parts = append(parts, "Style: "+style)
	}
	return strings.Join(parts, "\n")
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

// ProjectAreasParams for the project_areas prompt.
type ProjectAreasParams struct {
	DirectoryTree string
}

// BuildProjectAreasParams creates ProjectAreasParams from a directory tree string.
func BuildProjectAreasParams(directoryTree string) ProjectAreasParams {
	return ProjectAreasParams{DirectoryTree: directoryTree}
}

// GetProjectAreas returns the project_areas template.
func GetProjectAreas() string {
	tmpl, _ := Get("project_areas")
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
