package mcp

const gitCourerSummary = `= git-courer: git for LLMs, not for bash =

WHY NOT JUST USE GIT IN BASH?
You are an LLM. You cannot:
- Parse unstructured pager output reliably (git log, git diff, git status all hang or spew walls of text)
- Chunk a diff into atomic commits by dependency graph (you'd need tree-sitter + AST analysis)
- Classify hunks by semantic type (feat/fix/refactor/test) without external tools
- Know which files relate to each other before committing (you'd need a file relationship DB)
- Detect binary files before staging (git add doesn't warn you)
- Recover from a bad merge or rebase without structured conflict data
- Know if a hook will fail before committing (no dry-run in raw git)
- Generate conventional commit messages from structured analysis (you'd need DiffChunker + Classifier)
git-courer solves ALL of this. Every tool returns structured JSON. Every mutation has auto-backup. Every conflict returns {status,conflicted_files,hint}. No pagers. No parsing.

DRIVER CLASSIFICATION:
🧠 LLM-driven — YOU call these directly. They return JSON and handle the hard parts.
   commit, amend, revert, status, diff, history, merge, rebase, branch, cherry_pick, stage, reset, stash, sync, blame, pr-review
👤 Human-driven — simpler tools, designed for a human to invoke directly.
   backup, config, release, remotes, tag

SAFETY MODEL:
- Every mutation auto-creates a backup. Undo via backup RESTORE.
- Destructive ops (push, hard reset, branch delete, tag delete) require confirmed=true — ask the user "does this look right?" before proceeding.
- Hooks ALWAYS run. No --no-verify bypass exists.
- dry_run=true available on amend, revert, reset HARD, sync PUSH — use it to preview before executing.

WORKFLOW NUDGES (call these automatically, don't wait to be asked):
- BEFORE any write → call status to check for dirty tree, conflicts, or in-progress work.
- BEFORE pushing → call diff to review what will go up. Show the user the summary and ask "does this look right?"
- BEFORE creating/updating a PR → call pr-review FIRST. Always. No exceptions.
- AFTER merge/rebase conflict → call diff to see what needs resolving, then stage the resolved files.

COMMIT PIPELINE (the biggest LLM advantage):
commit is NOT "git add + git commit". It has a 3-phase pipeline that runs 100% locally:
1. PREVIEW → DiffChunker parses AST, groups files by dependency graph, splits into atomic commits (max 12 files). Classifier labels each chunk (feat/fix/refactor/BREAKING).
   - FAST: Returns {status:"pending", job_id, plan} directly.
   - SLOW (>30s): Returns {status:"processing", job_id}. Poll STATUS with job_id to get the plan.
2. Review the plan. Show the user the proposed commits and ask "does this look good?".
3. APPLY → executes all commits. Hooks always run. No bypass.
You CANNOT do this with raw git. The chunking, AST analysis, classification — it's all built-in.
If PREVIEW returns "processing", call STATUS with the job_id until it returns "done" or "failed". ABORT cancels a job. REGENERATE regenerates with optional feedback.

AVAILABLE TOOLS:
status, diff, commit, amend, revert, branch, merge, rebase, cherry_pick, stage, reset, stash, history, blame, sync, pr-review, config, backup, release, remotes, tag`

const descStatus = `🧠 LLM-driven. Returns COMPLETE repo state in ONE call. No discriminator, no sub-commands.
Returns: {branch, ahead, behind, staged[], unstaged[], untracked[], conflicted_files[], stash_count, in_progress, last_commit{hash,message,date}}.
WHY NOT bash: "git status" gives unstructured text. You'd need 5+ calls to get the same data. One status call replaces all of them.
NUDGE: Call this BEFORE any write operation — always know the repo state before mutating.`

const descDiff = `🧠 LLM-driven. Annotated diff with AST labels in @@ headers.
Returns: hunks with [NEW_FUNC: name], [MOD_SIG: name ⚠BREAKING], [DEPS: imports], [DEL: name] — you see WHAT changed at symbol level, not raw lines.
WHY NOT bash: "git diff" is unstructured text. You'd have to guess whether a hunk is a new function, a breaking signature change, or just imports. git-courer annotates each hunk via tree-sitter. Paginated — no pager hangs.
NUDGE: Call this before pushing or creating a PR to review what will go up.`

