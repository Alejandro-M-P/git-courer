#!/usr/bin/env bash

# git-courer Agent Setup — Configure MCP for detected AI tools.
# This is the equivalent of `engram setup [agent]` but for git-courer.
#
# Usage:
#   ./setup-agents.sh              # Detect and configure all agents
#   ./setup-agents.sh opencode     # Configure only OpenCode
#   ./setup-agents.sh claude       # Configure only Claude Code
#   ./setup-agents.sh --list       # List supported agents

set -euo pipefail

BINARY_NAME="git-courer"

# ─── Colors ──────────────────────────────────────────────────────────────────

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

info()    { echo -e "${CYAN}  $1${NC}"; }
success() { echo -e "${GREEN}✓ $1${NC}"; }
warn()    { echo -e "${YELLOW}⚠ $1${NC}"; }
error()   { echo -e "${RED}✗ $1${NC}" >&2; }

# ─── Platform Detection ──────────────────────────────────────────────────────

detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "macos" ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        *) echo "unknown" ;;
    esac
}

get_home_dir() {
    local os
    os=$(detect_os)
    case "$os" in
        windows)
            if [ -n "${USERPROFILE:-}" ]; then
                echo "$USERPROFILE"
            elif [ -n "${HOMEDRIVE:-}" ] && [ -n "${HOMEPATH:-}" ]; then
                echo "${HOMEDRIVE}${HOMEPATH}"
            else
                echo "/c/Users/$(whoami 2>/dev/null || echo "user")"
            fi
            ;;
        *)
            echo "${HOME:-$(eval echo ~)}"
            ;;
    esac
}

# ─── Binary Resolution ───────────────────────────────────────────────────────

resolve_binary_path() {
    local os
    os=$(detect_os)
    local binary="${BINARY_NAME}"
    [ "$os" = "windows" ] && binary="${BINARY_NAME}.exe"

    if command -v "${binary}" &>/dev/null; then
        local resolved
        resolved="$(command -v "${binary}")"
        if command -v realpath &>/dev/null; then
            realpath "$resolved" 2>/dev/null || echo "$resolved"
        else
            echo "$resolved"
        fi
        return
    fi

    local home
    home=$(get_home_dir)
    
    # Also check the repo directory (when running from source)
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    REPO_ROOT="$(cd "${SCRIPT_DIR}/.." 2>/dev/null && pwd || echo "")"
    
    local candidates=()
    case "$os" in
        linux)
            candidates=("$home/.local/bin/${binary}" "/usr/local/bin/${binary}" "/usr/bin/${binary}" "$home/go/bin/${binary}" "${REPO_ROOT}/${binary}")
            ;;
        macos)
            candidates=("$home/.local/bin/${binary}" "/usr/local/bin/${binary}" "/opt/homebrew/bin/${binary}" "$home/go/bin/${binary}" "${REPO_ROOT}/${binary}")
            ;;
        windows)
            candidates=("$home/.local/bin/${binary}" "$home/AppData/Local/bin/${binary}" "$home/AppData/Roaming/bin/${binary}" "$home/go/bin/${binary}" "${REPO_ROOT}/${binary}")
            ;;
    esac

    for candidate in "${candidates[@]}"; do
        if [ -f "$candidate" ]; then
            if command -v realpath &>/dev/null; then
                realpath "$candidate" 2>/dev/null || echo "$candidate"
            else
                echo "$candidate"
            fi
            return
        fi
    done

    echo "${binary}"
}

# ─── Agent Config Paths ──────────────────────────────────────────────────────

