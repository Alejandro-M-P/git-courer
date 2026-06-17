package rewrite

import (
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
)

func TestRewriteRegistration_EnumValues(t *testing.T) {
	s := server.NewMCPServer("test", "1.0")
	h := &Handler{}
	Register(s, h)

	tools := s.ListTools()
	st, ok := tools["rewrite"]
	assert.True(t, ok, "rewrite tool should be registered")

	props := st.Tool.InputSchema.Properties
	cmdRaw, ok := props["command"]
	assert.True(t, ok, "rewrite should have command param")

	cmdMap, ok := cmdRaw.(map[string]any)
	assert.True(t, ok, "command should be a map")

	enumVals, ok := cmdMap["enum"].([]string)
	assert.True(t, ok, "command should have enum constraint")

	want := []string{"AMEND", "REVERT", "SOFT", "HARD"}
	assert.Equal(t, want, enumVals, "rewrite enum should be AMEND/REVERT/SOFT/HARD")
}

func TestRewriteRegistration_Description(t *testing.T) {
	s := server.NewMCPServer("test", "1.0")
	h := &Handler{}
	Register(s, h)

	tools := s.ListTools()
	st, ok := tools["rewrite"]
	assert.True(t, ok)
	assert.NotEmpty(t, st.Tool.Description, "rewrite should have a description")
}

func TestRewriteRegistration_DestructiveHint(t *testing.T) {
	s := server.NewMCPServer("test", "1.0")
	h := &Handler{}
	Register(s, h)

	tools := s.ListTools()
	st, ok := tools["rewrite"]
	assert.True(t, ok)
	assert.NotNil(t, st.Tool.Annotations.DestructiveHint, "rewrite should have destructive hint annotation")
	assert.True(t, *st.Tool.Annotations.DestructiveHint, "rewrite should be marked destructive")
}
