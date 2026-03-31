#!/bin/sh
set -e

VERSION="${1:-dev}"
DIST="dist"

rm -rf "$DIST"
mkdir -p "$DIST"

build() {
    goos="$1"
    goarch="$2"
    name="$3"
    ext=""
    if [ "$goos" = "windows" ]; then
        ext=".exe"
    fi
    echo "Building $name..."
    GOOS="$goos" GOARCH="$goarch" go build -ldflags "-s -w" -o "$DIST/${name}${ext}" .
}

build darwin  arm64 butler-macos-apple-silicon
build darwin  amd64 butler-macos-intel
build linux   amd64 butler-linux-x64
build linux   arm64 butler-linux-arm64
build windows amd64 butler-windows-x64
build windows arm64 butler-windows-arm64

echo ""
echo "Binaries in $DIST/:"
ls -lh "$DIST/"
