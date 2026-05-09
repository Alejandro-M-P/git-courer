package openai_standard

import (
	"fmt"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/shared/prompts"
)

// CommitMessageJSON is the structured output from the commit LLM prompt.
// The LLM only generates description and body; type + breaking are classified by Go.
type CommitMessageJSON struct {
	Description string `json:"description"`
	Body        string `json:"body"`
}

// ToConventionalCommit formats the CommitMessageJSON as a conventional commit string.
// scope is included as type(scope): if non-empty.
func (c CommitMessageJSON) ToConventionalCommit(commitType, scope string, breaking bool) string {
	prefix := commitType
	if scope != "" {
		prefix = fmt.Sprintf("%s(%s)", commitType, scope)
	}
	if breaking {
		prefix += "!"
	}
	var sb strings.Builder
	sb.WriteString(prefix + ": " + c.Description)
	if c.Body != "" {
		sb.WriteString("\n\n" + c.Body)
	}
	return sb.String()
}

// extractCommitInfo extracts the commit type and breaking flag from a DiffChunk.
// CommitType may carry a "!" suffix to indicate breaking (e.g. "feat!").
func extractCommitInfo(chunk domain.DiffChunk) (string, bool) {
	if chunk.CommitType == "" {
		return "chore", false
	}
	breaking := strings.HasSuffix(chunk.CommitType, "!")
	return strings.TrimSuffix(chunk.CommitType, "!"), breaking
}

// GenerateChunkMessage generates a conventional commit message for a single diff chunk.
func (a *OpenAIStandardAdapter) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) {
	var prompt string
	var err error
	if a.retryContext != "" {
		prompt, err = prompts.Render(prompts.GetCommitMessage(), prompts.BuildMessageParamsWithRetry(chunk.Files, chunk.Diff, a.retryContext, a.context))
	} else {
		annotatedDiff := ""
		if chunk.AnnotatedDiff != "" {
			annotatedDiff = chunk.AnnotatedDiff
		}
		prompt, err = prompts.Render(prompts.GetCommitMessage(), prompts.BuildMessageParams(chunk.Files, chunk.Diff, annotatedDiff, a.context))
	}
	if err != nil {
		return "", fmt.Errorf("render commit prompt: %w", err)
	}

	result, err := a.chatCompletion(prompt, chatCompletionOpts{
		operation:       "commit",
		jsonMode:        true,
		reasoningEffort: "none",
		temperature:     floatPtr(commitGenTemp),
		maxTokens:       commitGenMaxTokens,
	})
	if err != nil {
		return "", err
	}

	var commit CommitMessageJSON
	if err := parseJSON(result, &commit); err != nil {
		return "", fmt.Errorf("parse commit message: %w", err)
	}

	commitType, breaking := extractCommitInfo(chunk)
	return commit.ToConventionalCommit(commitType, chunk.Scope, breaking), nil
}

// DecideCommit determines what files to stage based on user instruction and git status.
func (a *OpenAIStandardAdapter) DecideCommit(instruction, gitStatus, untracked, modified, deleted string) (domain.CommitIntent, error) {
	prompt, err := prompts.Render(prompts.GetDecideCommit(), prompts.BuildDecideParams(instruction, gitStatus, untracked, modified, deleted))
	if err != nil {
		return domain.CommitIntent{}, fmt.Errorf("render decide commit prompt: %w", err)
	}

	result, err := a.chatCompletion(prompt, chatCompletionOpts{
		operation:       "decide_commit",
		jsonMode:        true,
		reasoningEffort: "none",
		temperature:     floatPtr(decideTemp),
		maxTokens:       decideMaxTokens,
	})
	if err != nil {
		return domain.CommitIntent{}, err
	}

	var decision struct {
		IncludeUntracked bool     `json:"include_untracked"`
		FileFilter       []string `json:"file_filter"`
	}
	if err := parseJSON(result, &decision); err != nil {
		return domain.CommitIntent{}, fmt.Errorf("parse decide commit: %w", err)
	}

	return domain.CommitIntent{
		IncludeUntracked: decision.IncludeUntracked,
		Filter:           decision.FileFilter,
	}, nil
}

// InterpretGitOp interprets a natural language instruction for a git operation.
func (a *OpenAIStandardAdapter) InterpretGitOp(op, instruction string, context map[string]string) (map[string]string, error) {
	if context == nil {
		context = make(map[string]string)
	}
	context["Instruction"] = instruction

	tmpl, err := prompts.Get(op)
	if err != nil {
		// Fallback to a generic JSON prompt for undocumented git operations like branch/merge
		tmpl = `Interpret this git instruction: "{{.Instruction}}"\nRespond with ONLY a JSON object with the relevant arguments.`
	}
	prompt, err := prompts.Render(tmpl, context)
	if err != nil {
		return nil, fmt.Errorf("render interpret prompt: %w", err)
	}

	result, err := a.chatCompletion(prompt, chatCompletionOpts{
		operation:       op,
		jsonMode:        true,
		reasoningEffort: "none",
		temperature:     floatPtr(interpretTemp),
		maxTokens:       interpretMaxTokens,
	})
	if err != nil {
		return nil, err
	}

	var parsed map[string]interface{}
	if err := parseJSON(result, &parsed); err != nil {
		return nil, fmt.Errorf("parse git op: %w", err)
	}

	args := make(map[string]string)
	for k, v := range parsed {
		args[k] = fmt.Sprintf("%v", v)
	}
	return args, nil
}
