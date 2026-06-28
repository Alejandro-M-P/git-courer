#!/usr/bin/env bash

# git-courer Installer — curl | sh entry point.
# Downloads the binary and configures detected AI agents.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/blak0p/git-courer/main/scripts/install.sh | sh
#   curl -fsSL ... | sh -s -- --binary     # Only download binary
#   curl -fsSL ... | sh -s -- --agents     # Only configure agents (binary already installed)
#   curl -fsSL ... | sh -s -- --ollama     # Only setup Ollama

set -euo pipefail

BINARY_NAME="git-courer"
REPO="blak0p/git-courer"
GITHUB_RELEASES="https://api.github.com/repos/${REPO}/releases"

# ─── Platform Detection ──────────────────────────────────────────────────────

detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        *) echo "unknown" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64)  echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *) echo "$(uname -m)" ;;
    esac
}

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

# ─── Shell Detection ─────────────────────────────────────────────────────────

detect_shell() {
    if [ -n "$BASH_VERSION" ]; then
        echo "bash"
    elif [ -n "$ZSH_VERSION" ]; then
        echo "zsh"
    elif [ -n "$FISH_VERSION" ]; then
        echo "fish"
    else
        echo "unknown"
    fi
}

# ─── PATH Setup ──────────────────────────────────────────────────────────────

setup_path() {
    local install_dir="$1"
    local shell_type=$(detect_shell)

    # Check if already in PATH
    local current_path="${PATH}"
    case ":${current_path}:" in
        *:${install_dir}:*)
            return 0
            ;;
    esac

    # Find RC file
    local home="${HOME}"
    local rc_file=""
    case "$shell_type" in
        bash)
            if [ -f "$home/.bashrc" ]; then
                rc_file="$home/.bashrc"
            elif [ -f "$home/.bash_profile" ]; then
                rc_file="$home/.bash_profile"
            fi
            ;;
        zsh)
            if [ -f "$home/.zshrc" ]; then
                rc_file="$home/.zshrc"
            fi
            ;;
        fish)
            if [ -f "$home/.config/fish/config.fish" ]; then
                rc_file="$home/.config/fish/config.fish"
            fi
            ;;
    esac

    # Fallback for unknown shells
    if [ -z "$rc_file" ]; then
        if [ "$shell_type" = "bash" ]; then
            rc_file="$home/.bashrc"
        elif [ "$shell_type" = "zsh" ]; then
            rc_file="$home/.zshrc"
        else
            rc_file="$home/.profile"
        fi
    fi

    # Check if already configured in RC file
    if [ -f "$rc_file" ]; then
        if grep -q "git-courer" "$rc_file" 2>/dev/null; then
            return 0
        fi
    fi

    # Add to RC file
    info "Adding ${install_dir} to PATH in ${rc_file}..."
    if [ -w "$rc_file" ]; then
        echo "" >> "$rc_file"
        echo "# git-courer" >> "$rc_file"
        echo "export PATH=\"\$PATH:${install_dir}\"" >> "$rc_file"
        success "PATH updated in ${rc_file}"
        info "Restart your shell or run: source ${rc_file}"
    else
        warn "Cannot write to ${rc_file}"
        echo "  Add this to your shell config manually:"
        echo "    export PATH=\"\$PATH:${install_dir}\""
    fi
}

# ─── Install Binary ──────────────────────────────────────────────────────────

install_binary() {
    local os binary install_dir binary_path
    os=$(detect_os)
    binary="${BINARY_NAME}"
    [ "$os" = "windows" ] && binary="${BINARY_NAME}.exe"

    # Determine install directory
    case "$os" in
        linux)
            if [ -d "$HOME/.local/bin" ]; then
                install_dir="$HOME/.local/bin"
            elif [ -w "/usr/local/bin" ]; then
                install_dir="/usr/local/bin"
            else
                install_dir="$HOME/.local/bin"
                mkdir -p "$install_dir"
            fi
