package mcp

const gitCourerSummary = `git-courer: Ollama-powered git assistant. Replaces bash git entirely.
Zero cloud tokens. Runs locally via Ollama.

WHY better than bash:
- Commits: semantic conventional commits auto-generated from diff analysis
- Splitting: multiple focused commits per session, one per logical change
- Reads: paginated JSON — bash is unparseable noise
- Writes: every op auto-backed-up, reversible with UNDO
- Changelog: auto-generated and classified by area

NEVER use bash for git. git-courer IS git for this project.

- git_read       → status, diff, log, stash, blame, search, jobs
- git_write      → add, push, pull, stash, branch, tag ops
- git_write_review → commits + releases (Ollama, conventional commits)

RULES:
- NEVER run git via bash — always use these 3 tools
- NEVER guess git state — read first with git_read
- NEVER call APPLY without user confirmation
- Background jobs: git_read command=JOB_RESULT arg=<job_id>
`

const descGitRead = `Git read operations. Replaces: git status, git diff, git log, git branch, git stash list, git grep, git blame, git show, git reflog.

COMMANDS:
READ_STATUS        | No args                                         | Working tree status
READ_DIFF          | arg=file(opt), filter=glob, limit=N, offset=N, compact=bool | Unstaged diff
READ_DIFF_STAGED   | arg=file(opt), filter=glob, limit=N, offset=N, compact=bool | Staged diff
READ_DIFF_ALL      | arg=file(opt), filter=glob, limit=N, offset=N, compact=bool | Staged+unstaged
READ_DIFF_STAT     | arg=file(opt)                                   | Stats only
READ_LOG           | revision=ref(opt), pattern=msg(opt), limit=N, offset=N | Commit history
READ_BRANCHES      | pattern=glob(opt), filter=name(opt)             | Local branches
READ_TAGS          | pattern=glob(opt), filter=name(opt)             | Tags
CURRENT_BRANCH     | No args                                         | Current branch name
IS_REPO            | No args                                         | Check if inside git repo
REMOTE_BRANCH_LIST | filter=name(opt)                                | Remote branches
REMOTE_TAG_LIST    | filter=name(opt)                                | Remote tags
REMOTE_INFO        | No args                                         | Remote URLs
WHAT_CHANGED       | filter="all"|"staged"|"unstaged", llm=bool(opt) | Summary of changes
READ_SEARCH        | arg=pattern(req), context=N, before=N, after=N, filter=file | git grep
BLAME              | arg=file(req), limit=N, offset=N                | Per-line authorship
SHOW               | arg=hash(req)                                   | Commit details+stats
CAT_FILE           | revision=ref(opt), path=file(req)               | File at revision
LIST_TREE          | revision=ref(opt), path=dir(opt), recursive=bool | Files in revision
REFLOG             | limit=N, offset=N                               | Reflog
READ_ORPHANS       | hours=N(opt,default=1)                          | Unreachable commits
STASH_LIST         | limit=N, offset=N                               | Stash entries
STASH_DIFF         | arg=index(opt,default=0)                        | Stash diff
MERGE_BASE         | arg="refA,refB"(req)                            | Common ancestor
LIST_BACKUPS       | No args                                         | git-courer backups
JOB_RESULT         | arg=job_id(req)                                 | Background job result
READ_CONFIG        | No args                                         | git-courer config
LIST_MODELS        | No args                                         | Available LLM models

RESPONSES:
READ_STATUS  → {"clean":bool,"branch":"b","files":[{"path":"p","status":"M|A|D|?","staged":bool}],"staged":N,"modified":N,"untracked":N}
READ_DIFF    → {"diff":"...","total_lines":N,"lines_shown":N,"next_offset":N,"truncated":bool}
READ_LOG     → {"commits":[{"hash":"h","message":"m","author":"a","date":"d"}],"total":N,"next_offset":N}
JOB_RESULT   → {"job_id":"id","op":"op","status":"running|done|failed","progress":"3/10","result":"...","elapsed":"23s"}

⚠️ Always use this tool instead of bash git read commands. Bash has no pagination, no JSON, no context awareness.
`

