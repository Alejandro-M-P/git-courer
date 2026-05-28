// Package descriptions is the single source of truth for all MCP tool descriptions
// and the server instructions shown to LLM agents.
// Registration files import from here to avoid duplicating strings across packages.
package descriptions

// GitCourerSummary is the system instructions shown to LLM agents on MCP initialize.
// It's designed for agents — concise, focused on why git-courer serves agents better
// than raw bash git commands. First 512 chars should be self-contained for clients
// that truncate (e.g., Codex).
const GitCourerSummary = `= git-courer — structured git for agent workflows =

git-courer is DESIGNED for AI agents — every tool returns structured JSON that you
consume directly, NOT text you must parse. Raw bash git was built for humans reading
a terminal. git-courer was built for agents consuming data.

WHY GIT-COURER OVER BASH GIT:
• Structured status: one call = branch + ahead/behind + staged + unstaged +
  conflicts + stash + last commit. That is 5+ bash calls in one JSON response.
• AST-annotated diffs: see [NEW_FUNC] [MOD_SIG ⚠BREAKING] [DEPS] [DEL] instead of
  raw lines. You understand the change semantically, not just textually.
• Commit pipeline (cloud + local LLM collaboration): two LLMs work together to
  produce precise diffs and human-readable commits — grouped by dependency, with
  WHAT/WHY messages. The result is a repo humans can actually maintain.
• Pre-PR gate (pr-review): tests + conflicts + diff stats + divergence in one
  tool call. Replaces 4+ separate bash commands strung together.
• Auto-backup on every mutation: undo any mistake with one call — no data loss.
• Structured merge/rebase conflicts: grepping through ">>>>>>" text is gone.
• Safety gates: destructive operations need explicit confirmed=true.

COST GUIDE — plan your token budget:
  Fast (call freely):    status, diff, stage, stash, blame, history,
                         config, commit-jobs, undo, backup
  Medium (I/O cost):     sync (PULL/FETCH), cherry_pick, branch,
                         amend, revert, tag, remotes
  Heavy (plan around):   commit (PREVIEW + LLM collaboration), merge/rebase
                         (resolution cycles), pr-review (runs tests),
                         sync/PUSH (irreversible), reset/HARD (requires ✓)

ERRORS YOU WILL ENCOUNTER — handle them:
  status          → conflicted_files present → resolve before mutations
  diff            → empty result = no changes (not an error)
  commit          → hooks reject → create new commit (do NOT amend the failure)
  sync/PUSH       → rejected → fetch + diff first
  merge/rebase    → returns conflicted_files → read + diff + stage + continue
  stage/clean     → blocked without confirmed=true
  branch/delete   → blocked without confirmed=true
  config/get      → returns ALL config, not just a single key

GOLDEN RULES — save tokens and prevent mistakes:
  1. BEFORE any mutation → status (know the repo state)
  2. BEFORE push → diff + review
  3. BEFORE PR → pr-review (all checks in one call)

TOOLS: status diff commit amend revert branch merge rebase cherry_pick stage reset stash history blame sync pr-review commit-jobs config backup undo remotes tag`

// Core tools
const (
	DescStatus = `Returns COMPLETE repo state in ONE call — branch, ahead/behind, staged, unstaged, untracked, conflicted files, stash count, in-progress operations, last commit. Call BEFORE any write operation to know repo state. Do NOT use raw git status — this replaces 5+ bash calls.`

	DescDiff = `Annotated diff with AST labels in @@ headers — see WHAT changed at symbol level, not raw lines. Returns hunks labeled [NEW_FUNC], [MOD_SIG ⚠BREAKING], [DEPS], [DEL]. Paginated — no pager hangs. Call before pushing or creating a PR to review what will go up.`

	DescCommit = `3-phase commit pipeline: PREVIEW parses AST and groups files by dependency graph into atomic commits, APPLY executes them. PREVIEW accepts a 'why' parameter to justify changes. APPLY supports two paths: 1) With job_id: creates a single atomic commit from the PREVIEW tree snapshot via plumbing (CommitTree + UpdateRef), 2) Without job_id: executes the pending plan from ConfirmStore. Workflow: 1) PREVIEW → get plan, 2) Review with user, 3) APPLY. push_after:true on APPLY automatically pushes successful commits to remote. If PREVIEW returns 'processing', poll STATUS with job_id to get the result. If PREVIEW returns 'area_required', reply with area_response to assign directories to areas before continuing.`

	DescAmend = `Fix the last commit — change message, add files, or both. Use when the last commit needs fixing. Do NOT use for new changes (use commit instead). Creates backup BEFORE executing; undo with backup RESTORE. WITHOUT confirmed=true, the operation is BLOCKED and does NOT run.`

	DescRevert = `Revert a commit by creating a new commit that undoes it. Creates backup BEFORE executing; undo with backup RESTORE. WITHOUT confirmed=true, the operation is BLOCKED and does NOT run. Use dry_run=true first to see what will be reverted.`

	DescCommitJobs = `List active commit pipeline jobs — their status, commit message, and tree hash. Read-only tool for inspecting background jobs.`
)