agent_config_path() {
    local tool=$1
    local os
    os=$(detect_os)
    local home
    home=$(get_home_dir)

    case "$tool" in
        opencode)
            case "$os" in
                linux)   echo "${XDG_CONFIG_HOME:-$home/.config}/opencode/opencode.json" ;;
                macos)   echo "$home/.config/opencode/opencode.json" ;;
                windows) echo "$home/.config/opencode/opencode.json" ;;
            esac
            ;;
        claude)
            echo "$home/.claude/settings.json"
            ;;
        claude_desktop)
            case "$os" in
                macos)   echo "$home/Library/Application Support/Claude/claude_desktop_config.json" ;;
                windows) echo "${APPDATA:-$home/AppData/Roaming}/Claude/claude_desktop_config.json" ;;
                *)       echo "" ;;
            esac
            ;;
        cursor)
            echo "$home/.cursor/mcp.json"
            ;;
        cline)
            case "$os" in
                macos)   echo "$home/Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json" ;;
                windows) echo "${APPDATA:-$home/AppData/Roaming}/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json" ;;
                linux)   echo "$home/.config/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json" ;;
            esac
            ;;
        continue)
            echo "$home/.continue/config.json"
            ;;
        zed)
            case "$os" in
                linux)   echo "${XDG_CONFIG_HOME:-$home/.config}/zed/settings.json" ;;
                *)       echo "$home/.config/zed/settings.json" ;;
            esac
            ;;
        windsurf)
            echo "$home/.codeium/windsurf/mcp_config.json"
            ;;
        vscode)
            case "$os" in
                macos)   echo "$home/Library/Application Support/Code/User/globalStorage/github.copilot-chat/mcp.json" ;;
                windows) echo "${APPDATA:-$home/AppData/Roaming}/Code/User/globalStorage/github.copilot-chat/mcp.json" ;;
                linux)   echo "$home/.config/Code/User/globalStorage/github.copilot-chat/mcp.json" ;;
            esac
            ;;
        codex)
            echo "$home/.codex/mcp.json"
            ;;
    esac
}

# ─── JSON Helpers ──────────────────────────────────────────────────────────────
# Uses python3 for safe JSON merge — preserves existing config.
# Falls back to jq if available. Never uses unsafe bash on existing files.

inject_mcp_json() {
    local config_path=$1
    local root_key=$2
    local entry_json=$3

    local dir
    dir="$(dirname "$config_path")"
    mkdir -p "$dir"

    # Check if already configured
    if [ -f "$config_path" ] && grep -q "git-courer" "$config_path" 2>/dev/null; then
        echo "  MCP already configured"
        return
    fi

    # Try python3 first (safest JSON merge — preserves existing config)
    if command -v python3 &>/dev/null; then
        python3 - "$config_path" "$root_key" "$entry_json" << 'PYEOF'
import json, os, sys
config_path = sys.argv[1]
root_key = sys.argv[2]
entry = json.loads(sys.argv[3])
if os.path.exists(config_path):
    with open(config_path) as f:
        try:
            config = json.load(f)
        except json.JSONDecodeError:
            config = {}
else:
    config = {}
if root_key not in config:
    config[root_key] = {}
if 'git-courer' in config[root_key]:
    print('  MCP already configured')
    sys.exit(0)
config[root_key]['git-courer'] = entry
with open(config_path, 'w') as f:
    json.dump(config, f, indent=2)
    f.write('\n')
print('  Added MCP config')
PYEOF
        return
    fi

    # Try jq if available
    if command -v jq &>/dev/null; then
        local tmp="${config_path}.tmp"
        if [ -f "$config_path" ]; then
            jq --arg key "$root_key" --argjson entry "$entry_json" \
                '.[$key] = ((.[$key] // {}) + {"git-courer": $entry})' \
                "$config_path" > "$tmp" 2>/dev/null && mv "$tmp" "$config_path"
        else
            printf '{\n  "%s": {\n    "git-courer": %s\n  }\n}\n' "$root_key" "$entry_json" > "$config_path"
        fi
        echo "  Added MCP config"
        return
    fi

    # No python3, no jq — DON'T touch existing complex JSON files
    if [ -f "$config_path" ]; then
        echo "  ⚠ Cannot safely merge JSON without python3 or jq"
        echo "    Install python3 or configure manually"
        return
    fi

    # File doesn't exist — safe to create
    printf '{\n  "%s": {\n    "git-courer": %s\n  }\n}\n' "$root_key" "$entry_json" > "$config_path"
    echo "  Created config with MCP"
}

# ─── Instructions ──────────────────────────────────────────────────────────────
# Each agent has a different file for project-level instructions.

GIT_INSTRUCTIONS='# Git Operations — Use git-courer MCP

