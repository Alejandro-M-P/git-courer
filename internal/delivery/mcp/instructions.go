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