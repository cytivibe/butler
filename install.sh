#!/bin/sh
set -e

# Butler installer — detects OS/arch, downloads the right binary from GitHub Releases.
# Usage: curl -fsSL https://raw.githubusercontent.com/cytivibe/butler/main/install.sh | sh

REPO="cytivibe/butler"
INSTALL_DIR="/usr/local/bin"

# Detect OS
case "$(uname -s)" in
    Darwin)  OS="darwin" ;;
    Linux)   OS="linux" ;;
    MINGW*|MSYS*|CYGWIN*) OS="windows" ;;
    *) echo "Unsupported OS: $(uname -s)" >&2; exit 1 ;;
esac

# Detect architecture
case "$(uname -m)" in
    x86_64|amd64)  ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

# Get latest release tag
LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
if [ -z "$LATEST" ]; then
    echo "Error: could not determine latest release" >&2
    exit 1
fi

BINARY="butler-${OS}-${ARCH}"
if [ "$OS" = "windows" ]; then
    BINARY="${BINARY}.exe"
fi

URL="https://github.com/${REPO}/releases/download/${LATEST}/${BINARY}"

echo "Downloading butler ${LATEST} for ${OS}/${ARCH}..."
curl -fsSL -o butler "$URL"
chmod +x butler

# Install
if [ -w "$INSTALL_DIR" ]; then
    mv butler "$INSTALL_DIR/butler"
else
    echo "Installing to ${INSTALL_DIR} (requires sudo)..."
    sudo mv butler "$INSTALL_DIR/butler"
fi

echo "butler ${LATEST} installed to ${INSTALL_DIR}/butler"
