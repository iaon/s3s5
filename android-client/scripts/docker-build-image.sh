#!/usr/bin/env sh
set -eu

cd "$(dirname "$0")/../.."

IMAGE="${S3S5_ANDROID_DOCKER_IMAGE:-s3s5-android-build:36}"

docker build \
  -f android-client/Dockerfile \
  -t "$IMAGE" \
  android-client
