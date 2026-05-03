package mcp

// gitCourerSummary is the global summary of git-courer for ALL MCP clients.
// This text is injected once into the LLM context via server.WithInstructions().
const gitCourerSummary = `git-courer: Local git assistant via MCP. 3 tools. All responses compact JSON.
Tools: git_read (info), git_write (direct), git_write_review (confirmed).
Philosophy: NEVER guess. ALWAYS preview destructive ops. Use git_write_review for commits/releases.
`

// descGitRead describes every subcommand of git_read.
const descGitRead = `Read-only git ops. All JSON, descriptive keys.

COMMANDS AND PARAMETERS:

READ_STATUS        | No args                    | Working tree status (default limit: 100)
READ_DIFF          | path=file(opt), filter=glob(opt), limit=N, offset=N, compact=bool(opt) | Diff unstaged. compact=true for +/- only.
READ_DIFF_STAGED   | path=file(opt), filter=glob(opt), limit=N, offset=N, compact=bool(opt) | Diff staged (--cached)
READ_DIFF_ALL      | path=file(opt), filter=glob(opt), limit=N, offset=N, compact=bool(opt) | Combined staged + unstaged with markers
READ_LOG           | revision=ref(opt), pattern=msg(opt), limit=N, offset=N | Commit history. Smart: bare branch auto-converts to branch..HEAD. pattern applies git log --grep
READ_BRANCHES      | pattern=glob(opt), filter=name(opt) | Local branches
READ_TAGS          | pattern=glob(opt), filter=name(opt) | Tags
CURRENT_BRANCH     | No args                    | Current branch name
IS_REPO            | No args                    | Check if inside git repo
REMOTE_BRANCH_LIST | pattern=glob(opt), filter=name(opt) | Remote branches
REMOTE_TAG_LIST    | pattern=glob(opt), filter=name(opt) | Remote tags
WHAT_CHANGED       | filter="all"|"staged"|"unstaged", llm=bool(opt) | Summary of changes.
READ_DIFF_STAT     | path=file(opt)             | Stats only (insertions/deletions per file)
READ_SEARCH        | pattern=regex(req), context=N(opt), before=N(opt), after=N(opt) | Search using git grep. Returns empty on no match.
BLAME              | path=file(req), limit=N, offset=N | Per-line authorship (paginated)
SHOW               | hash=commit(req)           | Single commit details with stats AND file list
CAT_FILE           | revision=ref(opt), path=file(req) | Read file content at a specific revision
LIST_TREE          | revision=ref(opt), path=dir(opt), recursive=bool | List files in a revision. Auto-handles dirs.
REFLOG             | limit=N, offset=N         | Recover lost commits / dangling refs (paginated)
STASH_LIST         | limit=N, offset=N          | List stashed changes (paginated)
MERGE_BASE         | revision="refA,refB"(req)  | Common ancestor of two branches (comma-separated)
REMOTE_INFO        | No args                    | Returns remote URLs configured (git remote -v)
LIST_BACKUPS       | No args                    | List available git-courer backups.

RESPONSES (semantic keys):
READ_STATUS  -> {"clean":bool, "branch":"branch", "files":[{"path":"path","status":"M|A|D|?","staged":bool}], "staged":N, "modified":N, "untracked":N}
READ_DIFF    -> {"diff":"...","total_lines":N,"lines_shown":N,"next_offset":N,"truncated":bool}
READ_LOG     -> {"commits":[{"hash":"hash","message":"msg","author":"author","date":"date"}], "total":N, "returned":N, "next_offset":N}
READ_BRANCHES -> {"branches":["main","feat/x"], "current":"current"}
WHAT_CHANGED -> {"summary":"summary","files":N,"additions":N,"deletions":N, "mode":"all|staged|unstaged", "llm_used":bool}
READ_DIFF_STAT -> {"files":N, "additions":N, "deletions":N, "file_list":["a.go","b.go"]}
BLAME        -> {"file":"file.go", "lines":[{"line_number":N, "author":"author", "hash":"hash"}], "total":N, "offset":N, "limit":N, "next_off":N}
SHOW         -> {"hash":"hash", "message":"msg", "author":"author", "date":"date", "files":[files], "stats":{"files":N, "additions":N, "deletions":N}}
REFLOG       -> {"operations":[{"index":N, "action":"action", "hash":"hash"}], "total":N, "offset":N, "limit":N, "next":N}
STASH_LIST   -> {"stashes":[{"index":N, "message":"message", "hash":"hash"}], "total":N, "offset":N, "limit":N, "next_offset":N}
MERGE_BASE   -> {"base":"hash", "a":"branchA", "b":"branchB"}
LIST_BACKUPS -> {"backups":[{"ref":"ref", "operation":"op", "created_at":"date"}]}

EXAMPLES:
READ_LOG revision="main" pattern="fix:" -> search "fix:" in main..HEAD
READ_SEARCH pattern="func.*READ" context=2 -> grep with 2 lines context
READ_DIFF path="handlers.go"      -> full sanitized diff for handlers.go
LIST_TREE path="internal"         -> auto-lists directory content
MERGE_BASE revision="main,develop" -> common ancestor
`

