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
{"command": "git_read", "args": ["READ_TAGS"]}
```

### git_write (Direct writes)
```json
{"command": "git_write", "args": ["ADD", "."]}
{"command": "git_write", "args": ["CHECKOUT", "branch-name"]}
{"command": "git_write", "args": ["PUSH"]}
{"command": "git_write", "args": ["PULL"]}
# Tag operations
{"command": "git_write", "args": ["TAG_CREATE", "v1.0.0"]}
{"command": "git_write", "args": ["TAG_DELETE", "v1.0.0"]}
{"command": "git_write", "args": ["TAG_PUSH", "v1.0.0"]}
{"command": "git_write", "args": ["TAG_DELETE_REMOTE", "v1.0.0"]}
```

### git_write_review (Workflow with LLM + confirmation)
```json
{"command": "git_write_review", "args": ["COMMIT_START", "instruction"]}
{"command": "git_write_review", "args": ["COMMIT_APPLY"]}
{"command": "git_write_review", "args": ["TAG_CREATE_START", "crear tag v1.0.0"]}
{"command": "git_write_review", "args": ["TAG_CREATE_APPLY"]}
{"command": "git_write_review", "args": ["TAG_DELETE_START", "borrar tag v1.0.0"]}
{"command": "git_write_review", "args": ["TAG_DELETE_APPLY"]}
{"command": "git_write_review", "args": ["RELEASE_START", "release"]}
{"command": "git_write_review", "args": ["RELEASE_APPLY"]}
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
← {status: "pending_approval", tag: "v1.1.0", changelog: "..."}
User: "confirm"
→ git_write_review(command="RELEASE_APPLY")
← "✓ Release created: v1.1.0"
```

### Tag Operations
```
# Create tag (direct)
→ git_write(command="TAG_CREATE", arg="v1.0.0")

# Delete tag with LLM
→ git_write_review(command="TAG_DELETE_START", instruction="borrar tag v1.0.0")
← {status: "pending_approval", preview: "Delete tag: v1.0.0"}
→ git_write_review(command="TAG_DELETE_APPLY")

# Push tag to remote
→ git_write(command="TAG_PUSH", arg="v1.0.0")

# Delete remote tag (⚠️ destructive)
→ git_write(command="TAG_DELETE_REMOTE", arg="v1.0.0")
```

## Error Handling

All operations send notifications. Listen for:
- `notifications/message` with level: "info" or "error"
- Check the "operation" field to identify what failed
- "hint" field often provides solution

## Detection

Each AI tool looks for instructions in different files:
- Claude Code → `CLAUDE.md`
- Cursor → `.cursorrules`
- Cline → `.clinerules`
- Windsurf → `.windsurfrules`
- Zed → `.zed/rules.md`
- OpenCode → skills in config

Copy this prompt to the appropriate file for your client.