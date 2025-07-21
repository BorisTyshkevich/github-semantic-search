#!/usr/bin/env bash
set -euo pipefail

OUT_DIR="bin"
GO_MAIN="./cmd/ghsearch"
LDFLAGS="-s -w"

mkdir -p "$OUT_DIR"

echo "🔨 linux/amd64 ..."
GOOS=linux  GOARCH=amd64 CGO_ENABLED=0 \
  go build -ldflags="$LDFLAGS" -o "$OUT_DIR/ghsearch-linux-amd64" "$GO_MAIN"

echo "🔨 darwin/arm64 ..."
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 \
  go build -ldflags="$LDFLAGS" -o "$OUT_DIR/ghsearch-darwin-arm64" "$GO_MAIN"