;;
        darwin|macos)
            if command -v brew &>/dev/null; then
                install_dir="$(brew --prefix)/bin"
            elif [ -d "$HOME/.local/bin" ]; then
                install_dir="$HOME/.local/bin"
            elif [ -w "/usr/local/bin" ]; then
                install_dir="/usr/local/bin"
            else
                install_dir="$HOME/.local/bin"
                mkdir -p "$install_dir"
            fi
            ;;
        windows)
            install_dir="${LOCALAPPDATA:-$HOME/.local}/bin"
            mkdir -p "$install_dir"
            ;;
        *)
            install_dir="$HOME/.local/bin"
            mkdir -p "$install_dir"
            ;;
    esac

    binary_path="${install_dir}/${binary}"

    echo -e "${BOLD}Installing ${binary}...${NC}"

    # Try local build first
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    REPO_ROOT="$(cd "${SCRIPT_DIR}/.." 2>/dev/null && pwd || echo "")"

    if [ -f "${REPO_ROOT}/${binary}" ]; then
        info "Moving from local build..."
        mv -f "${REPO_ROOT}/${binary}" "$binary_path"
        [ "$os" != "windows" ] && chmod +x "$binary_path"
        success "Installed to ${binary_path}"
        return
    fi

    # Download from GitHub releases
    local platform version url

    platform="$(detect_os)_$(detect_arch)"

    info "Downloading for ${platform}..."

    # Fetch release to get asset download URL
    local release_json
    release_json=$(curl -sf "${GITHUB_RELEASES}/latest" 2>/dev/null) || release_json=""

    if [ -z "$release_json" ]; then
        error "No releases found for ${REPO}."
        echo ""
        echo "Try another install method:"
        echo "  go install:  go install github.com/${REPO}/cmd@latest"
        echo "  From source: git clone https://github.com/${REPO}.git && cd ${BINARY_NAME} && go build -o ${binary} ./cmd/main.go"
        exit 1
    fi

    version=$(echo "$release_json" | grep -o '"tag_name": "[^"]*' | cut -d'"' -f4 || echo "latest")
    [ -z "$version" ] && version="latest"

    # Find matching asset (format: git-courer_1.0.1_darwin_amd64.tar.gz or git-courer-linux-amd64)
    local asset_url
    # Try with underscore first (GoReleaser default)
    asset_url=$(echo "$release_json" | grep -o "\"browser_download_url\": \"[^\"]*${platform}[^\"]*\"" | head -1 | cut -d'"' -f4) || asset_url=""
    
    # Fallback: Try with dash (Manual upload format)
    if [ -z "$asset_url" ]; then
        local dash_platform="${platform/_/-}"
        asset_url=$(echo "$release_json" | grep -o "\"browser_download_url\": \"[^\"]*${dash_platform}[^\"]*\"" | head -1 | cut -d'"' -f4) || asset_url=""
    fi

    # Ultra-Fallback: Search for OS and ARCH independently in the URL
    if [ -z "$asset_url" ]; then
        local os=$(detect_os)
        local arch=$(detect_arch)
        asset_url=$(echo "$release_json" | grep -o "\"browser_download_url\": \"[^\"]*${os}[^\"]*${arch}[^\"]*\"" | head -1 | cut -d'"' -f4) || asset_url=""
    fi

    if [ -z "$asset_url" ]; then
        error "No binary found for ${platform}"
        echo ""
        echo "Try another install method:"
        echo "  go install:  go install github.com/${REPO}/cmd@latest"
        echo "  From source: git clone https://github.com/${REPO}.git && cd ${BINARY_NAME} && go build -o ${binary} ./cmd/main.go"
        exit 1
    fi

    # Download to temp file
    local tmp_file="/tmp/${BINARY_NAME}_download"
    if ! curl -fsSL "$asset_url" -o "$tmp_file" 2>/dev/null; then
        error "Failed to download from GitHub releases."
        exit 1
    fi

    # Check if it's a tar.gz or a raw binary
    if file "$tmp_file" | grep -q "gzip compressed"; then
        info "Extracting archive..."
        local tmp_dir="/tmp/${BINARY_NAME}-extract"
        mkdir -p "$tmp_dir"
        tar -xzf "$tmp_file" -C "$tmp_dir"
        local extracted_binary
        extracted_binary=$(find "$tmp_dir" -type f -name "$binary" -o -name "${binary}.exe" 2>/dev/null | head -1)
        if [ -z "$extracted_binary" ]; then
            error "Binary not found in archive"
            rm -rf "$tmp_dir" "$tmp_file"
            exit 1
        fi
        mv -f "$extracted_binary" "$binary_path"
        rm -rf "$tmp_dir"
    else
        info "Installing raw binary..."
        mv -f "$tmp_file" "$binary_path"
    fi
    
    rm -f "$tmp_file"
    [ "$os" != "windows" ] && chmod +x "$binary_path"
    success "Installed ${version} to ${binary_path}"

    # Setup PATH
    setup_path "$install_dir"
}

# ─── Setup Agents ────────────────────────────────────────────────────────────

