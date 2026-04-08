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
git_write_commit(COMMIT_START, preview=false)
THAT'S IT. No staging, no status check, no confirmation.

### FOR OTHER OPERATIONS:

| User wants | Tool | Call |
|------------|------|------|
| status | git_read | READ_STATUS |
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
