#!/bin/sh
set -e

VERSION="${1:-dev}"
DIST="dist"

rm -rf "$DIST"
mkdir -p "$DIST"

platforms="darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64 windows/arm64"

for platform in $platforms; do
    os="${platform%/*}"
    arch="${platform#*/}"
    output="$DIST/butler-${os}-${arch}"
    if [ "$os" = "windows" ]; then
        output="${output}.exe"
    fi
    echo "Building $os/$arch..."
    GOOS="$os" GOARCH="$arch" go build -ldflags "-s -w" -o "$output" .
done

echo ""
echo "Binaries in $DIST/:"
ls -lh "$DIST/"
