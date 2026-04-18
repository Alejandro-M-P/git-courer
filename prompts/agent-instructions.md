# Git-Courer Prompt

> Instructions for AI coding agents using git-courer MCP server.

## What is git-courer?

git-courer is a local git assistant that uses Ollama (local LLM) for natural language git operations. It runs as an MCP server and handles ALL git operations.

## MCP Server

git-courer provides these MCP tools:

### git_read (Read-only operations)
```json
{"command": "git_read", "args": ["READ_STATUS"]}
{"command": "git_read", "args": ["READ_DIFF"]}
{"command": "git_read", "args": ["READ_LOG"]}
{"command": "git_read", "args": ["READ_BRANCHES"]}
```

### git_write (Direct writes)
```json
{"command": "git_write", "args": ["ADD", "."]}
{"command": "git_write", "args": ["CHECKOUT", "branch-name"]}
{"command": "git_write", "args": ["PUSH"]}
{"command": "git_write", "args": ["PULL"]}
```

### git_write_review (Workflow with confirmation)
```json
{"command": "git_write_review", "args": ["COMMIT_START", "instruction"]}
{"command": "git_write_review", "args": ["COMMIT_APPLY"]}
{"command": "git_write_review", "args": ["BRANCH_CREATE_START", "name"]}
{"command": "git_write_review", "args": ["BRANCH_CREATE_APPLY"]}
```

## HARD RULES

1. **NEVER run `git` commands via bash** — always use the MCP tools
2. **NEVER generate commit messages yourself** — let git-courer/Ollama do it
3. **NEVER read diffs or status before calling a tool** — the tools handle that
4. **Use confirmation workflow for commits, branches, releases** — START → wait → APPLY

## Workflow Examples

### Commit Changes
```
User: "commit my changes"
→ git_write_review(command="COMMIT_START", instruction="commit all changes")
← {status: "pending_approval", preview: "Commit: feat: add feature"}
User: "confirm"
→ git_write_review(command="COMMIT_APPLY")
← "✓ Committed: feat: add feature"
```

### Create Branch
```
User: "create branch for login"
→ git_write_review(command="BRANCH_CREATE_START", instruction="create branch for login")
← {status: "pending_approval", preview: "Create branch: feat/login"}
User: "confirm"
→ git_write_review(command="BRANCH_CREATE_APPLY")
← "✓ Branch created: feat/login"
```

### Release
```
User: "create a release"
→ git_write_review(command="RELEASE_START", instruction="create release")
...
```

## Detection

Each AI tool looks for instructions in different files:
- Claude Code → `CLAUDE.md`
- Cursor → `.cursorrules`
- Cline → `.clinerules`
- Windsurf → `.windsurfrules`
- Zed → `.zed/rules.md`
- OpenCode → skills in config

Copy this prompt to the appropriate file for your client.