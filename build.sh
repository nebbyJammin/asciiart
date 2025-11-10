#!/bin/bash
set -e

mkdir -p dist

for os in linux windows darwin; do
  for arch in amd64 arm64; do
    outname="./dist/asciiart-${os}-${arch}"
    [ "$os" = "windows" ] && outname+=".exe"
    echo "Building $outname..."
    CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -ldflags="-s -w" -o $outname ./cmd/asciiart/.
  done
done
