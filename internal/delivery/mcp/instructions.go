package mcp

// gitCourerSummary is injected into ALL clients and sub-agents via server.WithInstructions().
// This is the FIRST thing every agent (main or sub) reads. Keep ultra-compact.
const gitCourerSummary = `git-courer: local git MCP assistant. 3 tools, compact JSON.
ALL agents (main and sub) MUST use these tools — never raw bash git commands.

TOOLS:
- git_read       → info: status, diff, log, stash, blame, search, forensic (orphans), jobs
- git_write      → direct ops: add, stash (apply/drop/clear), push, pull (read tool description)
- git_write_review → LLM-confirmed: commit, release, branch, merge (3-phase protocol, read tool description)

BACKGROUND JOBS (auto-triggered, no config needed):
  Release changelog >2 chunks OR any _START op taking >45s → returns {"status":"background","job_id":"..."}
  Progress notifications arrive automatically: "⚙️ 3/10 chunks [job:ID]"
  Completion notification: "✅ done [job:ID] → call {OP}_APPLY"
  Poll anytime: git_read command=JOB_RESULT arg=<job_id>

RULES (apply to ALL agents):
- NEVER guess git state — always read first
- NEVER call git_write_review APPLY without user confirmation (exception: background op done notification)
- ALWAYS use git_write_review for commits and releases
- Read each tool's description before first use — commands and responses are documented there
`

// descGitRead describes every subcommand of git_read.
const descGitRead = `Read-only git ops. All JSON.

COMMANDS:

READ_STATUS        | No args                                        | Working tree status
READ_DIFF          | arg=file(opt), filter=glob(opt), limit=N, offset=N, compact=bool | Unstaged diff
READ_DIFF_STAGED   | arg=file(opt), filter=glob(opt), limit=N, offset=N, compact=bool | Staged diff
READ_DIFF_ALL      | arg=file(opt), filter=glob(opt), limit=N, offset=N, compact=bool | Staged+unstaged
READ_DIFF_STAT     | arg=file(opt)                                  | Stats only (insertions/deletions)
READ_LOG           | revision=ref(opt), pattern=msg(opt), limit=N, offset=N | Commit history
READ_BRANCHES      | pattern=glob(opt), filter=name(opt)            | Local branches
READ_TAGS          | pattern=glob(opt), filter=name(opt)            | Tags
CURRENT_BRANCH     | No args                                        | Current branch name
IS_REPO            | No args                                        | Check if inside git repo
REMOTE_BRANCH_LIST | filter=name(opt)                               | Remote branches
REMOTE_TAG_LIST    | filter=name(opt)                               | Remote tags
REMOTE_INFO        | No args                                        | Remote URLs (git remote -v)
WHAT_CHANGED       | filter="all"|"staged"|"unstaged", llm=bool(opt) | Summary of changes
READ_SEARCH        | arg=pattern(req), context=N, before=N, after=N, filter=file | git grep
BLAME              | arg=file(req), limit=N, offset=N               | Per-line authorship
SHOW               | arg=hash(req)                                  | Single commit details+stats
CAT_FILE           | revision=ref(opt), path=file(req)              | File content at revision
LIST_TREE          | revision=ref(opt), path=dir(opt), recursive=bool | Files in revision
REFLOG             | limit=N, offset=N                              | Reflog (recover lost commits)
READ_ORPHANS       | hours=N(opt,default=1)                         | Find commits not reachable from any branch
STASH_LIST         | limit=N, offset=N                              | List stash entries
STASH_DIFF         | arg=index(opt,default=0)                       | Diff of a stash entry (e.g. arg=1)
MERGE_BASE         | arg="refA,refB"(req)                           | Common ancestor of two branches
LIST_BACKUPS       | No args                                        | List git-courer backups
JOB_RESULT         | arg=job_id(req)                                | Get result of a background job
READ_CONFIG        | No args                                        | View current git-courer config (includes provider/backend)
LIST_MODELS        | No args                                        | List available LLM models

RESPONSES:
READ_STATUS  -> {"clean":bool,"branch":"b","files":[{"path":"p","status":"M|A|D|?","staged":bool}],"staged":N,"modified":N,"untracked":N}
READ_DIFF    -> {"diff":"...","total_lines":N,"lines_shown":N,"next_offset":N,"truncated":bool}
READ_LOG     -> {"commits":[{"hash":"h","message":"m","author":"a","date":"d"}],"total":N,"returned":N,"next_offset":N}
READ_ORPHANS -> {"orphans":[{"hash":"h","message":"m","date":"d"}],"total":N,"returned":N,"next_offset":N}
STASH_LIST   -> {"stashes":[{"index":N,"message":"m","hash":"h"}],"total":N,"offset":N,"limit":N,"next_offset":N}
STASH_DIFF   -> {"diff":"...","total_lines":N,"lines_shown":N,"next_offset":N,"truncated":bool}
MERGE_BASE   -> {"base":"hash","a":"branchA","b":"branchB"}
LIST_BACKUPS -> {"backups":[{"ref":"ref","operation":"op","created_at":"date"}]}
JOB_RESULT   -> {"job_id":"id","op":"op","status":"running|done|failed","progress":"3/10 chunks","result":"...","elapsed":"23s"}
READ_CONFIG  -> {"config_path":"...","content":{...}}
LIST_MODELS  -> {"provider":"...","models":["m1", "m2"]}

EXAMPLES:
READ_LOG revision="main" pattern="fix:"  -> commits matching "fix:" in main..HEAD
READ_ORPHANS hours=2                     -> find orphan commits from the last 2 hours
JOB_RESULT arg=release-changelog-123456  -> check background job result
`