For ALL git operations, use the `git_do` MCP tool.
NEVER run `git` commands directly via bash.

## 🚨 ONE CALL PER REQUEST - MANDATORY
If you call git_do more than once, you will get an ERROR:
  "Only ONE git_do call allowed per request"

✅ CORRECT — single call:
  git_do("commit all changes and push to remote")

❌ WRONG — multiple calls (ERROR):
  git_do("show status")
  git_do("commit and push")

## 🚨 EXECUTE ONLY — NEVER THINK
When the user asks for ANY git operation, execute DIRECTLY with git_do.
- DO NOT analyze or plan before executing
- DO NOT read diffs first
- DO NOT check status before committing
- git-courer + Ollama handle everything locally

The user validates git operations through the UI — NOT through the orchestrator.

## Examples:
- `git_do("commit all changes and push")` — commit + push
- `git_do("show status")` — git status
- `git_do("create branch for auth feature")` — creates and switches

## Rules:
1. NEVER use `git status`, `git log`, `git diff`, `git commit`, `git push` via bash
2. ALWAYS use `git_do("...")` — ONE call, ONE intent
3. Pass the user intent EXACTLY — do not translate or plan
4. TRUST git-courer — it handles analysis, commit messages, and validation locally
'

# instruction_path returns the file where instructions should be written for each agent
instruction_path() {
    local tool=$1
    local os
    os=$(detect_os)
    local home
    home=$(get_home_dir)

    case "$tool" in
        opencode)
            # Plugin TS file — handled separately in configure_opencode
            echo ""
            ;;
        claude)
            # Claude Code: CLAUDE.md in project root
            echo "CLAUDE.md"
            ;;
        cursor)
            # Cursor: .cursorrules in project root
            echo ".cursorrules"
            ;;
        cline)
            # Cline: .clinerules in project root
            echo ".clinerules"
            ;;
        windsurf)
            # Windsurf: .windsurfrules in project root
            echo ".windsurfrules"
            ;;
        zed)
            # Zed: .zed/rules.md in project root
            echo ".zed/rules.md"
            ;;
        vscode)
            # VS Code Copilot: .github/copilot-instructions.md
            echo ".github/copilot-instructions.md"
            ;;
        codex)
            # Codex: ~/.codex/instructions.md (global)
            echo "$home/.codex/instructions.md"
            ;;
        continue)
            # Continue: no standard instructions file
            echo ""
            ;;
        claude_desktop)
            # Claude Desktop: no project-level instructions
            echo ""
            ;;
    esac
}

# write_instructions writes the git-courer instructions to the agent's instruction file
write_instructions() {
    local tool=$1
    local path
    path="$(instruction_path "$tool")"

    # Skip if no instruction file for this agent
    [ -z "$path" ] && return

    local dir
    dir="$(dirname "$path")"

    # Skip if file already has git-courer instructions
    if [ -f "$path" ] && grep -q "git_do" "$path" 2>/dev/null; then
        echo "  Instructions already present"
        return
    fi

    mkdir -p "$dir"

    if [ -f "$path" ]; then
        # Append to existing file
        printf '\n---\n\n%s\n' "$GIT_INSTRUCTIONS" >> "$path"
        echo "  Appended git-courer instructions to $path"
    else
        # Create new file
        printf '%s\n' "$GIT_INSTRUCTIONS" > "$path"
        echo "  Created $path with git-courer instructions"
    fi
}

# ─── Agent Configuration ─────────────────────────────────────────────────────

configure_opencode() {
    local config_path bin_path entry
    config_path="$(agent_config_path opencode)"
    bin_path="$1"
    info "Configuring OpenCode MCP..."

    # OpenCode/Crush uses mcpServers format (NOT "mcp" root key)
    # Command must be a STRING, not an array
    entry=$(printf '{"type":"stdio","command":"%s"}' "$bin_path")
    inject_mcp_json "$config_path" "mcpServers" "$entry"

    # Plugin is no longer needed — MCP is the real connection
    # Just leave a note in case someone wonders why the plugin folder exists
    echo "  MCP configured (stdio mode)"
    echo "  Note: Plugin git-courier.ts is not needed for MCP connection"
}

