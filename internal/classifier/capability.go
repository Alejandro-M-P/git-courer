package classifier

import "strings"

// ClientCapability representa las capacidades del cliente MCP
type ClientCapability struct {
	Name                 string
	InteractiveQuestions bool // Soporta QuestionTool/AskUserQuestion
	EditableParams       bool // Puede editar parámetros antes de ejecutar
}

// DetectClientCapability detecta las capacidades del cliente desde User-Agent
func DetectClientCapability(userAgent string) ClientCapability {
	switch {
	case contains(userAgent, "Claude Code"):
		return ClientCapability{
			Name:                 "Claude Code",
			InteractiveQuestions: true,
			EditableParams:       true,
		}
	case contains(userAgent, "OpenCode"):
		return ClientCapability{
			Name:                 "OpenCode",
			InteractiveQuestions: true,
			EditableParams:       false,
		}
	default:
		// Otros clientes: fallback a chat
		return ClientCapability{
			Name:                 "Unknown",
			InteractiveQuestions: false,
			EditableParams:       false,
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s[:len(substr)] == substr ||
			strings.Contains(s, substr))
}
