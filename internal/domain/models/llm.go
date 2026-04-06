package models

// CommitMessage represents an AI-generated commit message
type CommitMessage struct {
	Type    string // feat, fix, chore, docs, etc.
	Subject string // Short description
	Full    string // Full conventional commit message
}

// SecretDetection represents a detected secret
type SecretDetection struct {
	File    string
	Line    int
	Type    string // api_key, password, token, etc.
	Content string // The detected secret (redacted)
}

// ContextRequest - response from Ollama's first call (what context is needed)
type ContextRequest struct {
	FilesNeeded []string `json:"files_needed"` // which files to read
	DiffNeeded  bool     `json:"diff_needed"`  // whether diff is needed
	BranchInfo  bool     `json:"branch_info"`  // whether branch info is needed
	StatusInfo  bool     `json:"status_info"`  // whether status is needed
	LogInfo     bool     `json:"log_info"`     // whether log is needed
	Description string   `json:"description"`  // what Ollama will do with this context
}

// GitDecision - full decision JSON from Ollama's second call
type GitDecision struct {
	Type       string            `json:"type"`       // read|write
	Strategy   string            `json:"strategy"`   // single|split
	Commits    []CommitPlan      `json:"commits"`    // commit plans
	Summary    DecisionSummary   `json:"summary"`    // summary
	Secrets    []SecretDetection `json:"secrets"`    // detected secrets
	Suspicious []SuspiciousFile  `json:"suspicious"` // suspicious files
}

// CommitPlan - individual commit plan
type CommitPlan struct {
	Files    []string `json:"files"`    // files for this commit
	Commit   string   `json:"commit"`   // commit message
	Commands []string `json:"commands"` // git commands to execute
}

// DecisionSummary - summary of the decision
type DecisionSummary struct {
	Action string   `json:"action"` // what will be done
	Files  []string `json:"files"`  // files involved
	Branch string   `json:"branch"` // branch (if applicable)
}

// SuspiciousFile - file that looks suspicious
type SuspiciousFile struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
}

// LLMPort defines what AI operations the core can do
type LLMPort interface {
	GetContextNeeded(instruction string) (ContextRequest, error)
	GetFullDecision(instruction, context string) (GitDecision, error)
	IsAvailable() bool
}
