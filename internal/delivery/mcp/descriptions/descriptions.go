// Package descriptions is the single source of truth for all MCP tool descriptions
// and the server instructions shown to LLM agents.
// Registration files import from here to avoid duplicating strings across packages.
package descriptions

// GitCourerSummary is the system instructions shown to LLM agents on MCP initialize.
// First 512 chars must be self-contained for clients that truncate (e.g., Codex).
const GitCourerSummary = `= git-courer — git for agents, NOT a git wrapper =

git-courer is NOT a wrapper around git. Some tools do things raw git CANNOT.
Others are structured replacements that return JSON instead of human text.

═══ UNIQUE — git CANNOT do this ═══
  diff         AST-labeled diffs: [NEW_FUNC] [MOD_SIG ⚠BREAKING] [DEPS] [DEL]
  commit       LLM pipeline: chunks by dependency graph, writes commit messages
  status       5+ git calls combined into one JSON response
  pr-review    Tests + conflicts + diffs + divergence in a single call
  backup       Git has NO undo. Auto-backup before every mutation.

═══ REPLACEMENTS — structured version of git X ═══
  history      Structured git log / reflog / blame     stage     Structured git add / restore
  branch       Structured git branch / switch           integrate Structured git merge / rebase / cherry-pick
  stash        Structured git stash                     rewrite   Structured git amend / revert / reset
  sync         Structured git push / pull / fetch

COST GUIDE — plan your token budget:
  Fast (call freely):    status, diff, stage, stash, history, backup
  Medium (I/O cost):     sync (PULL/FETCH), branch, rewrite (AMEND/REVERT/SOFT)
  Heavy (plan around):   commit (PREVIEW + LLM collaboration), integrate
                          (MERGE/UPDATE resolution cycles), pr-review (runs tests),
                          sync/PUSH (irreversible), rewrite/HARD (requires ✓)

ERRORS YOU WILL ENCOUNTER — handle them:
  status          → conflicted_files present → resolve before mutations
  diff            → empty result = no changes (not an error)
  commit          → hooks reject → create new commit (do NOT amend the failure)
  sync/PUSH       → rejected → fetch + diff first
  integrate       → returns conflicted_files → read + diff + stage + CONTINUE
  stage/clean     → blocked without confirmed=true
  branch/delete   → blocked without confirmed=true

GOLDEN RULES — save tokens and prevent mistakes:
   0. On session start → session start (create an isolated worktree before any work).
      On session end → session finish (merge + cleanup). Both are MANDATORY.
   1. BEFORE any mutation → status (know the repo state)
   2. BEFORE push → diff + review
   3. BEFORE PR → pr-review (all checks in one call)

TOOLS: status diff commit branch stage stash history sync pr-review backup rewrite integrate session

Note: ` + "`session`" + ` supports start (create isolated worktree+branch), finish
(merge + cleanup), status (read session state), and discard (remove without
merging, requires confirmed=true).`

// ─── UNIQUE tools (git CANNOT do this) ─────────────────────────────────────

const (
	// DescStatus: COMPLETE repo state in one call — 5+ git commands worth of data.
	// Raw git status only shows staged/unstaged. This adds ahead/behind, stash,
	// conflicts, last commit, in-progress operations, user config, and remotes.
	DescStatus = `IMPOSSIBLE with raw git. Returns COMPLETE repo state in ONE call — branch, ahead/behind, staged, unstaged, untracked, conflicted files, stash count, in-progress operations, last commit, user.name, user.email, test_command, and remotes. Raw git status needs 5+ separate commands for this data. Call BEFORE any write operation.`

	// DescDiff: AST-annotated diffs with semantic labels.
	// Raw git diff shows text lines. This shows what CHANGED (new function, modified signature, deleted code).
	DescDiff = `IMPOSSIBLE with raw git. Returns AST-labeled diffs — hunks marked [NEW_FUNC], [MOD_SIG ⚠BREAKING], [DEPS], [DEL]. Raw git diff only shows text lines. This tells you WHAT changed semantically. Paginated — no pager hangs. Call before push or PR. Use 'branch' param to compare against another branch directly by name (symmetric diff; dot prefixes like '..' or '...' are not supported).`

	// DescCommit: LLM-powered commit pipeline. Two LLMs collaborate to chunk
	// changes by dependency graph, produce atomic commits, and write human messages.
	// Raw git commit is a single monolithic blob with a manual message.
	DescCommit = `IMPOSSIBLE with raw git. LLM-powered 3-phase commit pipeline: PREVIEW parses AST and groups files by dependency graph into atomic commits, APPLY executes them. PREVIEW requires a 'why' parameter — the REAL reason for the change (problem/symptom/limitation), NOT what the code does. APPLY supports two paths: 1) With job_id: creates a single atomic commit from the PREVIEW snapshot via plumbing (CommitTree + UpdateRef), 2) Without job_id: executes the pending plan from ConfirmStore. APPLY accepts an optional 'type' parameter to override the commit type prefix. Workflow: 1) PREVIEW → get plan, 2) Review with user, 3) APPLY. push_after:true on APPLY pushes to remote. IMPORTANT: if PREVIEW takes longer than 45s it returns {'status':'processing','job_id':'...'} — the goroutine continues in background. DO NOT wait idly: do other work (read files, explore, etc.) and poll STATUS with the job_id when convenient.`

	// DescPrReview: Pre-PR gate that runs tests, detects conflicts, shows diff
	// stats, and checks branch divergence — all in one call.
	// Raw git needs 4+ separate commands for the same information.
	DescPrReview = `IMPOSSIBLE with raw git. Pre-PR gate: runs tests, detects conflicts, shows diff stats, and checks branch divergence in ONE call. Raw git needs 4+ separate commands. Call BEFORE creating ANY PR — no exceptions. Returns test results, conflict files with AST-annotated hunks, and divergence info.`

	// DescBackup: Auto-backup before every destructive operation.
	// Raw git has NO undo mechanism — data loss is permanent.
	DescBackup = `Raw git has NO undo. Auto-backup before every mutation — RESTORE or LIST backups. Use RESTORE to undo a mistaken amend/merge/rebase/revert. Use LIST to see available backups.`
)

