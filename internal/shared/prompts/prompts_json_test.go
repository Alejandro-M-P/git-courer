package prompts

import (
	"strings"
	"testing"
)

// TestMessageParams_JSONFields verifies the new JSON string fields are present
// and flow through BuildMessageParams.
func TestMessageParams_JSONFields(t *testing.T) {
	params := BuildMessageParams(
		[]string{"a.go"},
		`[{"file":"a.go","symbol":"F","type":"MOD_SIG"}]`,
		`[{"from":"a.go","to":"b.go","symbol":"G"}]`,
		`{"conditionals":{"before":0,"after":1}}`,
		"",
		"raw diff",
		"ctx",
		"fix",
		"core",
		false,
		"why text",
	)
	if params.AnnotatedJSON != `[{"file":"a.go","symbol":"F","type":"MOD_SIG"}]` {
		t.Errorf("AnnotatedJSON: got %q", params.AnnotatedJSON)
	}
	if params.CallGraphJSON != `[{"from":"a.go","to":"b.go","symbol":"G"}]` {
		t.Errorf("CallGraphJSON: got %q", params.CallGraphJSON)
	}
	if params.CFGJSON != `{"conditionals":{"before":0,"after":1}}` {
		t.Errorf("CFGJSON: got %q", params.CFGJSON)
	}
	// Legacy AnnotatedDiff should still be populated from the rawDiff fallback
	// path — here we pass annotatedDiff as the legacy string for back-compat.
	if params.Diff != "raw diff" {
		t.Errorf("Diff: got %q, want raw diff", params.Diff)
	}
}

// TestBuildMessageParamsWithRetry_JSONFields verifies the retry variant carries
// the JSON fields and the rejected message.
func TestBuildMessageParamsWithRetry_JSONFields(t *testing.T) {
	params := BuildMessageParamsWithRetry(
		[]string{"a.go"},
		`[{"file":"a.go"}]`,
		`[]`,
		`null`,
		"",
		"raw",
		"rejected msg",
		"ctx",
		"feat",
		"",
		true,
		"because",
	)
	if params.AnnotatedJSON != `[{"file":"a.go"}]` {
		t.Errorf("AnnotatedJSON: got %q", params.AnnotatedJSON)
	}
	if params.CallGraphJSON != `[]` {
		t.Errorf("CallGraphJSON: got %q", params.CallGraphJSON)
	}
	if params.CFGJSON != `null` {
		t.Errorf("CFGJSON: got %q", params.CFGJSON)
	}
	if params.RejectedMessage != "rejected msg" {
		t.Errorf("RejectedMessage: got %q", params.RejectedMessage)
	}
	if !params.Breaking {
		t.Errorf("Breaking: got false, want true")
	}
}

// TestRender_CommitMessage_WithAnnotatedJSON verifies the template renders the
// new JSON block when AnnotatedJSON is populated.
func TestRender_CommitMessage_WithAnnotatedJSON(t *testing.T) {
	data := MessageParams{
		Files:         "a.go",
		AnnotatedJSON: `[{"file":"a.go","symbol":"F","type":"MOD_SIG","before":"-x","after":"+y"}]`,
		CallGraphJSON: `[]`,
		CFGJSON:       `null`,
	}
	got, err := Render(GetCommitMessage(), data)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(got, "annotated_diff") {
		t.Errorf("rendered prompt should contain annotated_diff block; got:\n%s", got)
	}
	if !strings.Contains(got, `"symbol":"F"`) {
		t.Errorf("rendered prompt should contain the JSON entry; got:\n%s", got)
	}
}

// TestRender_CommitMessage_FallsBackToDiff_WhenNoAnnotatedJSON verifies the
// raw diff is rendered when AnnotatedJSON is empty.
func TestRender_CommitMessage_FallsBackToDiff_WhenNoAnnotatedJSON(t *testing.T) {
	data := MessageParams{
		Files: "a.go",
		Diff:  "+added raw line",
	}
	got, err := Render(GetCommitMessage(), data)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(got, "+added raw line") {
		t.Errorf("rendered prompt should contain raw diff fallback; got:\n%s", got)
	}
	if strings.Contains(got, "annotated_diff") {
		t.Errorf("rendered prompt should NOT contain annotated_diff when empty; got:\n%s", got)
	}
}

// TestRender_CommitMessage_LegacyAnnotatedDiff_WhenNoJSON verifies the legacy
// emoji AnnotatedDiff is still rendered when AnnotatedJSON is empty but
// AnnotatedDiff is populated (backward compat).
func TestRender_CommitMessage_LegacyAnnotatedDiff_WhenNoJSON(t *testing.T) {
	data := MessageParams{
		Files:         "a.go",
		AnnotatedDiff: "legacy emoji annotation text",
	}
	got, err := Render(GetCommitMessage(), data)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if !strings.Contains(got, "legacy emoji annotation text") {
		t.Errorf("rendered prompt should contain legacy AnnotatedDiff; got:\n%s", got)
	}
}

// TestRender_CommitMessage_NoEmojiInTemplate verifies the template itself
// contains no emoji characters (per spec: no emoji in prompt input).
func TestRender_CommitMessage_NoEmojiInTemplate(t *testing.T) {
	tmpl := GetCommitMessage()
	for _, r := range tmpl {
		if r >= 0x1F000 { // emoji codepoint range
			t.Errorf("template contains emoji rune %U; template must be emoji-free", r)
		}
	}
}