#!/bin/bash

# git-courer Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/Alejandro-M-P/git-courer/main/install.sh | sh

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Detect OS
detect_os() {
    case "$(uname -s)" in
        Linux*)     echo "linux";;
        Darwin*)    echo "macos";;
        MINGW*|MSYS*|CYGWIN*) echo "windows";;
        *)          echo "unknown";;
    esac
}

# Detect architecture
detect_arch() {
    case "$(uname -m)" in
        x86_64|AMD64)   echo "amd64";;
        aarch64|ARM64)  echo "arm64";;
        *)              echo "amd64";;
    esac
}

# Get latest version
get_latest_version() {
    curl -s https://api.github.com/repos/Alejandro-M-P/git-courer/releases/latest | grep -o '"tag_name": "[^"]*' | cut -d'"' -f4
}

# Download binary
download_binary() {
    local os=$1
    local arch=$2
    local version=$3
    
    echo -e "${BLUE}Downloading git-courer v${version}...${NC}"
    
    local url="https://github.com/Alejandro-M-P/git-courer/releases/download/${version}/git-courer-${os}-${arch}"
    
    curl -fsSL "$url" -o "$BINARY_PATH"
    chmod +x "$BINARY_PATH"
    
    echo -e "${GREEN}✓ Downloaded${NC}"
}

# Detect installed tools
detect_tools() {
    echo -e "${YELLOW}Detecting installed tools...${NC}"
    
    local tools=()
    
    # Check Opencode
    if command -v opencode &> /dev/null || [ -f "$HOME/.config/opencode/opencode.json" ]; then
        tools+=("opencode")
    fi
    
    # Check Claude Code
    if command -v claude &> /dev/null || [ -f "$HOME/.claude.json" ]; then
        tools+=("claude")
    fi
    
    # Check Cursor
    if command -v cursor &> /dev/null || [ -f "$HOME/.cursor" ]; then
        tools+=("cursor")
    fi
    
    if [ ${#tools[@]} -eq 0 ]; then
        echo -e "${YELLOW}No AI tools detected. You'll configure manually.${NC}"
    else
        echo -e "${GREEN}Found: ${tools[*]}${NC}"
    fi
    
    echo "${tools[*]}"
}

# Ask for tool
ask_tool() {
    echo -e "${BLUE}Which AI tool are you using?${NC}"
    echo "1) Opencode"
    echo "2) Claude Code"
    echo "3) Cursor"
    echo "4) Other / Manual"
    echo ""
    read -p "Select (1-4): " choice
    
    case $choice in
        1) echo "opencode";;
        2) echo "claude";;
        3) echo "cursor";;
        *) echo "manual";;
    esac
}

# Generate config for tool
generate_tool_config() {
    local tool=$1
    local project_dir=$2
    
    echo -e "${BLUE}Generating config for $tool...${NC}"
    
    case $tool in
        opencode)
            mkdir -p "$project_dir/.opencode"
            cat > "$project_dir/.opencode/config.json" << 'EOF'
{
  "mcp": {
    "git-courer": {
      "type": "local",
      "command": ["git-courer"]
    }
  }
}
EOF
            ;;
        claude)
            mkdir -p "$project_dir/.claude"
            cat > "$project_dir/.claude/settings.json" << 'EOF'
{
  "mcpServers": {
    "git-courer": {
      "command": "git-courer"
    }
  }
}
EOF
            ;;
        cursor)
            mkdir -p "$project_dir/.cursor"
            cat > "$project_dir/.cursor/mcp.json" << 'EOF'
{
  "mcpServers": {
    "git-courer": {
      "command": "git-courer"
    }
  }
}
EOF
            ;;
    esac
    
    echo -e "${GREEN}✓ Config generated${NC}"
}

# Add git-courer rules to AI tool config
add_tool_rules() {
    local tool=$1
    
    case $tool in
        opencode)
            # Add to AGENTS.md if exists
            if [ -f "AGENTS.md" ]; then
                if ! grep -q "git-courer" "AGENTS.md"; then
                    cat >> "AGENTS.md" << 'EOF'

## Git Operations

**For ALL git operations, ALWAYS use the MCP tools from git-courer:**
- git_status, git_diff, git_log, git_add, git_commit, git_push, git_pull, git_branch, git_checkout, git_stash, git_reset, git_ai_commit

**NEVER execute git directly with bash.** Always delegate to git-courer MCP tools.

This saves tokens on cloud AI by delegating all git work to the local git-courer server.

**When user asks for git operations (commit, push, etc):**
1. Execute the appropriate MCP tool(s) automatically
2. Return ONLY the result — no explanation, no description, no conversational text
3. Response should be ONE LINE only (e.g., "✅ Committed: feat: add feature [local-ollama]" or just the tool output)

**MINIMAL OUTPUT RULE**: Zero tokens for git responses. Just output the result.
EOF
                    echo -e "${GREEN}✓ Added rules to AGENTS.md${NC}"
                fi
            fi
            ;;
        claude)
            # Add to CLAUDE.md if exists
            if [ -f "CLAUDE.md" ]; then
                if ! grep -q "git-courer" "CLAUDE.md"; then
                    cat >> "CLAUDE.md" << 'EOF'

## Git Operations