// ─── REPLACEMENTS (structured version of git X) ────────────────────────────

const (
	DescBranch = `Structured git branch / git switch / git checkout. Branch lifecycle — CREATE, SWITCH, DELETE, RENAME, LIST. Lifecycle safety, auto-stash, structured JSON output. DELETE requires confirmed=true. SWITCH auto-stashes dirty tree. DELETE with filter=REMOTE deletes remote branch. Do NOT use for merging — use integrate instead.`

	DescStage = `Structured git add / git restore / git clean. Stage, unstage, restore, or clean files with structured JSON, safety gates, and binary-file interception. CLEAN requires confirmed=true. Do NOT use for committing — use commit instead.`

	DescStash = `Structured git stash. Save, restore, or inspect stashed changes. SAVE stores working tree changes. POP restores them. SHOW previews stash diff as structured JSON — no parsing git stash list text. Use before switching branches with dirty tree.`

	DescSync = `Structured git push / git pull / git fetch. PUSH is IRREVERSIBLE and requires confirmed=true. PULL and FETCH create a backup before executing. Use FETCH to check remote changes without merging (safer than PULL). When branch is specified, pushes/pulls only that branch. Always call diff before pushing.`

	DescHistory = `Structured git log / git reflog / git blame. Show commit history (LOG), reflog (REFLOG), or line-by-line attribution (BLAME) with pagination and filtering. Structured JSON — no pager hangs, no unstructured text. Do NOT use raw git log. BLAME requires target_paths.`

	DescRewrite = `Structured git history rewrite. AMEND (fix last commit), REVERT (undo a commit), SOFT (move HEAD only), HARD (discard everything). Creates backup BEFORE executing; undo with backup RESTORE. HARD requires confirmed=true. Do NOT use AMEND for new changes — use commit instead.`

	DescIntegrate = `Structured git integration. MERGE (merge a branch), UPDATE (rebase onto a branch), PICK (cherry-pick a commit), CONTINUE (continue after resolving conflicts), ABORT (abort in-progress operation). Structured conflict detection — returns conflict file list instead of raw >>>>>> text. Creates backup BEFORE executing; undo with backup RESTORE.`

	// DescSession: Isolated git worktree + branch per agent for parallel work.
	// start creates a session; finish merges + cleans up; status reads state;
	// discard removes a session without merging.
	DescSession = `Isolated git worktree + branch per agent for parallel work. ` + "`start`" + ` creates a ` + "`{id}`" + ` branch (atomic ` + "`git update-ref`" + `) and a linked worktree at ` + "`../git-courer-worktrees/{id}/`" + `, persisting session metadata. ` + "`finish`" + ` loads a session, runs preview validation (uncommitted changes, test command, dry-run merge conflict), merges the session branch into its base branch, then cleans up the worktree + branch. ` + "`status`" + ` returns the current session state (active | finished | cleanup_failed). ` + "`discard`" + ` removes a session without merging — requires ` + "`confirmed=true`" + `. The ` + "`agent`" + ` and ` + "`goal`" + ` parameters are required for ` + "`start`" + `. ` + "`branch`" + ` is optional for ` + "`start`" + ` — when provided, uses that exact branch name (fails if exists); when omitted, derives from goal slug. ` + "`session_id`" + ` is required for finish/status/discard.`
)
