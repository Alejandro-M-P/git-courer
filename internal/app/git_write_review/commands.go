package git_write_review

// All confirmable git operations share the same three-phase flow:
//   <OP>_START  → prepare, optionally save plan + blocker (if preview required by config)
//   <OP>_APPLY  → execute the persisted plan
//   <OP>_ABORT  → cancel and clean up
//
// Whether an operation requires confirmation is controlled by preview.operations in config.
// Operations not present in ConfigKey always execute directly on _START.
const (
	// Commit (uses LLM to prepare messages)
	COMMIT = "COMMIT"

	// Branch operations
	BRANCH_CREATE = "BRANCH_CREATE"
	BRANCH_DELETE = "BRANCH_DELETE"
	BRANCH_RENAME = "BRANCH_RENAME"

	// Tag operations
	TAG_CREATE = "TAG_CREATE"
	TAG_DELETE = "TAG_DELETE"

	// Merge / rebase
	MERGE           = "MERGE"
	REBASE          = "REBASE"
	REBASE_CONTINUE = "REBASE_CONTINUE"
	REBASE_ABORT    = "REBASE_ABORT"

	// Reset
	RESET_SOFT = "RESET_SOFT"
	RESET_HARD = "RESET_HARD"

	// Other destructive / creative ops
	CLEAN         = "CLEAN"
	REMOTE_ADD    = "REMOTE_ADD"
	REMOTE_REMOVE = "REMOTE_REMOVE"
	CHERRY_PICK   = "CHERRY_PICK"
	REVERT        = "REVERT"
	INIT          = "INIT"
	CLONE         = "CLONE"

	// Utility phases (no op prefix needed)
	STATUS  = "STATUS"  // check current pending plan
	SUMMARY = "SUMMARY" // git status summary

	// Phase suffixes applied to every operation command
	PhaseSuffixStart = "_START"
	PhaseSuffixApply = "_APPLY"
	PhaseSuffixAbort = "_ABORT"
)

// ConfigKey maps each operation to its key in preview.operations config.
// Operations not listed here always execute directly (no confirmation required).
var ConfigKey = map[string]string{
	COMMIT:        "commit",
	BRANCH_CREATE: "branch_create",
	BRANCH_DELETE: "branch_delete",
	MERGE:         "merge",
	RESET_HARD:    "reset_hard",
	REBASE:        "rebase",
}
