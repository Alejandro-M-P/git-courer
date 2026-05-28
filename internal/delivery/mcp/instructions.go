package mcp

// gitCourerSummary is the system instructions shown to LLM agents on MCP initialize.
// It's designed for agents — concise, focused on what the tool does for YOU.
// Test: instructions_test.go / TestInstructions_SummaryReferencesRealToolNames
const gitCourerSummary = `= git-courer — structured git for agent workflows =

Every tool returns JSON. No text parsing, no pager hangs, no grep over git status.
Every mutation auto-creates a backup — undo any mistake with a single call.
Destructive operations (push, hard reset, branch delete) require explicit confirmed=true.

CORE TOOLS — you will use these constantly:
  status     — full repo state in one call (branch, staged, unstaged, conflicts, stash, last commit)
  diff       — annotated diff with AST labels: [NEW_FUNC] [MOD_SIG ⚠BREAKING] [DEPS] [DEL]
  commit     — atomic commits with cloud+local LLM writing WHY/WHAT messages together
  pr-review  — pre-PR gate: runs tests, detects conflicts, shows diff stats

GOLDEN RULES:
  1. BEFORE any mutation → status (know the repo state)
  2. BEFORE push → diff + ask "does this look right?"
  3. BEFORE creating/updating a PR → pr-review

TOOLS: status diff commit amend revert branch merge rebase cherry_pick stage reset stash history blame sync pr-review commit-jobs config backup undo remotes tag`

const descStatus = `Full repo state in one call: branch, ahead/behind, staged, unstaged, untracked, conflicted files, stash count, in-progress operations, last commit. Call before any write operation.`

const descDiff = `Annotated diff with AST labels in @@ headers — see WHAT changed at symbol level, not raw lines. Returns hunks labeled [NEW_FUNC], [MOD_SIG ⚠BREAKING], [DEPS], [DEL]. Paginated. Call before push or PR.`

const descCommit = `Atomic commit pipeline. Supports two modes:
  PREVIEW+APPLY — DiffChunker parses AST, groups files by dependency, generates commits with WHY/WHAT messages via cloud+local LLM collaboration. Area mappings supported for team changelogs (area_response param).
  APPLY (direct) — Creates a single atomic commit from working tree changes.

Commit messages include:
  - WHAT: structured description of the change
  - WHY: context and justification
  - Test evidence references when applicable

All commits run hooks. No --no-verify bypass.`

const descAmend = `Fix the last commit — change message, add files, or both. Creates backup BEFORE executing. Use dry_run=true to preview first. confirmed=true required to execute.`

const descRevert = `Revert a commit by creating a new commit that undoes it. Creates backup BEFORE executing. Use dry_run=true to preview, confirmed=true to execute.`

const descBranch = `Branch lifecycle — CREATE, DELETE, RENAME, REMOTE_DELETE, SET_UPSTREAM, UNSET_UPSTREAM, SWITCH. SWITCH auto-stashes dirty tree. DELETE/REMOTE_DELETE require confirmed=true.`

const descMerge = `Merge with structured conflict detection. Returns conflict file list on failure — no text parsing needed. Supports delete_source, push_after, new_branch composition in one call.`

const descRebase = `Rebase with the same structured conflict contract as merge. Returns conflicted_files on failure. Supports --onto for transplanting. Do not use for preserving merge history — use merge instead.`

const descCherryPick = `Apply a specific commit onto the current branch. Creates backup; undo with backup RESTORE.`

const descStage = `Stage, unstage, restore, or clean files. Binary files intercepted before staging. CLEAN requires confirmed=true.`

const descReset = `Undo commits at different safety levels: SOFT (move HEAD), MIXED (unstages too), HARD (discards everything — requires confirmed=true). Use dry_run=true to preview.`

const descStash = `Save, restore, or inspect stashed changes. SAVE stores working tree changes, POP restores them, SHOW previews content without popping.`

const descHistory = `Commit history (LOG) or reflog (REFLOG) with pagination and filtering. Structured JSON — no pager hangs.`

const descBlame = `Line-by-line attribution for a file — who changed what and when. Returns JSON per line.`

const descSync = `PUSH (irreversible, requires confirmed=true), PULL (auto-backup before merging), FETCH (check remote changes safely). Use dry_run=true to preview what would be pushed.`

const descPrReview = `Pre-PR gate: runs tests, detects conflicts, shows diff stats, and checks branch divergence against the target branch (default: main). Call BEFORE creating ANY PR — no exceptions.`

const descCommitJobs = `List active commit pipeline jobs — running, done, or failed — with status, commit message, and tree hash. Read-only.`

const descConfig = `Read or update project configuration. GET returns all config. SET_TEST_COMMAND, SET_USER_NAME, SET_USER_EMAIL, SET_SIGNING_KEY persist settings in project-local config.`

const descBackup = `Manage git backups. Every write operation auto-creates one. CREATE, RESTORE (undo), DELETE (requires confirmed), or LIST (show available with undoable indicators). Max 20 backups — oldest auto-pruned.`

const descUndo = `Shortcut for backup RESTORE. Undo the most recent destructive git operation (amend, merge, rebase, revert) by restoring the latest backup.`

const descRemotes = `ADD or REMOVE remote URLs. REMOVE requires confirmed=true (NOT undoable via backup).`

const descTag = `CREATE annotated tags, DELETE tags locally or remotely, PUSH tags. DELETE operations require confirmed=true.`
