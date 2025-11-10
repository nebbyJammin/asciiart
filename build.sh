#!/bin/bash
set -euo pipefail

VERSION="${1:-}"
if [[ -z "$VERSION" ]]; then
  echo "Usage: $0 <version>"
  echo "Example: $0 v1.0.2"
  exit 1
fi

mkdir -p dist

for os in linux windows darwin; do
  for arch in amd64 arm64; do
    outname="./dist/asciiart-${VERSION}-${os}-${arch}"
    [[ "$os" == "windows" ]] && outname+=".exe"

    echo "Building $outname..."
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
      go build -ldflags="-s -w" -o "$outname" ./cmd/asciiart/.
  done
done

echo "All builds completed. Binaries are in ./dist/"
