#!/bin/sh
# One-line installer for Inguma
# Usage: curl -sSL https://inguma.dev/install.sh | sh

set -e

REPO="enekos/inguma"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Detect OS
os=""
case "$(uname -s)" in
    Linux*)     os=linux;;
    Darwin*)    os=darwin;;
    CYGWIN*|MINGW*|MSYS*) os=windows;;
    *)          echo "Unsupported OS: $(uname -s)"; exit 1;;
esac

# Detect architecture
arch=""
case "$(uname -m)" in
    x86_64|amd64)   arch=amd64;;
    arm64|aarch64)  arch=arm64;;
    *)              echo "Unsupported architecture: $(uname -m)"; exit 1;;
esac

if [ "$os" = "windows" ]; then
    echo "Windows is not supported by this installer yet. Please download the release manually."
    exit 1
fi

# Find latest release
api_url="https://api.github.com/repos/${REPO}/releases/latest"
echo "Fetching latest release info..."

download_url=$(curl -sSL "$api_url" | grep -o '"browser_download_url": "[^"]*' | grep "_${os}_${arch}\.tar\.gz" | head -n 1 | sed 's/"browser_download_url": "//')

if [ -z "$download_url" ]; then
    echo "Could not find release asset for ${os}_${arch}"
    echo "Please check https://github.com/${REPO}/releases manually."
    exit 1
fi

# Download
tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

echo "Downloading..."
curl -sSL -o "$tmpdir/inguma.tar.gz" "$download_url"

# Extract
echo "Extracting..."
tar xzf "$tmpdir/inguma.tar.gz" -C "$tmpdir"

# Install
if [ -w "$INSTALL_DIR" ]; then
    mv "$tmpdir/inguma" "$INSTALL_DIR/inguma"
else
    echo "Installing to $INSTALL_DIR requires sudo..."
    sudo mv "$tmpdir/inguma" "$INSTALL_DIR/inguma"
fi

echo "Inguma installed to ${INSTALL_DIR}/inguma"
"${INSTALL_DIR}/inguma" --help || true
