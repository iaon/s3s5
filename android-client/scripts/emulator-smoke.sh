#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")/.."
./scripts/install-debug.sh
./scripts/adb-forward.sh 1080 1080
echo "Configure and start s3s5 in the emulator, then run:"
echo "curl --socks5-hostname 127.0.0.1:1080 https://example.com/"