configure_claude() {
    local config_path bin_path entry
    config_path="$(agent_config_path claude)"
    bin_path="$1"
    if command -v claude &>/dev/null; then
        info "Configuring Claude Code via plugin system..."
        if claude plugin marketplace add "Alejandro-M-P/git-courer" 2>/dev/null; then
            claude plugin install "git-courer" 2>/dev/null || true
            success "Claude Code plugin installed"
            return
        fi
    fi
    info 'Configuring Claude Code (MCP fallback)...'
    # Claude Code uses mcpServers with command as string
    entry=$(printf '{"command":"%s","label":"git-courer"}' "$bin_path")
    inject_mcp_json "$config_path" "mcpServers" "$entry"
}

configure_claude_desktop() {
    local config_path
    config_path="$(agent_config_path claude_desktop)"
    [ -z "$config_path" ] && { warn "Claude Desktop not supported on this platform"; return; }
    info "Configuring Claude Desktop..."
    local entry
    entry=$(printf '{"command":"%s"}' "$1")
    inject_mcp_json "$config_path" "mcpServers" "$entry"
}

configure_cursor() {
    local config_path entry
    config_path="$(agent_config_path cursor)"
    info "Configuring Cursor..."
    entry=$(printf '{"command":"%s"}' "$1")
    inject_mcp_json "$config_path" "mcpServers" "$entry"
}

configure_cline() {
    local config_path entry
    config_path="$(agent_config_path cline)"
    info "Configuring Cline..."
    entry=$(printf '{"command":"%s"}' "$1")
    inject_mcp_json "$config_path" "mcpServers" "$entry"
}

configure_continue() {
    local config_path bin_path
    config_path="$(agent_config_path continue)"
    bin_path="$1"
    info "Configuring Continue..."

    # Continue uses ARRAY format: {"mcpServers":[{"name":"git-courer","command":"/path"}]}
    if [ -f "$config_path" ] && grep -q "git-courer" "$config_path" 2>/dev/null; then
        echo "  MCP already configured"
        return
    fi

    if [ ! -f "$config_path" ]; then
        printf '{\n  "mcpServers": [\n    {"name": "git-courer", "command": "%s"}\n  ]\n}\n' "$bin_path" > "$config_path"
        echo "  Created config with MCP"
        return
    fi

    # File exists — inject into array before closing ]
    local tmp="${config_path}.tmp"
    local line_count
    line_count=$(wc -l < "$config_path")
    head -n $((line_count - 1)) "$config_path" > "$tmp"
    sed -i '$ s/,$//' "$tmp"
    printf ',\n    {"name": "git-courer", "command": "%s"}\n  ]\n}\n' "$bin_path" >> "$tmp"
    mv "$tmp" "$config_path"
    echo "  Added MCP config"
}

configure_zed() {
    local config_path bin_path
    config_path="$(agent_config_path zed)"
    bin_path="$1"
    info "Configuring Zed..."

    # Zed format: {"context_servers":{"git-courer":{"command":{"path":"/path"}}}}
    if [ -f "$config_path" ] && grep -q "git-courer" "$config_path" 2>/dev/null; then
        echo "  MCP already configured"
        return
    fi

    if [ ! -f "$config_path" ]; then
        printf '{\n  "context_servers": {\n    "git-courer": {"command": {"path": "%s"}}\n  }\n}\n' "$bin_path" > "$config_path"
        echo "  Created config with MCP"
        return
    fi

    # File exists — inject before closing brace
    local tmp="${config_path}.tmp"
    local line_count
    line_count=$(wc -l < "$config_path")
    head -n $((line_count - 1)) "$config_path" > "$tmp"
    sed -i '$ s/,$//' "$tmp"
    printf ',\n  "context_servers": {\n    "git-courer": {"command": {"path": "%s"}}\n  }\n}\n' "$bin_path" >> "$tmp"
    mv "$tmp" "$config_path"
    echo "  Added MCP config"
}

configure_windsurf() {
    local config_path entry
    config_path="$(agent_config_path windsurf)"
    info "Configuring Windsurf..."
    entry=$(printf '{"command":"%s"}' "$1")
    inject_mcp_json "$config_path" "mcpServers" "$entry"
}