**For ALL git operations, ALWAYS use the MCP tools from git-courer:**
- git_status, git_diff, git_log, git_add, git_commit, git_push, git_pull, git_branch, git_checkout, git_stash, git_reset, git_ai_commit

**NEVER execute git directly with bash.** Always delegate to git-courer MCP tools.

This saves tokens on cloud AI by delegating all git work to the local git-courer server.

**When user asks for git operations (commit, push, etc):**
1. Execute the appropriate MCP tool(s) automatically
2. Return ONLY the result — no explanation, no description, no conversational text
3. Response should be ONE LINE only (e.g., "✅ Committed: feat: add feature [local-ollama]" or just the tool output)

**MINIMAL OUTPUT RULE**: Zero tokens for git responses. Just output the result.
EOF

**NEVER execute git directly with bash.** Always delegate to git-courer MCP tools.
EOF
                    echo -e "${GREEN}✓ Added rules to CLAUDE.md${NC}"
                fi
            fi
            ;;
    esac
}

# Create git-courer.yaml template
create_config() {
    local project_dir=$1
    
    if [ -f "$project_dir/git-courer.yaml" ]; then
        echo -e "${YELLOW}Config already exists. Skipping.${NC}"
        return
    fi
    
    echo -e "${BLUE}Creating git-courer.yaml...${NC}"
    
    cat > "$project_dir/git-courer.yaml" << 'EOF'
# git-courer configuration
# This file is specific to each project

ollama:
  host: http://localhost:11434
  model: llama3.2
  auto_start: false

git:
  workdir: .
  auto_add_secrets: true
  require_clean_repo: false

validation:
  require_confirmation: true
  max_commit_length: 72

ui:
  theme: dark
  show_icons: true
EOF
    
    echo -e "${GREEN}✓ Config created${NC}"
}

# Add to .gitignore
add_gitignore() {
    local entries=(
        "# git-courer config"
        "git-courer.yaml"
        ""
        "# MCP configs"
        ".opencode/"
        ".claude/"
        ".cursor/"
    )
    
    if [ ! -f ".gitignore" ]; then
        touch .gitignore
    fi
    
    for entry in "${entries[@]}"; do
        if [ -z "$entry" ]; then
            continue
        fi
        if ! grep -q "^$entry$" ".gitignore" 2>/dev/null; then
            echo "$entry" >> .gitignore
        fi
    done
    
    echo -e "${GREEN}✓ Added to .gitignore${NC}"
}

# Main installation
main() {
    echo ""
    echo -e "${BLUE}╔════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║     git-courer Installer                ║${NC}"
    echo -e "${BLUE}║     Local Git Specialist                ║${NC}"
    echo -e "${BLUE}╚════════════════════════════════════════╝${NC}"
    echo ""
    
    local os=$(detect_os)
    local arch=$(detect_arch)
    
    echo -e "${BLUE}Detected: $os ($arch)${NC}"
    
    # Determine install path
    local install_dir="$HOME/.local/bin"
    
    if [ "$os" = "windows" ]; then
        install_dir="$LOCALAPPDATA/git-courer"
    fi
    
    # Create directory
    mkdir -p "$install_dir"
    
    BINARY_PATH="$install_dir/git-courer"
    
    # Get version
    local version=$(get_latest_version)
    if [ -z "$version" ]; then
        version="latest"
    fi
    
    # Download
    download_binary "$os" "$arch" "$version"
    
    # Add to PATH
    echo -e "${BLUE}Adding to PATH...${NC}"
    
    local shell_rc=""
    case "$SHELL" in
        *zsh) shell_rc="$HOME/.zshrc";;
        *bash) shell_rc="$HOME/.bashrc";;
        *) shell_rc="$HOME/.profile";;
    esac
    
    # Check if already in PATH
    if ! grep -q "$install_dir" "$shell_rc" 2>/dev/null; then
        echo "export PATH=\"\$PATH:$install_dir\"" >> "$shell_rc"
        echo -e "${GREEN}✓ Added to PATH ($shell_rc)${NC}"
        echo -e "${YELLOW}Please run: source $shell_rc${NC}"
    else
        echo -e "${GREEN}✓ Already in PATH${NC}"
    fi
    
    echo ""
    echo -e "${BLUE}═══════════════════════════════════════════${NC}"
    echo -e "${BLUE}  Setup git-courer in your project${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════${NC}"
    echo ""
    
    # Ask for tool
    local tool=$(ask_tool)
    local project_dir="${PWD}"
    
    echo ""
    
    # Create project config
    create_config "$project_dir"
    add_gitignore
    
    # Generate tool config
    if [ "$tool" != "manual" ]; then
        generate_tool_config "$tool" "$project_dir"
        add_tool_rules "$tool"
    fi
    
    echo ""
    echo -e "${GREEN}═══════════════════════════════════════════${NC}"
    echo -e "${GREEN}  Installation complete!${NC}"
    echo -e "${GREEN}═══════════════════════════════════════════${NC}"
    echo ""
    echo "Next steps:"
    echo "1. Run: source ~/.bashrc (or ~/.zshrc)"
    echo "2. Restart your AI tool (Opencode/Claude/Cursor)"
    echo "3. In your project, run: git-courer"
    echo ""
    echo "For help: https://github.com/Alejandro-M-P/git-courer"
    echo ""
}

main "$@"
