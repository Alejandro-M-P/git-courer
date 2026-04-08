/**
 * git-courer — OpenCode plugin adapter
 *
 * Injects minimal git operation instructions into every system prompt.
 *
 * IMPORTANT: This plugin does NOT register any tools.
 * Tools are exposed via MCP (mcpServers config).
 * This plugin only exists to prevent the LLM from running git via bash.
 */

import type { Plugin } from "@opencode-ai/plugin"

// Git Instructions - prevent git-via-bash, trust MCP
const GIT_INSTRUCTIONS = `# Git Operations — git-courer MCP

## MANDATORY — No exceptions

You have access to these git tools via MCP. Use the RIGHT tool for each job.

### TOOLS:

1. **git_read** — Read-only operations
   - READ_STATUS: git status
   - READ_DIFF: git diff (unstaged)
   - READ_LOG: recent commits
   - READ_BRANCHES: current branch

2. **git_write** — Direct write operations (no confirmation needed)
   - ADD: stage files (subcommand: file path or "." for all)
   - PUSH, PULL, FETCH
   - CHECKOUT, SWITCH: change branches
   - STASH, STASH_POP

3. **git_write_commit** — Commit operations
   - COMMIT_START: start commit (preview=false for auto-commit)
   - COMMIT_APPLY: execute after approval
   - COMMIT_ABORT: cancel

4. **git_write_review** — Operations needing confirmation
   - BRANCH_CREATE, BRANCH_DELETE
   - MERGE, REBASE, RESET_HARD, CLEAN

### HARD RULES:

1. NEVER run \`git\` via bash/shell
2. NEVER generate commit messages yourself (Ollama does it)
3. ONE tool call per operation — no exploration

### QUICK REFERENCE:

| User wants | Tool | Call |
|------------|------|------|
| "status" | git_read | READ_STATUS |
| "diff" | git_read | READ_DIFF |
| "commit" | git_write_commit | COMMIT_START(preview=false) |
| "push" | git_write | PUSH |
| "pull" | git_write | PULL |
| "stage all" | git_write | ADD(".") |
| "checkout branch" | git_write | CHECKOUT(branch) |
| "new branch" | git_write_review | BRANCH_CREATE(branch) |
| "delete branch" | git_write_review | BRANCH_DELETE(branch) |
| "merge" | git_write_review | MERGE(branch) |
| "reset hard" | git_write_review | RESET_HARD(target) |

### COMMIT FLOW (ONE CALL):

User: "commit my changes"
Call: git_write_commit(COMMAND="COMMIT_START", PREVIEW=false)

That is all. git-courer handles staging, message generation, and committing.
`

// ─── Plugin Export ───────────────────────────────────────────────────────────

export const GitCourer: Plugin = async (ctx) => {
  return {
    "experimental.chat.system.transform": async (_input, output) => {
      if (output.system.length > 0) {
        output.system[output.system.length - 1] += "\n\n" + GIT_INSTRUCTIONS
      } else {
        output.system.push(GIT_INSTRUCTIONS)
      }
    },
  }
}