configure_vscode() {
    local config_path entry
    config_path="$(agent_config_path vscode)"
    info 'Configuring VS Code (Copilot)...'
    entry=$(printf '{"command":"%s"}' "$1")
    inject_mcp_json "$config_path" "servers" "$entry"
}

configure_codex() {
    local config_path entry
    config_path="$(agent_config_path codex)"
    info "Configuring Codex..."
    entry=$(printf '{"command":"%s"}' "$1")
    inject_mcp_json "$config_path" "mcpServers" "$entry"
}

# ─── Agent Detection ─────────────────────────────────────────────────────────

detect_agent() {
    local tool=$1
    local os
    os=$(detect_os)
    local home
    home=$(get_home_dir)
    case "$tool" in
        opencode)       command -v opencode &>/dev/null ;;
        claude)         command -v claude &>/dev/null ;;
        cursor)         command -v cursor &>/dev/null ;;
        zed)            command -v zed &>/dev/null ;;
        code)           command -v code &>/dev/null ;;
        codex)          command -v codex &>/dev/null ;;
        windsurf)       command -v windsurf &>/dev/null ;;
        claude_desktop)
            case "$os" in
                macos)   [ -d "$home/Library/Application Support/Claude" ] 2>/dev/null ;;
                windows) [ -d "${APPDATA:-$home/AppData/Roaming}/Claude" ] 2>/dev/null ;;
                *)       false ;;
            esac
            ;;
        cline)
            case "$os" in
                macos)   [ -d "$home/Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev" ] 2>/dev/null ;;
                windows) [ -d "${APPDATA:-$home/AppData/Roaming}/Code/User/globalStorage/saoudrizwan.claude-dev" ] 2>/dev/null ;;
                linux)   [ -d "$home/.config/Code/User/globalStorage/saoudrizwan.claude-dev" ] 2>/dev/null ;;
            esac
            ;;
        continue)       [ -d "$home/.continue" ] 2>/dev/null ;;
        vscode)         command -v code &>/dev/null ;;
    esac
}

# ─── Main ─────────────────────────────────────────────────────────────────────

main() {
    local target="${1:-}"

    if [ "$target" = "--list" ] || [ "$target" = "-l" ]; then
        echo 'Supported agents:'
        echo '  opencode        - OpenCode (CLI)'
        echo '  claude          - Claude Code (CLI)'
        echo '  cursor          - Cursor (CLI)'
        echo '  zed             - Zed (CLI)'
        echo '  codex           - Codex CLI (CLI)'
        echo '  windsurf        - Windsurf (CLI)'
        echo '  claude_desktop  - Claude Desktop (app)'
        echo '  cline           - Cline (VS Code extension)'
        echo '  continue        - Continue (VS Code/JetBrains extension)'
        echo '  vscode          - VS Code + Copilot (CLI)'
        exit 0
    fi

    local bin_path
    bin_path="$(resolve_binary_path)"

    if [ -n "$target" ]; then
        # Configure specific agent
        if declare -f "configure_${target}" &>/dev/null; then
            info "Configuring ${target}..."
            "configure_${target}" "$bin_path"
        else
            error "Unknown agent: ${target}"
            echo "Run with --list to see supported agents."
            exit 1
        fi
    else
        # Auto-detect and configure all
        echo -e "${BOLD}git-courer Agent Setup${NC}"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo ""

        local configured=0
        for tool in opencode claude cursor zed codex windsurf claude_desktop cline continue vscode; do
            if detect_agent "$tool"; then
                "configure_${tool}" "$bin_path"
                write_instructions "$tool"
                configured=$((configured + 1))
            fi
        done

        echo ""
        if [ "$configured" -eq 0 ]; then
            warn "No agents detected."
            echo "Supported agents:"
            echo "  Opencode, Claude Code, Cursor, Zed, Codex, Windsurf"
            echo "  Claude Desktop, Cline, Continue, VS Code"
        else
            success "Configured ${configured} agent(s)"
        fi
    fi
}

main "$@"
