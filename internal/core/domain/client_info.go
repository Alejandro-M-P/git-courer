package domain

// ClientInfo represents the MCP client information from initialize handshake
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ClientCapabilities represents what the client supports
type ClientCapabilities struct {
	Sampling    bool `json:"sampling"`
	Elicitation bool `json:"elicitation"`
}

// KnownClients maps client names to their behaviors
var KnownClients = map[string]ClientBehavior{
	"opencode": {
		Name:                "opencode",
		SupportsElicitation: true,
		SupportsSampling:    false, // Coming soon
	},
	"claude-code": {
		Name:                "claude-code",
		SupportsElicitation: false,
		SupportsSampling:    false,
	},
	"claude-desktop": {
		Name:                "claude-desktop",
		SupportsElicitation: false,
		SupportsSampling:    false,
	},
}

// ClientBehavior defines how a client handles confirmations
type ClientBehavior struct {
	Name                string
	SupportsElicitation bool
	SupportsSampling    bool
}

// GetClientBehavior returns the behavior for a known client or default fallback
func GetClientBehavior(name string) ClientBehavior {
	if behavior, ok := KnownClients[name]; ok {
		return behavior
	}
	// Default fallback
	return ClientBehavior{
		Name:                "unknown",
		SupportsElicitation: false,
		SupportsSampling:    false,
	}
}
