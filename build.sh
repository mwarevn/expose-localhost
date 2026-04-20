#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

OUT_DIR="dist"
BIN_NAME="expose-localhost"
LDFLAGS="-s -w"

TARGETS=(
  "linux/amd64"
  "linux/386"
  "linux/arm64"
  "linux/arm"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
  "windows/386"
)

rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"

echo "→ go mod tidy"
go mod tidy

for target in "${TARGETS[@]}"; do
  os="${target%/*}"
  arch="${target#*/}"
  ext=""
  [ "$os" = "windows" ] && ext=".exe"
  out="$OUT_DIR/${BIN_NAME}-${os}-${arch}${ext}"

  printf "→ building %-32s" "$out"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go build -trimpath -ldflags="$LDFLAGS" -o "$out" .
  size=$(du -h "$out" | cut -f1)
  echo "  ($size)"
done

echo
echo "✓ built $(ls -1 "$OUT_DIR" | wc -l | tr -d ' ') binaries in $OUT_DIR/"