// Branching tools
const (
	DescBranch = `Branch lifecycle — CREATE, DELETE, RENAME, REMOTE_DELETE, SET_UPSTREAM, UNSET_UPSTREAM, SWITCH. Use for branch management instead of raw 'git branch' / 'git switch' — lifecycle safety, auto-stash, and structured output. DELETE and REMOTE_DELETE require confirmed=true — without it, the operation is blocked. SWITCH auto-stashes dirty tree. Do NOT use for merging — use merge instead.`

	DescMerge = `Merge a branch into the current branch (or into_branch) with structured conflict detection — returns conflict file list instead of raw text to parse. After successful merge, delete_source:true removes the source branch, push_after:true pushes to remote, and new_branch:"name" creates and switches to a new branch. All composition steps only run if merge succeeds without conflicts.`

	DescRebase = `Rebase current branch onto a target branch. Same structured conflict output as merge. After resolving conflicts, call rebase with continue=true. Skip a conflicting commit with skip=true. Use --onto to transplant a branch onto a different base. Do NOT use for preserving merge history — use merge instead.`

	DescTag = `Tag lifecycle — CREATE annotated tags, DELETE tags locally or remotely, PUSH tags. Use for version management. Do NOT use for commits. DELETE operations require confirmation.`

	DescCherryPick = `Apply a specific commit onto the current branch. Use for selectively bringing changes from one branch to another. Do NOT use for full branch integration — use merge instead. Creates backup; undo with backup RESTORE.`
)

// Stage tools
const (
	DescStage = `Stage, unstage, restore, or clean files. Use instead of raw 'git add' / 'git clean' — structured JSON, safety gates, and binary-file interception before staging. CLEAN requires confirmed=true. Do NOT use for committing — use commit instead.`

	DescReset = `Undo commits at different safety levels. SOFT moves HEAD only (safest). MIXED unstages too. HARD discards everything — requires confirmed=true. Use dry_run=true to preview. Do NOT use for amending the last commit — use amend instead.`

	DescStash = `Save, restore, or inspect stashed changes. SAVE stores working tree changes for later. POP restores them. SHOW previews stash diff as structured JSON — no parsing 'git stash list' text. Use before switching branches with dirty tree.`
)

// Sync tools
const (
	DescSync = `Push, pull, or fetch from remote. PUSH is IRREVERSIBLE and requires confirmed=true. PULL and FETCH create a backup before executing. Use FETCH to check remote changes without merging (safer than PULL). When branch is specified, pushes/pulls only that branch instead of the current one. Always call diff before pushing.`

	DescRemotes = `Manage remote repositories — ADD a new remote URL or REMOVE an existing one. REMOVE requires confirmed=true and is NOT undoable via backup.`
)

// History tools
const (
	DescHistory = `Show commit history (LOG) or reflog (REFLOG) with pagination and filtering. Structured JSON — no pager hangs, no unstructured text. Use LOG for commit history, REFLOG for recovery operations. Do NOT use raw git log.`

	DescBlame = `Line-by-line attribution for a specific file — who changed what and when. Returns JSON per line. Do NOT use raw git blame — this gives structured data, no text parsing.`
)

// Utility tools
const (
	DescConfig = `Read or update project configuration. GET returns all config (global and project-level). SET_TEST_COMMAND saves the project test command for release validation. SET_USER_NAME, SET_USER_EMAIL, and SET_SIGNING_KEY persist git identity settings in project-local config.`

	DescBackup = `Manage git backups — CREATE, RESTORE, DELETE, or LIST. Every write operation auto-creates a backup. Use RESTORE to undo a mutation. Use LIST to see available backups with undoable indicators. DELETE removes a specific backup ref.`

	DescUndo = `Undo the most recent destructive git operation by restoring the latest backup. Shortcut for backup RESTORE without specifying a ref. WHEN to use: immediately after a mistaken amend, merge, rebase, or revert. CONSEQUENCES: resets HEAD to the pre-operation state.`
)

// Review tool
const (
	DescPrReview = `Pre-PR gate: runs tests, detects conflicts, shows diff stats, and checks branch divergence. Call BEFORE creating ANY PR — no exceptions. Returns test results, conflict files with AST-annotated hunks, and divergence info. Use instead of raw git diff + test commands.`
)
