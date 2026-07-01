#!/usr/bin/env sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT_DIR"

export AWS_ACCESS_KEY_ID=minioadmin
export AWS_SECRET_ACCESS_KEY=minioadmin
export AWS_REGION=us-east-1
export S3S5_BUCKET=s3s5-test
export S3S5_PREFIX=s3s5-minio-$(date +%s)
export S3S5_ENDPOINT=http://127.0.0.1:9000
export S3S5_FORCE_PATH_STYLE=true
export S3S5_PSK=${S3S5_PSK:-"local-minio-test-passphrase-change-me"}

make build
./bin/s3s5-doctor \
  --bucket "$S3S5_BUCKET" \
  --prefix "$S3S5_PREFIX" \
  --region "$AWS_REGION" \
  --endpoint "$S3S5_ENDPOINT" \
  --force-path-style \
  --latency-rounds 3 \
  --json
