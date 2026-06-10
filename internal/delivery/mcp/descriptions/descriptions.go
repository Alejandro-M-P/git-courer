// Package descriptions is the single source of truth for all MCP tool descriptions
// and the server instructions shown to LLM agents.
// Registration files import from here to avoid duplicating strings across packages.
package descriptions

// GitCourerSummary is the system instructions shown to LLM agents on MCP initialize.
// First 512 chars must be self-contained for clients that truncate (e.g., Codex).
// The summary is split into two sections:
//   - UNIQUE: tools that do things raw git CANNOT do. No alternative exists.
//   - REPLACEMENTS: structured versions of familiar git commands.
const GitCourerSummary = `= git-courer — git for agents, NOT a git wrapper =

git-courer is NOT a wrapper around git. Some tools do things raw git CANNOT.
Others are structured replacements that return JSON instead of human text.

═══ UNIQUE — git CANNOT do this ═══
  diff         AST-labeled diffs: [NEW_FUNC] [MOD_SIG ⚠BREAKING] [DEPS] [DEL]
  commit       LLM pipeline: chunks by dependency graph, writes commit messages
  status       5+ git calls combined into one JSON response
  pr-review    Tests + conflicts + diffs + divergence in a single call
  backup/undo  Git has NO undo. Auto-backup before every mutation.

═══ REPLACEMENTS — structured version of git X ═══
  history      Structured git log / reflog          stage     Structured git add / restore
  branch       Structured git branch / switch       merge     Structured git merge
  rebase       Structured git rebase                sync      Structured git push / pull / fetch
  stash        Structured git stash                 blame     Structured git blame
  reset        Structured git reset                 revert    Structured git revert
  amend        Structured git commit --amend         tag       Structured git tag
  config       Structured git config                remotes   Structured git remote

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

// ─── UNIQUE tools (git CANNOT do this) ─────────────────────────────────────

const (
	// DescStatus: COMPLETE repo state in one call — 5+ git commands worth of data.
	// Raw git status only shows staged/unstaged. This adds ahead/behind, stash,
	// conflicts, last commit, in-progress operations.
	DescStatus = `IMPOSSIBLE with raw git. Returns COMPLETE repo state in ONE call — branch, ahead/behind, staged, unstaged, untracked, conflicted files, stash count, in-progress operations, last commit. Raw git status needs 5+ separate commands for this data. Call BEFORE any write operation.`

	// DescDiff: AST-annotated diffs with semantic labels.
	// Raw git diff shows text lines. This shows what CHANGED (new function, modified signature, deleted code).
	DescDiff = `IMPOSSIBLE with raw git. Returns AST-labeled diffs — hunks marked [NEW_FUNC], [MOD_SIG ⚠BREAKING], [DEPS], [DEL]. Raw git diff only shows text lines. This tells you WHAT changed semantically. Paginated — no pager hangs. Call before push or PR.`

	// DescCommit: LLM-powered commit pipeline. Two LLMs collaborate to chunk
	// changes by dependency graph, produce atomic commits, and write human messages.
	// Raw git commit is a single monolithic blob with a manual message.
	DescCommit = `IMPOSSIBLE with raw git. LLM-powered 3-phase commit pipeline: PREVIEW parses AST and groups files by dependency graph into atomic commits, APPLY executes them. PREVIEW accepts a 'why' parameter to justify changes. APPLY supports two paths: 1) With job_id: creates a single atomic commit from the PREVIEW tree snapshot via plumbing (CommitTree + UpdateRef), 2) Without job_id: executes the pending plan from ConfirmStore. APPLY accepts an optional 'type' parameter to override the commit type prefix (e.g., type: 'fix' replaces 'feat:' with 'fix:'). Useful when the AST classifier infers a different type than the intended change. Workflow: 1) PREVIEW → get plan, 2) Review with user, 3) APPLY. push_after:true on APPLY pushes to remote. If PREVIEW returns 'processing', poll STATUS with job_id. Do NOT pass target_paths or any file-selection params — the pipeline commits everything from the PREVIEW snapshot.`

	// DescPrReview: Pre-PR gate that runs tests, detects conflicts, shows diff
	// stats, and checks branch divergence — all in one call.
	// Raw git needs 4+ separate commands for the same information.
	DescPrReview = `IMPOSSIBLE with raw git. Pre-PR gate: runs tests, detects conflicts, shows diff stats, and checks branch divergence in ONE call. Raw git needs 4+ separate commands. Call BEFORE creating ANY PR — no exceptions. Returns test results, conflict files with AST-annotated hunks, and divergence info.`

	// DescBackup: Auto-backup before every destructive operation.
	// Raw git has NO undo mechanism — data loss is permanent.
	DescBackup = `Raw git has NO undo. Auto-backup before every mutation — CREATE, RESTORE, DELETE, or LIST backups. Use RESTORE to undo a mistaken amend/merge/rebase/revert. Use LIST to see available backups. DELETE removes a backup ref.`

	// DescUndo: Shortcut for backup RESTORE — undoes the last destructive operation.
	// Raw git has NO equivalent.
	DescUndo = `Raw git has NO undo. Shortcut for backup RESTORE — undoes the most recent destructive git operation (amend, merge, rebase, revert). CONSEQUENCES: resets HEAD to the pre-operation state. Use after a mistake.`
)

// ─── REPLACEMENTS (structured version of git X) ────────────────────────────

// Core replacement tools
const (
	DescAmend = `Structured git commit --amend. Fix the last commit — change message, add files, or both. Creates backup BEFORE executing; undo with backup RESTORE. WITHOUT confirmed=true, the operation is BLOCKED. Do NOT use for new changes (use commit instead).`

	DescRevert = `Structured git revert. Create a new commit that undoes a previous commit. Creates backup BEFORE executing; undo with backup RESTORE. WITHOUT confirmed=true, the operation is BLOCKED. Use dry_run=true first to preview.`

	DescCommitJobs = `List active commit pipeline jobs — their status, commit message, and tree hash. Read-only tool for inspecting background commit pipeline state.`
)

// Branching replacement tools
const (
	DescBranch = `Structured git branch / git switch / git checkout. Branch lifecycle — CREATE, DELETE, RENAME, REMOTE_DELETE, SET_UPSTREAM, UNSET_UPSTREAM, SWITCH. Lifecycle safety, auto-stash, structured JSON output. DELETE and REMOTE_DELETE require confirmed=true. SWITCH auto-stashes dirty tree. Do NOT use for merging — use merge tool instead.`

	DescMerge = `Structured git merge. Merge a branch with structured conflict detection — returns conflict file list instead of raw >>>>>> text to parse. After successful merge, delete_source removes source branch, push_after pushes to remote. All composition steps only run if merge succeeds without conflicts.`

	DescRebase = `Structured git rebase. Rebase current branch with the same structured conflict output as merge. After resolving conflicts, call rebase with continue=true. Skip a conflicting commit with skip=true. Use --onto to transplant to a different base. Do NOT use for preserving merge history — use merge instead.`

	DescTag = `Structured git tag. Tag lifecycle — CREATE annotated tags, DELETE tags locally or remotely, PUSH tags. DELETE operations require confirmation.`

	DescCherryPick = `Structured git cherry-pick. Apply a specific commit onto the current branch. Creates backup; undo with backup RESTORE.`
)

// Stage replacement tools
const (
	DescStage = `Structured git add / git restore / git clean. Stage, unstage, restore, or clean files with structured JSON, safety gates, and binary-file interception. CLEAN requires confirmed=true. Do NOT use for committing — use commit instead.`

	DescReset = `Structured git reset. Undo commits at different safety levels. SOFT (safest — moves HEAD only). MIXED (unstages too). HARD (discards everything — requires confirmed=true). Use dry_run=true to preview. Do NOT use for amending — use amend instead.`

	DescStash = `Structured git stash. Save, restore, or inspect stashed changes. SAVE stores working tree changes. POP restores them. SHOW previews stash diff as structured JSON — no parsing git stash list text. Use before switching branches with dirty tree.`
)

// Sync replacement tools
const (
	DescSync = `Structured git push / git pull / git fetch. PUSH is IRREVERSIBLE and requires confirmed=true. PULL and FETCH create a backup before executing. Use FETCH to check remote changes without merging (safer than PULL). When branch is specified, pushes/pulls only that branch. Always call diff before pushing.`

	DescRemotes = `Structured git remote. Manage remote repositories — ADD a new remote URL or REMOVE an existing one. REMOVE requires confirmed=true and is NOT undoable via backup.`
)

// History replacement tools
const (
	DescHistory = `Structured git log / git reflog. Show commit history (LOG) or reflog (REFLOG) with pagination and filtering. Structured JSON — no pager hangs, no unstructured text. Do NOT use raw git log.`

	DescBlame = `Structured git blame. Line-by-line attribution for a specific file — who changed what and when. Returns JSON per line. Do NOT use raw git blame — structured data, no text parsing.`
)

// Utility replacement tools
const (
	DescConfig = `Structured git config. Read or update project configuration. GET returns all config (global and project-level). SET_TEST_COMMAND saves the project test command. SET_USER_NAME, SET_USER_EMAIL, and SET_SIGNING_KEY persist git identity settings.`
)
