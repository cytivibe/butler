#!/bin/sh
set -e

# Butler installer - detects OS/arch, downloads the right binary from GitHub Releases.
# Usage: curl -fsSL https://raw.githubusercontent.com/cytivibe/butler/main/install.sh | sh

REPO="cytivibe/butler"
INSTALL_DIR="/usr/local/bin"

# Detect platform name
case "$(uname -s)" in
    Darwin)
        case "$(uname -m)" in
            arm64|aarch64) BINARY="butler-macos-apple-silicon" ;;
            x86_64|amd64)  BINARY="butler-macos-intel" ;;
            *) echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
        esac
        ;;
    Linux)
        case "$(uname -m)" in
            x86_64|amd64)  BINARY="butler-linux-x64" ;;
            arm64|aarch64) BINARY="butler-linux-arm64" ;;
            *) echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
        esac
        ;;
    *) echo "Unsupported OS: $(uname -s). For Windows, use install.ps1." >&2; exit 1 ;;
esac

# Get latest release tag
LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
if [ -z "$LATEST" ]; then
    echo "Error: could not determine latest release" >&2
    exit 1
fi

URL="https://github.com/${REPO}/releases/download/${LATEST}/${BINARY}"

echo "Downloading butler ${LATEST} (${BINARY})..."
curl -fL --progress-bar -o butler "$URL"
chmod +x butler

# Install
if [ -w "$INSTALL_DIR" ]; then
    mv butler "$INSTALL_DIR/butler"
else
    echo "Installing to ${INSTALL_DIR} (requires sudo)..."
    sudo mv butler "$INSTALL_DIR/butler"
fi

echo "butler ${LATEST} installed to ${INSTALL_DIR}/butler"