const descGitWrite = `Direct write ops

COMMANDS AND PARAMETERS:

ADD                  | paths=file1,file2(req)      | Stage files.
RM                   | paths=file1,file2(req)      | Remove files and stage deletion.
SWITCH               | branch=name(req)            | Switch to branch.
STASH                | message=text(opt)           | Save changes to stash with optional message.
STASH_POP            | No args                     | Restore latest stash.
PUSH                 | remote=name(opt), branch=name(opt) | Push changes to remote.
PULL                 | remote=name(opt), branch=name(opt) | Pull changes from remote.
FETCH                | No args                     | Fetch all remotes.
RESET_SOFT           | commit=ref(req)             | Soft reset (unstage).
RENAME_BRANCH        | name="old:new"(req)         | Rename branch.
BRANCH_CREATE        | name=branch(req)            | Create branch, DO NOT switch.
BRANCH_DELETE        | name=branch(req), force=bool | Delete branch.
REMOTE_BRANCH_DELETE | name="origin/branch"(req)   | Delete remote branch.
REMOTE_TAG_DELETE    | name=tag(req)               | Delete remote tag.
TAG_CREATE           | name=tag(req)               | Create tag.
TAG_DELETE           | name=tag(req)               | Delete local tag.
TAG_PUSH             | name=tag(req)               | Push tag to remote.
TAG_DELETE_REMOTE    | name=tag(req)               | Delete tag from remote.
MERGE                | branch=name(req)            | Merge branch into current.
UNDO                 | No args                     | Reverts the last git_write operation.
PRUNE_BACKUPS        | days=N(opt)                 | Deletes backups older than N days (default 7).

RESPONSE FORMAT:
{"success":true, "operation":"COMMAND", "message":"description"}
Error -> {"status":"error", "command":"COMMAND", "error":"msg", "hint":"fix"}

EXAMPLES:
STASH message="working on feature x" -> stash with custom description
UNDO -> revert last direct write operation
PUSH remote="origin" branch="main" -> explicit push
`

// descGitWriteReview describes the 3-phase confirmed ops.
const descGitWriteReview = `Confirmed write ops with LLM preview. 3-phase protocol REQUIRED.

PHASES:
START      | {OP}_START    | command, instruction | Generate preview of operation
APPLY      | {OP}_APPLY    | command              | Execute after user confirms
ABORT      | {OP}_ABORT    | command              | Cancel operation
REGENERATE | {OP}_REGENERATE | command, feedback  | Regenerate preview with feedback
STATUS     | STATUS        | command              | Check pending review operations
SUMMARY    | SUMMARY       | command              | Get overview of current state

OPERATIONS (OP): COMMIT, RELEASE, BRANCH_CREATE, BRANCH_DELETE

PARAMETERS:
command     = one of phase patterns above (REQUIRED)
instruction = natural language for START (e.g., "commit all changes")
branch      = branch name for branch ops (optional)
preview     = bool: show preview before executing (default from config)
feedback    = string: feedback for REGENERATE (e.g., "make messages shorter")

3-PHASE PROTOCOL:
1. Call {OP}_START with instruction -> returns preview
2. Show ENTIRE preview to user BEFORE asking confirmation
3. User confirms -> call {OP}_APPLY
   User rejects  -> call {OP}_ABORT or {OP}_REGENERATE with feedback

RULES:
- NEVER call APPLY without explicit user confirmation
- ALWAYS copy-paste the FULL preview, do NOT summarize or truncate
- COMMIT uses conventional commits via Ollama (type, scope, description, body)
- RELEASE creates GitHub release with auto-generated changelog

EXAMPLES:
COMMIT_START instruction="commit all changes" -> returns preview
COMMIT_APPLY -> executes the commit
RELEASE_START instruction="release v1.2.0" -> generates release notes and tag
`