const descCommit = `🧠 LLM-driven. The MOST IMPORTANT tool. 3-phase pipeline, 100% local.
PREVIEW → DiffChunker parses AST, groups files by dependency graph, splits into atomic commits (max 12 files). Classifier labels each chunk (feat/fix/refactor/BREAKING CHANGE). Returns {status:"pending"|"processing", job_id, plan}.
  - status:"pending" → plan ready, proceed to APPLY.
  - status:"processing" → plan is being generated (slow LLM). Poll STATUS with job_id until you get status:"done" or "failed".
APPLY → executes all commits. Hooks ALWAYS run. No --no-verify bypass.
ABORT → cancels a running or pending job.
REGENERATE → regenerates plan with optional feedback. May also return "processing" — poll STATUS.
STATUS → polls job state. Returns {status, progress, plan} when done.
WHY NOT bash: You CANNOT chunk a diff by dependency graph. You CANNOT classify hunks by semantic type. You CANNOT generate conventional commit messages from structured analysis. The pipeline does all of this — you review and confirm.`

const descAmend = `🧠 LLM-driven. Fix the last commit SAFELY.
Returns: {status:"ok"} or error with hint. Creates backup BEFORE mutating.
WHY NOT bash: "git commit --amend" has no safety net. Wrong message? Wrong files? Reflog is your only recovery. git-courer creates a backup first — backup RESTORE undoes it cleanly.
NUDGE: Use dry_run=true to preview what would change, then ask the user "does this look right?" before executing.`

const descRevert = `🧠 LLM-driven. Revert a commit with automatic backup.
Returns: {status:"ok"} or structured error with hint. Backup created BEFORE executing.
WHY NOT bash: "git revert" drops you into conflict state with no structured output. You can't detect which files conflicted without parsing text. git-courer returns clean JSON.
NUDGE: Use dry_run=true first to show the user what will be reverted, then confirm.`

const descBranch = `🧠 LLM-driven. Branch lifecycle with safety gates.
Returns: {status:"ok"} or error. SWITCH auto-stashes dirty working tree.
WHY NOT bash: "git branch" can delete a branch with no recovery. Switching with dirty tree fails or forces stash. git-courer auto-stashes on switch — no manual stash needed.
NOTE: DELETE and REMOTE_DELETE require confirmed=true. Ask the user "are you sure?" before proceeding — these are NOT undoable via backup.`

const descMerge = `🧠 LLM-driven. Merge with STRUCTURED conflict detection.
Returns: {status:"ok"} or {status:"conflict",conflicted_files:["src/main.go",...],hint:"Resolve..."}.
WHY NOT bash: "git merge" on conflict dumps unstructured text. You can't tell which files conflicted without parsing. git-courer gives you the file list, no parsing needed.
COMPOSITION: Use delete_source:true, push_after:true, and new_branch:"name" to clean up and pivot in one call after a successful merge.
NUDGE: After resolving conflicts, call diff to verify, then stage the resolved files, then merge continue=true.`

const descRebase = `🧠 LLM-driven. Rebase with the SAME structured contract as merge.
Returns: {status:"ok"} or {status:"conflict",conflicted_files:[...]} — identical shape to merge.
WHY NOT bash: Same problem as merge — raw git gives unstructured conflict output. git-courer gives you the file list.
NUDGE: After resolving conflicts, call diff to verify, then stage the resolved files, then rebase continue=true.`

const descCherryPick = `🧠 LLM-driven. Cherry-pick a commit onto the current branch.
Returns: {status:"ok"} or {status:"conflict",conflicted_files:[...]} on conflicts.
WHY NOT bash: "git cherry-pick" drops you into unstructured conflict state. git-courer returns structured data so you know exactly what happened.`

const descStage = `🧠 LLM-driven. Stage/unstage with binary file SAFETY.
Returns: structured confirmation of what was staged/unstaged. Binary files are intercepted — you get warned BEFORE staging.
WHY NOT bash: "git add" silently stages binaries. git-courer catches them first. Auto-backup before every mutation.`

