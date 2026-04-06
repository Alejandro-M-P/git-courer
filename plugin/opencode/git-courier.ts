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

// Minimal instructions - just prevent git-via-bash
// The LLM should discover git_do automatically as an MCP tool
const GIT_INSTRUCTIONS = `## 🚨 Git Operations — Use MCP Tool ONLY

When the user asks for ANY git operation:
- NEVER run \`git\` commands via bash
- ALWAYS use the \`git_do\` MCP tool
- The \`git_do\` tool handles everything (commit messages, push, etc.)

The \`git_do\` tool is available as an MCP tool when git-courer is configured.
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
