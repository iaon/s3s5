#!/usr/bin/env sh
set -eu

docker compose up -d minio minio-init
