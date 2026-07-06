#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="${S3S5_PACKAGE_DOCKER_IMAGE:-s3s5-package-build:bookworm}"
FORMAT="${1:-all}"

if [[ "$FORMAT" != "all" && "$FORMAT" != "deb" && "$FORMAT" != "rpm" && "$FORMAT" != "--image-only" ]]; then
  echo "usage: $0 [all|deb|rpm|--image-only]" >&2
  exit 2
fi

if [[ "${S3S5_PACKAGE_DOCKER_REBUILD:-0}" == "1" ]] || ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
  docker build -f "$ROOT_DIR/packaging/docker/Dockerfile" -t "$IMAGE" "$ROOT_DIR"
fi

if [[ "$FORMAT" == "--image-only" ]]; then
  echo "$IMAGE"
  exit 0
fi

mkdir -p "$ROOT_DIR/.cache/go-build" "$ROOT_DIR/dist/packages" "$ROOT_DIR/tmp"

docker run --rm \
  --user "$(id -u):$(id -g)" \
  -e HOME=/tmp \
  -e GOCACHE=/work/.cache/go-build \
  -e VERSION="${VERSION:-0.1.0}" \
  -e RELEASE="${RELEASE:-1}" \
  -e GOARCH="${GOARCH:-amd64}" \
  -v "$ROOT_DIR:/work" \
  -w /work \
  "$IMAGE" \
  ./scripts/package-server.sh "$FORMAT"
