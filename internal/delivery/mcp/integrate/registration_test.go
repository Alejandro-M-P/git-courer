package integrate

import (
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
)

func TestIntegrateRegistration_EnumValues(t *testing.T) {
	s := server.NewMCPServer("test", "1.0")
	h := &Handler{}
	Register(s, h)

	tools := s.ListTools()
	st, ok := tools["integrate"]
	assert.True(t, ok, "integrate tool should be registered")

	props := st.Tool.InputSchema.Properties
	cmdRaw, ok := props["command"]
	assert.True(t, ok, "integrate should have command param")

	cmdMap, ok := cmdRaw.(map[string]any)
	assert.True(t, ok, "command should be a map")

	enumVals, ok := cmdMap["enum"].([]string)
	assert.True(t, ok, "command should have enum constraint")

	want := []string{"MERGE", "UPDATE", "PICK", "CONTINUE", "ABORT"}
	assert.Equal(t, want, enumVals, "integrate enum should be MERGE/UPDATE/PICK/CONTINUE/ABORT")
}

func TestIntegrateRegistration_Description(t *testing.T) {
	s := server.NewMCPServer("test", "1.0")
	h := &Handler{}
	Register(s, h)

	tools := s.ListTools()
	st, ok := tools["integrate"]
	assert.True(t, ok)
	assert.NotEmpty(t, st.Tool.Description, "integrate should have a description")
}

func TestIntegrateRegistration_DestructiveHint(t *testing.T) {
	s := server.NewMCPServer("test", "1.0")
	h := &Handler{}
	Register(s, h)

	tools := s.ListTools()
	st, ok := tools["integrate"]
	assert.True(t, ok)
	assert.NotNil(t, st.Tool.Annotations.DestructiveHint, "integrate should have destructive hint annotation")
	assert.True(t, *st.Tool.Annotations.DestructiveHint, "integrate should be marked destructive")
}