const descGitWrite = `Direct write ops (no LLM). Auto-backed-up (use UNDO to revert).

COMMANDS:

ADD                  | paths=file1,file2(req)          | Stage files
RM                   | paths=file1,file2(req)          | Remove files and stage deletion
SWITCH               | branch=name(req)                | Switch to branch
STASH                | message=text(opt)               | Save changes to stash
STASH_POP            | No args                         | Restore latest stash
STASH_APPLY          | arg=index(opt,default=latest)   | Apply stash entry, keep in list
STASH_DROP           | arg=index(req)                  | Delete specific stash entry (e.g. arg=0)
STASH_CLEAR          | No args                         | Delete ALL stash entries
PUSH                 | remote=name(opt), branch=name(opt) | Push to remote
PULL                 | remote=name(opt), branch=name(opt) | Pull from remote
FETCH                | No args                         | Fetch all remotes
RESET_SOFT           | commit=ref(req)                 | Soft reset (unstage)
RENAME_BRANCH        | name="old:new"(req)             | Rename branch
BRANCH_CREATE        | name=branch(req)                | Create branch (no switch)
BRANCH_DELETE        | name=branch(req), force=bool    | Delete branch (accepts comma-separated list)
REMOTE_BRANCH_DELETE | name="origin/branch"(req)       | Delete remote branch (accepts comma-separated list)
REMOTE_TAG_DELETE    | name=tag(req)                   | Delete remote tag (accepts comma-separated list)
TAG_CREATE           | name=tag(req)                   | Create tag
TAG_DELETE           | name=tag(req)                   | Delete local tag (accepts comma-separated list)
TAG_PUSH             | name=tag(req)                   | Push tag to remote (accepts comma-separated list)
TAG_DELETE_REMOTE    | name=tag(req)                   | Delete tag from remote (accepts comma-separated list)
MERGE                | branch=name(req)                | Merge branch into current
UNDO                 | No args                         | Revert last git_write operation
PRUNE_BACKUPS        | days=N(opt,default=7)           | Delete backups older than N days
UPDATE_CONFIG        | arg=key:value(req)              | Update git-courer configuration

RESPONSE: {"success":bool,"operation":"CMD","message":"description"}
Error:    {"status":"error","command":"CMD","error":"msg"}
`

// descGitWriteReview describes the 3-phase confirmed ops.
const descGitWriteReview = `Confirmed write ops with LLM preview. 3-phase protocol REQUIRED.

PHASES:
{OP}_START      | command(req), instruction(req) | Generate plan. May background if too long.
{OP}_APPLY      | command(req)                   | Execute after confirmation
{OP}_ABORT      | command(req)                   | Cancel operation
{OP}_REGENERATE | command(req), feedback(req)    | Regenerate with feedback
STATUS          | command(req)                   | Check pending operations
SUMMARY         | command(req)                   | Git state overview

OPERATIONS: COMMIT, RELEASE

PARAMETERS:
command     = phase pattern above (REQUIRED)
instruction = natural language for START
feedback    = string for REGENERATE

BACKGROUND PROTOCOL (Universal: COMMIT, RELEASE):
1. Any {OP}_START taking >45s (or RELEASE >2 chunks) returns {"status":"background","job_id":"..."}
2. Progress notifications: "⚙️ 2/5 chunks [job:ID]"
3. Completion: "✅ done [job:ID] → call {OP}_APPLY"

RULES:
- NEVER call APPLY without explicit user confirmation (exception: background notification says it's safe)
- ALWAYS show FULL preview to user before confirming
- COMMIT: conventional commits via Ollama
- RELEASE: auto-generates changelog, creates tag + GitHub release

EXAMPLES:
COMMIT_START instruction="commit all changes"  -> may return background status if slow
COMMIT_APPLY                                   -> execute after preview or job done
`
