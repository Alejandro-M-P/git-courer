/**
 * git-courer — OpenCode plugin adapter
 *
 * Injects minimal git operation instructions into every system prompt.
 */

import type { Plugin } from "@opencode-ai/plugin"

// STRICT instructions — ONE CALL, NO EXPLORATION
const GIT_INSTRUCTIONS = `## Git Operations — git-courer MCP

### COMMANDMENTS:

1. **ONE CALL** — No exploration, no reading status first
2. **NO GIT BASH** — Never run \`git\` via shell
3. **NO PREVIOUS HISTORY** — Don't read status/diff before acting
4. **OLLAMA GENERATES MESSAGES** — Never write commit messages yourself

### FOR COMMITS — ONE CALL ONLY:

When user says "commit", "commitea", "guarda" or similar:
git_write_commit(COMMIT_START)
Use the tool's default for preview. No staging, no confirmation.

### FOR RELEASES:

When user says "sacar versión", "release", "nueva versión" or similar:
- Use: git_write_review(RELEASE_START, instruction="...")
- The system will generate changelog from commits since last release
- WAIT for user confirmation BEFORE applying!
- Only call RELEASE_APPLY after user explicitly confirms

### FOR TAGS:

When user says "crear tag X" or "borrar tag X":
- Use: git_write_review(TAG_CREATE_START) or TAG_DELETE_START

### FOR READ OPERATIONS:

| User wants | Tool | Command | arg |
|------------|------|---------|-----|
| status | git_read | READ_STATUS | — |
| diff (all) | git_read | READ_DIFF | — |
| diff (specific file) | git_read | READ_DIFF | "path/to/file" |
| staged diff | git_read | READ_DIFF_STAGED | — |
| staged diff (file) | git_read | READ_DIFF_STAGED | "path/to/file" |
| log | git_read | READ_LOG | — |
| log (file history) | git_read | READ_LOG | "path/to/file" |
| branches | git_read | READ_BRANCHES | — |
| branches (filter) | git_read | READ_BRANCHES | "feat/*" |
| tags | git_read | READ_TAGS | — |
| tags (filter) | git_read | READ_TAGS | "v1.*" |

### FOR WRITE OPERATIONS:

| User wants | Tool | Call |
|------------|------|------|
| push | git_write | PUSH |
| pull | git_write | PULL |
| stash | git_write | STASH |
| checkout branch | git_write | CHECKOUT(branch) |
| new branch | git_write_review | BRANCH_CREATE(name) |
| merge | git_write_review | MERGE(branch) |
| delete branch | git_write_review | BRANCH_DELETE(name) |

### EXECUTE IMMEDIATELY. NO THINKING. NO EXPLORATION.`

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