setup_agents() {
    local install_dir
    case "$(detect_os)" in
        linux|darwin)
            if [ -d "$HOME/.local/bin" ]; then
                install_dir="$HOME/.local/bin"
            elif [ -w "/usr/local/bin" ]; then
                install_dir="/usr/local/bin"
            else
                install_dir="$HOME/.local/bin"
            fi
            ;;
        windows)
            install_dir="${LOCALAPPDATA:-$HOME/.local}/bin"
            ;;
        *)
            install_dir="$HOME/.local/bin"
            ;;
    esac

    if [ -x "${install_dir}/${BINARY_NAME}" ]; then
        info "Setting up git-courer..."
        setup_path "$install_dir"
		"${install_dir}/${BINARY_NAME}" mcp setup
    else
        warn "git-courer not found in PATH."
        setup_path "$install_dir"
        echo ""
        echo "Run 'git-courer setup' manually after installation."
    fi
}

# ─── Dependency Check ────────────────────────────────────────────────────────

ensure_python3() {
    if command -v python3 &>/dev/null; then
        return 0
    fi

    local os
    os=$(detect_os)

    info "Installing python3 (required for safe JSON merge)..."

    case "$os" in
        linux)
            if command -v apt-get &>/dev/null; then
                sudo apt-get update -qq && sudo apt-get install -y -qq python3 2>/dev/null
            elif command -v dnf &>/dev/null; then
                sudo dnf install -y python3 2>/dev/null
            elif command -v pacman &>/dev/null; then
                sudo pacman -S --noconfirm python3 2>/dev/null
            elif command -v apk &>/dev/null; then
                sudo apk add python3 2>/dev/null
            else
                warn "Could not install python3 automatically. Install it manually."
                return 1
            fi
            ;;
        macos)
            if command -v brew &>/dev/null; then
                brew install python3 2>/dev/null
            else
                warn "Install Homebrew or python3 manually."
                return 1
            fi
            ;;
        windows)
            warn "Install python3 from https://python.org/downloads"
            return 1
            ;;
    esac

    if command -v python3 &>/dev/null; then
        success "python3 installed"
        return 0
    else
        warn "python3 installation failed — JSON merge will use best-effort bash fallback"
        return 1
    fi
}

# ─── Ollama Setup ─────────────────────────────────────────────────────────────

setup_ollama() {
    echo ""
    echo -e "${BOLD}Ollama Setup${NC}"
    echo "git-courer uses Ollama for AI-powered git operations."
    echo ""

    if command -v ollama &>/dev/null; then
        success "Ollama is already installed"
        if curl -sf http://localhost:11434/api/tags &>/dev/null; then
            success "Ollama is running"
        else
            warn "Ollama is installed but not running. Start with: ollama serve"
        fi
        local models
        models=$(ollama list 2>/dev/null | tail -n +2 | head -5 || echo "")
        if [ -n "$models" ]; then
            success "Models found:"
            echo "$models" | while read -r line; do echo "    $line"; done
        else
            warn "No models found. Pull one: ollama pull qwen3.5:0.6b"
        fi
    else
        warn "Ollama is not installed."
        echo ""
        echo "Install:  curl -fsSL https://ollama.com/install.sh | sh"
        echo "Model:    ollama pull qwen3.5:0.6b"
    fi
}

# ─── Main ─────────────────────────────────────────────────────────────────────

main() {
    local mode="full"

    for arg in "$@"; do
        case "$arg" in
            --binary)  mode="binary" ;;
            --agents)  mode="agents" ;;
            --ollama)  mode="ollama" ;;
            --help|-h)
                echo "Usage: install.sh [options]"
                echo ""
                echo "Options:"
                echo "  --binary    Install binary only"
                echo "  --agents    Configure detected agents only"
                echo "  --ollama    Setup Ollama only"
                echo "  --help, -h  Show this help"
                exit 0
                ;;
            *)
                error "Unknown option: $arg"
                exit 1
                ;;
        esac
    done

    echo -e "${BOLD}git-courer Installer${NC}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""

    if [ "$mode" = "full" ] || [ "$mode" = "binary" ]; then
        install_binary
        echo ""
    fi

    if [ "$mode" = "agents" ]; then
        setup_agents
        echo ""
    fi

    if [ "$mode" = "full" ] || [ "$mode" = "ollama" ]; then
        setup_ollama
        echo ""
    fi

    echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}INSTALL COMPLETE${NC}"
    echo ""
    echo "Next steps:"
    echo "  1. Restart your AI tool(s)"
    echo "  2. Try: git-courer setup  (in any git project)"
    echo ""
    echo "Docs: https://github.com/${REPO}"
}

main "$@"
