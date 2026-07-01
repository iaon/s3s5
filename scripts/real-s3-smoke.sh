#!/usr/bin/env sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT_DIR"

: "${AWS_ACCESS_KEY_ID:?set AWS_ACCESS_KEY_ID}"
: "${AWS_SECRET_ACCESS_KEY:?set AWS_SECRET_ACCESS_KEY}"
: "${AWS_REGION:?set AWS_REGION}"
: "${S3S5_BUCKET:?set S3S5_BUCKET}"
: "${S3S5_PSK:?set S3S5_PSK to a long random test passphrase}"

export S3S5_PREFIX=${S3S5_PREFIX:-s3s5-real-$(date +%s)}

make build
./bin/s3s5-doctor \
  --bucket "$S3S5_BUCKET" \
  --prefix "$S3S5_PREFIX" \
  --region "$AWS_REGION" \
  --latency-rounds "${S3S5_LATENCY_ROUNDS:-3}" \
  --json

cat <<EOF
Doctor smoke test completed for prefix: $S3S5_PREFIX

To run a manual SOCKS5 smoke test, start the server and client in separate terminals:

./bin/s3s5-server --bucket "\$S3S5_BUCKET" --prefix "$S3S5_PREFIX" --region "\$AWS_REGION" --allow-target example.com:443
./bin/s3s5-client --listen 127.0.0.1:1080 --bucket "\$S3S5_BUCKET" --prefix "$S3S5_PREFIX" --region "\$AWS_REGION"
curl --socks5-hostname 127.0.0.1:1080 https://example.com/
EOF
