package openai_standard

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/shared/prompts"
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
// When CommitType is empty, falls back to InferCommitType for smart heuristic inference.
func extractCommitInfo(chunk domain.DiffChunk) (string, bool) {
	commitType := chunk.CommitType
	if commitType == "" {
		commitType = domain.InferCommitType(chunk)
	}
	breaking := strings.HasSuffix(commitType, "!")
	return strings.TrimSuffix(commitType, "!"), breaking
}

// buildChunkAnnotationJSON marshals the structured annotation fields of a
// DiffChunk into the JSON strings expected by MessageParams.
//
// annotatedJSON is empty when AnnotatedEntries is empty (so the template falls
// back to the legacy AnnotatedDiff / raw Diff). callGraphJSON is "[]" when
// there are no call edges (still rendered as an empty array alongside the
// annotated_diff block). cfgJSON is empty when CFGBefore/CFGAfter are nil (cfg
// not computed); otherwise it is a CFGSummary JSON object.
func buildChunkAnnotationJSON(chunk *domain.DiffChunk) (annotatedJSON, callGraphJSON, cfgJSON string) {
	if len(chunk.AnnotatedEntries) > 0 {
		data, err := json.Marshal(chunk.AnnotatedEntries)
		if err == nil {
			annotatedJSON = string(data)
		}
	}
	if annotatedJSON == "" {
		return "", "", ""
	}

	// Call graph: marshal to "[]" when empty so the template renders the block.
	cgData, err := json.Marshal(chunk.CallGraph)
	if err == nil {
		callGraphJSON = string(cgData)
	} else {
		callGraphJSON = "[]"
	}

	// CFG summary: only when both before/after are present (computed).
	if chunk.CFGBefore != nil && chunk.CFGAfter != nil {
		summary := domain.CFGSummary{
			Conditionals: domain.CFGEntry{Before: chunk.CFGBefore.Branch, After: chunk.CFGAfter.Branch},
			Loops:        domain.CFGEntry{Before: chunk.CFGBefore.Loop, After: chunk.CFGAfter.Loop},
			Returns:      domain.CFGEntry{Before: chunk.CFGBefore.Return, After: chunk.CFGAfter.Return},
			Errors:       domain.CFGEntry{Before: chunk.CFGBefore.Error, After: chunk.CFGAfter.Error},
		}
		if data, err := json.Marshal(summary); err == nil {
			cfgJSON = string(data)
		}
	}

	return annotatedJSON, callGraphJSON, cfgJSON
}

// GenerateChunkMessage generates a conventional commit message for a single diff chunk.
func (a *OpenAIStandardAdapter) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) {
	commitType, breaking := extractCommitInfo(chunk)

	annotatedJSON, callGraphJSON, cfgJSON := buildChunkAnnotationJSON(&chunk)
	annotatedDiff := chunk.AnnotatedDiff
	rawDiff := chunk.Diff
	// When structured entries are present, drop the legacy emoji AnnotatedDiff
	// so the prompt uses the JSON path exclusively (spec: no emoji in prompt
	// input, no raw diff when annotated_diff is non-empty).
	if annotatedJSON != "" {
		annotatedDiff = ""
		rawDiff = ""
	}

	var prompt string
	var err error
	if a.retryContext != "" {
		prompt, err = prompts.Render(prompts.GetCommitMessage(), prompts.BuildMessageParamsWithRetry(chunk.Files, annotatedJSON, callGraphJSON, cfgJSON, annotatedDiff, rawDiff, a.retryContext, a.context, commitType, chunk.Scope, breaking, a.why))
	} else {
		prompt, err = prompts.Render(prompts.GetCommitMessage(), prompts.BuildMessageParams(chunk.Files, annotatedJSON, callGraphJSON, cfgJSON, annotatedDiff, rawDiff, a.context, commitType, chunk.Scope, breaking, a.why))
	}
	if err != nil {
		return "", fmt.Errorf("render commit prompt: %w", err)
	}

	commit, err := a.commitJSONWithFallback(prompt, commitGenTemp, commitGenMaxTokens)
	if err != nil {
		return "", err
	}

	return commit.ToConventionalCommit(commitType, chunk.Scope, breaking), nil
}

