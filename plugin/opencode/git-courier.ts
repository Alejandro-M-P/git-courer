/**
 * git-courer — OpenCode plugin adapter
 *
 * Injects git operation instructions into every system prompt so the agent
 * ALWAYS knows to use the git_do MCP tool instead of running git
 * commands via bash.
 *
 * This saves cloud tokens — all git operations (status, diff, commit, push)
 * are handled locally by the git-courer binary via MCP.
 */

import type { Plugin } from "@opencode-ai/plugin"

// ─── Git Instructions ────────────────────────────────────────────────────────
// These get injected into the agent's system prompt so it knows to use
// git_do instead of running git commands via bash.

const GIT_INSTRUCTIONS = `## 🚨 CRITICAL: Git Operations — EXECUTE ONLY, NEVER THINK

When the user asks for ANY git operation:

### 🔥 USE git_do — THE ONLY GIT TOOL

DO NOT analyze, think, or plan. Just EXECUTE exactly what the user says.

✅ CORRECT — direct execution:
  git_do("commit all changes and push to remote")
  git_do("show status")
  git_do("create branch for auth feature")

❌ WRONG — thinking/analysis before execution:
  git_do("show status, show diff, show recent commits")  ← NO
  git_do("check status then commit")                    ← NO  
  git_do("show me the diff first")                       ← NO

### ⚠️ ONE CALL PER REQUEST - MANDATORY
If you call git_do more than once, you will get an ERROR:
  "Only ONE git_do call allowed per request"

✅ CORRECT — single call:
  git_do("commit all changes and push to remote")

❌ WRONG — multiple calls (ERROR):
  git_do("show status")
  git_do("commit and push")

### ✅ ALWAYS DO:
- ONE call, ONE intent (what the user said)
- git-courer handles context internally — you don't need to know what's happening
- If you don't know what to do, pass the user's exact words to git_do
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
