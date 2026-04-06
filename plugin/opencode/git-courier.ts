/**
 * git-courer — OpenCode plugin adapter
 *
 * Injects git operation instructions into every system prompt so the agent
 * ALWAYS knows to use the git_local_task MCP tool instead of running git
 * commands via bash.
 *
 * This saves cloud tokens — all git operations (status, diff, commit, push)
 * are handled locally by the git-courer binary via MCP.
 */

import type { Plugin } from "@opencode-ai/plugin"

// ─── Git Instructions ────────────────────────────────────────────────────────
// These get injected into the agent's system prompt so it knows to use
// git_local_task instead of running git commands via bash.

const GIT_INSTRUCTIONS = `## Git Operations — Use git-courer MCP

For ALL git operations, use the \`git_local_task\` MCP tool.
NEVER run \`git\` commands directly via bash.

### TRUST & DELEGATE (zero cloud tokens):
When the user asks for ANY git operation, delegate DIRECTLY to \`git_local_task\`.
- DO NOT read diffs first
- DO NOT analyze changes before committing
- DO NOT check status before delegating
- DO NOT try to understand what will be committed
- git-courer + Ollama handle everything locally

The user validates git operations through the UI — NOT through the orchestrator.

### Read-only (instant, no AI tokens):
- \`git_local_task("show status")\` — working tree status
- \`git_local_task("show log")\` — recent commit history
- \`git_local_task("show diff")\` — unstaged changes
- \`git_local_task("show branches")\` — current branch

### Write operations (uses local Ollama):
- \`git_local_task("commit the login changes")\` — analyzes diff, generates message, commits
- \`git_local_task("push to remote")\` — pushes current branch
- \`git_local_task("create branch for auth feature")\` — creates and switches branch

### Rules:
1. NEVER use \`git status\`, \`git log\`, \`git diff\`, \`git commit\`, \`git push\` via bash
2. ALWAYS use \`git_local_task("...")\` for any git operation
3. Describe intent in natural language: "commit the auth changes", "show what's modified"
4. TRUST git-courer — it handles analysis, commit messages, and validation locally
`

// ─── Plugin Export ───────────────────────────────────────────────────────────

export const GitCourer: Plugin = async (ctx) => {
  return {
    // ─── System Prompt: Always-on git instructions ──────────────────
    // Injects GIT_INSTRUCTIONS into the system prompt of every message.
    // This ensures the agent ALWAYS knows about git-courer, even after compaction.
    //
    // We append to the last existing system entry instead of pushing a new one.
    // Some models (Qwen3.5, Mistral/Ministral via llama.cpp) reject multiple
    // system messages — their Jinja chat templates only allow a single system
    // block at the beginning. By concatenating, we avoid adding extra system
    // messages that would break these models.

    "experimental.chat.system.transform": async (_input, output) => {
      if (output.system.length > 0) {
        output.system[output.system.length - 1] += "\n\n" + GIT_INSTRUCTIONS
      } else {
        output.system.push(GIT_INSTRUCTIONS)
      }
    },
  }
}