// GenerateCommitSynthesis synthesizes multiple file-by-file commit messages into a single conventional commit message.
func (a *OpenAIStandardAdapter) GenerateCommitSynthesis(combinedChunk domain.DiffChunk, fileMessages []string) (string, error) {
	commitType, breaking := extractCommitInfo(combinedChunk)

	prompt, err := prompts.Render(prompts.GetCommitSynthesis(), prompts.BuildSynthesisParams(fileMessages, a.context, commitType, combinedChunk.Scope, breaking, a.why))
	if err != nil {
		return "", fmt.Errorf("render commit synthesis prompt: %w", err)
	}

	commit, err := a.commitJSONWithFallback(prompt, commitGenTemp, commitGenMaxTokens)
	if err != nil {
		return "", err
	}

	return commit.ToConventionalCommit(commitType, combinedChunk.Scope, breaking), nil
}

// commitJSONWithFallback tries jsonMode first, falls back to plain on parse failure.
func (a *OpenAIStandardAdapter) commitJSONWithFallback(prompt string, temperature float64, maxTokens int) (CommitMessageJSON, error) {
	opts := chatCompletionOpts{
		reasoningEffort: "none",
		temperature:     floatPtr(temperature),
		maxTokens:       maxTokens,
	}

	var commit CommitMessageJSON

	opts.operation = "commit"
	opts.jsonMode = true
	result, err := a.chatCompletion(prompt, opts)
	if err == nil {
		if commit, err = parseSingleOrArray(result); err == nil {
			return commit, nil
		}
	}

	opts.jsonMode = false
	opts.operation = "commit_fallback"
	result, err = a.chatCompletion(prompt, opts)
	if err != nil {
		return CommitMessageJSON{}, fmt.Errorf("model %q failed to generate a valid commit message: %w", a.model, err)
	}
	commit, err = parseSingleOrArray(result)
	if err != nil {
		return CommitMessageJSON{}, fmt.Errorf("model %q not suitable for commit generation: %w", a.model, err)
	}

	return commit, nil
}

// parseSingleOrArray tries to parse JSON as a single CommitMessageJSON,
// then as an array (taking the first element).
func parseSingleOrArray(raw string) (CommitMessageJSON, error) {
	var commit CommitMessageJSON
	if err := parseJSON(raw, &commit); err == nil {
		return commit, nil
	}

	var commits []CommitMessageJSON
	if err := parseJSON(raw, &commits); err == nil && len(commits) > 0 {
		return commits[0], nil
	}

	return CommitMessageJSON{}, ErrInvalidJSON
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

// ClassifyBinary categorizes a diff into exactly one of two categories with high precision.
func (a *OpenAIStandardAdapter) ClassifyBinary(diff string) (string, error) {
	tmpl := prompts.GetClassifyBinary()
	prompt, err := prompts.Render(tmpl, prompts.BuildClassifyBinaryParams(diff, ""))
	if err != nil {
		return "", fmt.Errorf("render classify_binary prompt: %w", err)
	}

	result, err := a.chatCompletion(prompt, chatCompletionOpts{
		operation:   "classify",
		temperature: floatPtr(0.1), // Very low temperature for deterministic classification
		maxTokens:   10,            // Only need a single word response
	})
	if err != nil {
		return "", fmt.Errorf("LLM classification failed: %w", err)
	}

	// Normalize the response to lower case and trim whitespace
	response := strings.ToLower(strings.TrimSpace(result))

	// Validate that the response is exactly "fix" or "refactor"
	if response != "fix" && response != "refactor" {
		return "", fmt.Errorf("invalid classification response: %s - expected 'fix' or 'refactor'", response)
	}

	return response, nil
}
