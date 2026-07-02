#!/usr/bin/env sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT_DIR"

: "${AWS_ACCESS_KEY_ID:?set AWS_ACCESS_KEY_ID}"
: "${AWS_SECRET_ACCESS_KEY:?set AWS_SECRET_ACCESS_KEY}"
: "${S3S5_BUCKET:?set S3S5_BUCKET}"
: "${S3S5_PSK:?set S3S5_PSK to a long random test passphrase}"

REGION=${S3S5_REGION:-${AWS_REGION:-}}
if [ -z "${S3S5_PROVIDER:-}" ]; then
  case "$REGION:${S3S5_ENDPOINT:-}" in
    ru-central1*:*|*:storage.yandexcloud.net*) S3S5_PROVIDER=yandex ;;
    *) S3S5_PROVIDER=aws ;;
  esac
  export S3S5_PROVIDER
fi
if [ "$S3S5_PROVIDER" = "yandex" ] || [ "$S3S5_PROVIDER" = "yc" ]; then
  export S3S5_REGION=${S3S5_REGION:-ru-central1}
  REGION=$S3S5_REGION
else
  : "${REGION:?set AWS_REGION/S3S5_REGION, or use S3S5_PROVIDER=yandex}"
fi
export S3S5_PREFIX=${S3S5_PREFIX:-s3s5-real-$(date +%s)}

make build
./bin/s3s5-doctor \
  --provider "$S3S5_PROVIDER" \
  --bucket "$S3S5_BUCKET" \
  --prefix "$S3S5_PREFIX" \
  --region "$REGION" \
  --latency-rounds "${S3S5_LATENCY_ROUNDS:-3}" \
  --json

cat <<EOF
Doctor smoke test completed for prefix: $S3S5_PREFIX

To run a manual SOCKS5 smoke test, start the server and client in separate terminals:

./bin/s3s5-server --provider "$S3S5_PROVIDER" --bucket "\$S3S5_BUCKET" --prefix "$S3S5_PREFIX" --region "$REGION" --allow-target example.com:443
./bin/s3s5-client --provider "$S3S5_PROVIDER" --listen 127.0.0.1:1080 --bucket "\$S3S5_BUCKET" --prefix "$S3S5_PREFIX" --region "$REGION"
curl --socks5-hostname 127.0.0.1:1080 https://example.com/
EOF