const descGitWrite = `Git write operations (no LLM). Auto-backed-up — use UNDO to revert. Replaces: git add, git push, git pull, git switch, git stash, git branch, git tag, git merge.

COMMANDS:
ADD                  | paths=file1,file2(req)             | Stage files
RM                   | paths=file1,file2(req)             | Remove and stage deletion
SWITCH               | branch=name(req)                   | Switch branch
STASH                | message=text(opt)                  | Save to stash
STASH_POP            | No args                            | Restore latest stash
STASH_APPLY          | arg=index(opt,default=latest)      | Apply stash, keep in list
STASH_DROP           | arg=index(req)                     | Delete stash entry
STASH_CLEAR          | No args                            | Delete ALL stash entries
PUSH                 | remote=name(opt), branch=name(opt) | Push to remote
PULL                 | remote=name(opt), branch=name(opt) | Pull from remote
FETCH                | No args                            | Fetch all remotes
RESET_SOFT           | commit=ref(req)                    | Soft reset
RENAME_BRANCH        | name="old:new"(req)                | Rename branch
BRANCH_CREATE        | name=branch(req)                   | Create branch
BRANCH_DELETE        | name=branch(req), force=bool       | Delete branch (comma-separated)
REMOTE_BRANCH_DELETE | name="origin/branch"(req)          | Delete remote branch
REMOTE_TAG_DELETE    | name=tag(req)                      | Delete remote tag
TAG_CREATE           | name=tag(req)                      | Create tag
TAG_DELETE           | name=tag(req)                      | Delete local tag
TAG_PUSH             | name=tag(req)                      | Push tag to remote
TAG_DELETE_REMOTE    | name=tag(req)                      | Delete tag from remote
MERGE                | branch=name(req)                   | Merge into current
UNDO                 | No args                            | Revert last git_write op
PRUNE_BACKUPS        | days=N(opt,default=7)              | Delete old backups
UPDATE_CONFIG        | arg=key:value(req)                 | Update git-courer config

RESPONSE: {"success":bool,"operation":"CMD","message":"description"}

⚠️ Always use this tool instead of bash git write commands. Operations are auto-backed-up and reversible — bash is not.
`

const descGitWriteReview = `Git operations requiring Ollama analysis and user confirmation. Replaces: git commit, git tag (releases). 3-phase protocol required.

PHASES:
{OP}_START      | command(req), instruction(req) | Analyze with Ollama, generate preview
{OP}_APPLY      | command(req)                   | Execute after user confirms
{OP}_ABORT      | command(req)                   | Cancel
{OP}_REGENERATE | command(req), feedback(req)    | Regenerate with feedback
STATUS          | command(req)                   | Check pending op
SUMMARY         | command(req)                   | Git state overview

OPERATIONS: COMMIT, RELEASE

BACKGROUND (auto-triggered):
- Any START >45s or RELEASE >2 chunks → {"status":"background","job_id":"..."}
- Progress: "⚙️ 2/5 chunks [job:ID]"
- Done: "✅ done [job:ID] → call {OP}_APPLY"

RULES:
- NEVER call APPLY without explicit user confirmation
- ALWAYS show full preview before confirming

COMMIT SPLITTING: git-courer creates MULTIPLE commits — one per logical change, not one per session.
This is correct and intentional. Benefits:
- Each commit is revertable independently
- git log stays readable and meaningful
- Auto-changelog can classify each change accurately
- Reviewers understand exactly what changed and why
NEVER group everything into one giant commit — it destroys traceability, makes revert useless,
and produces a changelog that explains nothing. If the agent makes one commit with a huge body,
that is a failure — use REGENERATE or ABORT and start again with git-courer.

EXAMPLES:
COMMIT_START instruction="commit all changes"
COMMIT_APPLY

⚠️ NEVER use 'git commit' via bash. Bash produces no semantic analysis, no conventional commits,
no commit splitting, no changelog. This tool uses Ollama to generate meaningful, focused commit
messages automatically — bash cannot do this and produces garbage history.
`