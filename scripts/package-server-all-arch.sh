#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FORMAT="${1:-all}"

case "$FORMAT" in
  all|deb|rpm) ;;
  *) echo "usage: $0 [all|deb|rpm]" >&2; exit 2 ;;
esac

mkdir -p "$ROOT_DIR/dist/packages"
rm -f "$ROOT_DIR"/dist/packages/s3s5-server_*.deb "$ROOT_DIR"/dist/packages/s3s5-server-*.rpm

S3S5_PACKAGE_CLEAN=0 GOARCH=amd64 "$ROOT_DIR/scripts/package-server-docker.sh" "$FORMAT"
S3S5_PACKAGE_CLEAN=0 GOARCH=arm64 "$ROOT_DIR/scripts/package-server-docker.sh" "$FORMAT"
