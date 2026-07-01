#!/usr/bin/env sh
set -eu

: "${AWS_ACCESS_KEY_ID:?set AWS_ACCESS_KEY_ID to your Yandex static access key ID}"
: "${AWS_SECRET_ACCESS_KEY:?set AWS_SECRET_ACCESS_KEY to your Yandex static secret key}"
: "${S3S5_BUCKET:?set S3S5_BUCKET}"
: "${S3S5_PSK:?set S3S5_PSK to a long random test passphrase}"

export S3S5_PROVIDER=yandex
export S3S5_REGION=${S3S5_REGION:-ru-central1}

exec "$(dirname -- "$0")/real-s3-smoke.sh"
