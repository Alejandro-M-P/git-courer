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
