#!/usr/bin/env sh
set -eu

cd "$(dirname "$0")/../.."

IMAGE="${S3S5_ANDROID_DOCKER_IMAGE:-s3s5-android-build:36}"

if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
  ./android-client/scripts/docker-build-image.sh
fi

mkdir -p .cache/gradle .cache/android-sdk

docker run --rm \
  -v "$PWD:/work" \
  -v "$PWD/.cache/gradle:/root/.gradle" \
  -v "$PWD/.cache/android-sdk:/root/.android" \
  -w /work/android-client \
  "$IMAGE" \
  ./gradlew "$@"
