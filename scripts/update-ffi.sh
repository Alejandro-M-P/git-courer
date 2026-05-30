#!/bin/bash
set -e

# Configuration
REPO="kreuzberg-dev/tree-sitter-language-pack"
EXT_LIB_DIR="internal/infra/chunkers/ext_lib"
TARGET_BASE="target/release"

echo "🔍 Fetching latest release tag from $REPO..."
LATEST_TAG=$(gh api repos/$REPO/releases --jq '.[0].tag_name')
echo "✅ Latest version found: $LATEST_TAG"

mkdir -p "$TARGET_BASE/linux_amd64"
mkdir -p "$TARGET_BASE/darwin_amd64"
mkdir -p "$TARGET_BASE/darwin_arm64"
mkdir -p "$TARGET_BASE/windows_amd64"

download_and_extract() {
    local platform=$1
    local arch=$2
    local asset_suffix=$3
    local dest_dir=$4
    local is_win=$5

    echo "📥 Downloading Go package for $platform ($arch)..."
    local asset_name="tree-sitter-language-pack-go-$LATEST_TAG-$asset_suffix.tar.gz"
    local tmp_dir="temp_$platform"
    mkdir -p "$tmp_dir"
    
    gh release download "$LATEST_TAG" -R "$REPO" -p "$asset_name" -D "$tmp_dir"
    
    echo "📦 Extracting..."
    tar -xzf "$tmp_dir/$asset_name" -C "$tmp_dir"
    
    local extracted_folder="tree-sitter-language-pack-go-${LATEST_TAG}-$asset_suffix"
    
    # Copy library
    if [ "$is_win" = "true" ]; then
        # Check windows extension in the go package
        cp "$tmp_dir/$extracted_folder/lib/libts_pack_core_ffi.a" "$dest_dir/" 2>/dev/null || \
        cp "$tmp_dir/$extracted_folder/lib/ts_pack_core_ffi.lib" "$dest_dir/libts_pack_core_ffi.a"
    else
        cp "$tmp_dir/$extracted_folder/lib/libts_pack_core_ffi.a" "$dest_dir/"
    fi

    # Update header (only once)
    if [ "$platform" = "linux" ]; then
        echo "📄 Updating C header file..."
        cp "$tmp_dir/$extracted_folder/include/ts_pack.h" "$EXT_LIB_DIR/include/"
    fi

    rm -rf "$tmp_dir"
    echo "✅ Done for $platform."
}

# Run downloads
download_and_extract "linux" "amd64" "linux-x86_64" "$TARGET_BASE/linux_amd64" "false"
download_and_extract "darwin" "amd64" "macos-x86_64" "$TARGET_BASE/darwin_amd64" "false"
download_and_extract "darwin" "arm64" "macos-arm64" "$TARGET_BASE/darwin_arm64" "false"
download_and_extract "windows" "amd64" "windows-x86_64" "$TARGET_BASE/windows_amd64" "true"

echo "✨ All FFI libraries updated to $LATEST_TAG"
