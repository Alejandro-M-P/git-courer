/**
 * git-courer — OpenCode plugin adapter
 *
 * Injects minimal git operation instructions into every system prompt.
 *
 * IMPORTANT: This plugin does NOT register git_do as a callable tool.
 * The real git_do tool is exposed via MCP (mcpServers config).
 * This plugin only exists to prevent the LLM from running git via bash.
 *
 * The LLM should automatically discover git_do as an MCP tool when
 * OpenCode is configured to connect to git-courer via mcpServers.
 */

import type { Plugin } from "@opencode-ai/plugin"

// Git Instructions - prevent git-via-bash, trust MCP
const GIT_INSTRUCTIONS = `# Git Operations — git-courer MCP

## MANDATORY — No exceptions

You have access to \`git_do\`. This tool handles ALL git operations locally via Ollama.

### HARD RULES:

1. NEVER run \`git\` commands via bash/shell
2. NEVER read diffs, status, or logs before calling git_do
3. NEVER generate commit messages yourself
4. NEVER call git_do more than once per user request
5. NEVER plan or analyze before calling git_do

### WHEN USER ASKS FOR ANY GIT OPERATION:

Call git_do IMMEDIATELY with the user intent. Nothing else.

✅ CORRECT:
User: "commit my changes"
You: git_do("commit my changes")

✅ CORRECT:
User: "commit and push"
You: git_do("commit and push")

❌ WRONG — multiple calls:
You: git_do("commit") then git_do("push")

❌ WRONG — thinking first:
You: [reads diff] [analyzes] [generates message] git_do(...)

❌ WRONG — bash git:
You: \`git status\`, \`git add\`, \`git commit -m "feat: ..."\`

### ONE CALL. ONE INTENT. TRUST git-courer.

git-courer handles locally:
- Reading diffs
- Generating commit messages (Ollama)
- Detecting secrets
- Pushing to remote

Your job: pass the user intent to git_do. That is all.
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