const descReset = `🧠 LLM-driven. Undo commits at different safety levels.
Returns: {status:"ok"} with backup info. SOFT moves HEAD only (safest). MIXED moves HEAD and unstages. HARD is destructive — discards everything.
NOTE: HARD requires confirmed=true. Ask the user first — "this will discard all uncommitted changes, sure?". dry_run=true previews which commit you' land on.
WHY NOT bash: "git reset --hard" is permanent and git doesn't warn you. git-courer HARD requires confirmation and creates a backup first.`

const descStash = `🧠 LLM-driven. Temporary saves you can inspect before restoring.
Returns: structured stash info. SHOW lets you see what's inside without popping.
WHY NOT bash: "git stash" is a pile of unnamed entries. git-courer SHOW lets you inspect before restoring. No DROP/CLEAR — stashes are auto-managed, you can't accidentally nuke them.`

const descHistory = `🧠 LLM-driven. LOG and REFLOG with pagination, no pager hangs.
Returns: structured JSON. Paginated with offset/limit.
WHY NOT bash: "git log" hangs on large repos. git-courer never hangs — paginated JSON with offset/limit. No unstructured text to parse.`

const descBlame = `🧠 LLM-driven. Line-by-line attribution for a specific file.
Returns: structured JSON with author, line, and commit per line.
WHY NOT bash: "git blame" spews unstructured text. git-courer returns JSON — no parsing needed.`

const descSync = `🧠 LLM-driven. PUSH (confirmed=true) / PULL (auto-backup) / AUTO (full sync).
Returns: structured JSON — you always know what happened.
NOTE: PUSH is IRREVERSIBLE. Use dry_run=true first to preview what would be pushed, show the user the summary, ask "does this look right?", then use confirmed=true to execute.
COMMAND AUTO: Fetches, Pulls, and Pushes in one turn. Use for fast-forwarding your state.
WHY NOT bash: "git push" is IRREVERSIBLE. If you push breaking changes, you need --force. git-courer requires confirmed=true and offers dry_run preview. PULL creates a backup before merging.
NUDGE: ALWAYS call diff before pushing to review what will go up. ALWAYS call pr-review before creating/updating a PR.`

const descPrReview = `🧠 LLM-driven. Pre-PR gate: tests + conflict detection + diff stats + divergence. Call BEFORE creating ANY PR. No exceptions.
Returns JSON with 5 possible states:
- no_test_command → first run hint: configure with config SET_TEST_COMMAND "make test-ci"
- test_fail → ONLY failing tests shown with truncated output (not the full log)
- conflict → conflicted files + AST-annotated conflict hunks ([NEW_FUNC], [MOD_SIG ⚠BREAKING], etc.)
- test_ok → all green, ready for PR
- error → unexpected failure (branch not found, etc.)
Also returns: branch divergence (ahead/behind/mergeable), diff stats (files/additions/deletions).
Param: to (default "main") — target branch.
WHY NOT bash: raw "git diff main..feature" + "go test" gives you unstructured text you have to parse. git-courer gives you structured analysis in one call — you know if the PR is safe to open.
NUDGE: ALWAYS call this before pushing or creating a PR. No exceptions.`

// --- 👤 Human-Driven ---

const descConfig = `👤 Human-driven. READ returns config + models in one call. LIST_MODELS shows available Ollama/cloud models.
SET_TEST_COMMAND saves the project's test command to .git-courer/config.json — this is per-project, committable, shared by the team.
Returns: config path, content, models (provider + name) and/or test_command confirmation.`

const descBackup = `👤 Human-driven. Manage git backups. Every write operation auto-creates one.
CREATE (manual backup), DELETE (confirmed=true), RESTORE (undo last mutation), LIST (show available backups with undoable indicator).
20 max backups. Oldest auto-pruned.`

const descRelease = `👤 Human-driven. Semver releases from conventional commits.
START → calculates version bump from commit history. Returns proposed version with changelog. Show the user and ask "does this look right?".
APPLY → creates tag + changelog + push. ABORT → cancel.
dry_run=true previews without creating anything.`

const descRemotes = `👤 Human-driven. ADD or REMOVE remote URLs.
ADD (remote_name + url). REMOVE (remote_name, confirmed=true — NOT undoable via backup).`

const descTag = `👤 Human-driven. CREATE annotated tags or DELETE them.
CREATE (annotated tag with message). DELETE (confirmed=true — NOT undoable via backup).`