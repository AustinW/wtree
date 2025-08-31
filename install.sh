#!/bin/bash
# WTree Installation Script
# Usage: curl -sSL https://raw.githubusercontent.com/awhite/wtree/main/install.sh | bash

set -e

# Configuration
REPO="awhite/wtree"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
TMPDIR="$(mktemp -d)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Detect OS and architecture
detect_platform() {
    local os arch
    
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    arch=$(uname -m)
    
    case $arch in
        x86_64) arch="x86_64" ;;
        arm64|aarch64) arch="arm64" ;;
        armv7l) arch="armv7" ;;
        i386|i686) arch="i386" ;;
        *) 
            log_error "Unsupported architecture: $arch"
            exit 1
            ;;
    esac
    
    case $os in
        darwin) os="Darwin" ;;
        linux) os="Linux" ;;
        windows*|mingw*|cygwin*) 
            os="Windows"
            arch="${arch}.exe"
            ;;
        *)
            log_error "Unsupported operating system: $os"
            exit 1
            ;;
    esac
    
    echo "${os}_${arch}"
}

# Get latest release version
get_latest_version() {
    curl -s "https://api.github.com/repos/$REPO/releases/latest" | \
        grep '"tag_name":' | \
        sed -E 's/.*"([^"]+)".*/\1/'
}

# Download and install
install_wtree() {
    log_info "Installing WTree..."
    
    local platform version download_url filename
    platform=$(detect_platform)
    version=$(get_latest_version)
    
    if [ -z "$version" ]; then
        log_error "Failed to get latest version"
        exit 1
    fi
    
    log_info "Latest version: $version"
    log_info "Platform: $platform"
    
    filename="wtree_${platform}.tar.gz"
    if [[ "$platform" == *"Windows"* ]]; then
        filename="wtree_${platform}.zip"
    fi
    
    download_url="https://github.com/$REPO/releases/download/$version/$filename"
    
    log_info "Downloading from: $download_url"
    
    # Download
    cd "$TMPDIR"
    if ! curl -sL "$download_url" -o "$filename"; then
        log_error "Failed to download WTree"
        exit 1
    fi
    
    # Extract
    log_info "Extracting..."
    if [[ "$filename" == *.zip ]]; then
        unzip -q "$filename"
    else
        tar -xzf "$filename"
    fi
    
    # Find binary
    local binary_path
    binary_path=$(find . -name "wtree" -type f | head -1)
    
    if [ -z "$binary_path" ]; then
        log_error "Binary not found in archive"
        exit 1
    fi
    
    # Install
    log_info "Installing to $INSTALL_DIR..."
    
    if [ ! -w "$INSTALL_DIR" ]; then
        log_info "Installing with sudo..."
        sudo install "$binary_path" "$INSTALL_DIR/wtree"
    else
        install "$binary_path" "$INSTALL_DIR/wtree"
    fi
    
    # Cleanup
    cd /
    rm -rf "$TMPDIR"
    
    # Verify installation
    if command -v wtree >/dev/null 2>&1; then
        log_info "WTree installed successfully!"
        log_info "Version: $(wtree --version)"
        log_info ""
        log_info "Get started with: wtree --help"
        log_info "Enable shell completion: wtree completion bash"
    else
        log_warn "WTree was installed but may not be in your PATH"
        log_warn "Try adding $INSTALL_DIR to your PATH, or run: $INSTALL_DIR/wtree"
    fi
}

# Main
main() {
    log_info "WTree Installer"
    log_info "==============="
    
    # Check dependencies
    for cmd in curl tar; do
        if ! command -v "$cmd" >/dev/null 2>&1; then
            log_error "Required command not found: $cmd"
            exit 1
        fi
    done
    
    install_wtree
}

# Run if executed directly
if [ "${BASH_SOURCE[0]}" == "${0}" ]; then
    main "$@"
fi