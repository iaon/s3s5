#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${VERSION:-$(cat "$ROOT_DIR/VERSION")}"
COMMIT="${COMMIT:-$(git -C "$ROOT_DIR" rev-parse --short=12 HEAD 2>/dev/null || echo unknown)}"
DATE="${DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
DIST_DIR="$ROOT_DIR/dist/release"
LDFLAGS="-s -w -X s3s5/internal/version.Version=${VERSION} -X s3s5/internal/version.Commit=${COMMIT} -X s3s5/internal/version.Date=${DATE}"

export GOCACHE="${GOCACHE:-$ROOT_DIR/.cache/go-build}"

build_archive() {
  local goos="$1"
  local goarch="$2"
  local name="s3s5_${VERSION}_${goos}_${goarch}"
  local out="$DIST_DIR/$name"

  rm -rf "$out"
  install -d "$out"

  (
    cd "$ROOT_DIR"
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath -ldflags="$LDFLAGS" -o "$out/s3s5-client" ./cmd/s3s5-client
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath -ldflags="$LDFLAGS" -o "$out/s3s5-server" ./cmd/s3s5-server
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath -ldflags="$LDFLAGS" -o "$out/s3s5-doctor" ./cmd/s3s5-doctor
  )

  install -m 0644 "$ROOT_DIR/README.md" "$out/README.md"
  install -m 0644 "$ROOT_DIR/LICENSE" "$out/LICENSE"
  install -m 0644 "$ROOT_DIR/CHANGELOG.md" "$out/CHANGELOG.md"

  tar -C "$DIST_DIR" -czf "$DIST_DIR/$name.tar.gz" "$name"
  rm -rf "$out"
}

rm -rf "$DIST_DIR"
install -d "$DIST_DIR"

build_archive linux amd64
build_archive linux arm64

find "$DIST_DIR" -maxdepth 1 -type f -name '*.tar.gz' -print | sort
