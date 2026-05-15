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
1. PREVIEW → DiffChunker parses AST, groups files by dependency graph, splits into atomic commits (max 12 files). Classifier labels each chunk (feat/fix/refactor/BREAKING). Returns plan with job_id.
2. You review the plan. Show the user the proposed commits and ask "does this look good?".
3. APPLY → executes all commits. Hooks always run. No bypass.
You CANNOT do this with raw git. The chunking, AST analysis, classification — it's all built-in.

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
PREVIEW → DiffChunker parses AST, groups files by dependency graph, splits into atomic commits (max 12 files). Classifier labels each chunk (feat/fix/refactor/BREAKING CHANGE). Returns {job_id, chunks:[{title,description,files}]}.
Review the plan — show the user the proposed commits and ask "does this look good?".
APPLY → executes all commits. Hooks ALWAYS run. No --no-verify bypass.
WHY NOT bash: You CANNOT chunk a diff by dependency graph. You CANNOT classify hunks by semantic type. You CANNOT generate conventional commit messages from structured analysis. The pipeline does all of this — you review and confirm.
ABORT cancels a pending plan. REGENERATE regenerates with optional feedback.`

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
NOTE: HARD requires confirmed=true. Ask the user first — "this will discard all uncommitted changes, sure?". dry_run=true previews which commit you'd land on.
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

const descSync = `🧠 LLM-driven. PUSH (confirmed=true) / PULL (auto-backup).
Returns: structured JSON — you always know what happened.
NOTE: PUSH is IRREVERSIBLE. Use dry_run=true first to preview what would be pushed, show the user the summary, ask "does this look right?", then use confirmed=true to execute.
WHY NOT bash: "git push" is IRREVERSIBLE. If you push breaking changes, you need --force. git-courer requires confirmed=true and offers dry_run preview. PULL creates a backup before merging.
NUDGE: ALWAYS call diff before pushing to review what will go up. ALWAYS call pr-review before creating/updating a PR.`

const descPrReview = `🧠 LLM-driven. Pre-PR analysis: diff + stats + divergence. Call this BEFORE creating or updating ANY PR. No exceptions.
Returns: annotated diff, stat (files added/modified/deleted, lines), and branch divergence (ahead/behind, mergeable status).
WHY NOT bash: raw "git diff main..feature" gives you unstructured text with no summary. git-courer gives you annotated diff + file stats + merge status in one call.
NUDGE: ALWAYS call this before pushing or creating a PR. Show the review summary to the user and ask "ready to open the PR?" before proceeding.`

// --- 👤 Human-Driven ---

const descConfig = `👤 Human-driven. READ config or LIST_MODELS. Read-only, no parameters needed.
Returns: config path, content, and available models (provider + name) in ONE response.`

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