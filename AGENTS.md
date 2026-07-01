# Repository Guidelines

## Project Structure & Module Organization

`s3s5` is a Go MVP for SOCKS5-over-S3. Command entrypoints live in `cmd/s3s5-client`, `cmd/s3s5-server`, and `cmd/s3s5-doctor`. Core packages live under `internal/`: `socks5`, `protocol`, `objectstore`, `objectstore/s3`, `objectstore/memory`, `tunnel`, `crypto`, `policy`, and `config`. Documentation is in `docs/`, operational scripts are in `scripts/`, and generated binaries go to `bin/`.

Treat `~/s3socks/File-Tunnel`, `~/s3socks/hev-socks5-server`, and `~/s3socks/socks5` as read-only references. Do not modify them or copy code without documenting license compatibility.

## Build, Test, and Development Commands

- `make build`: builds `bin/s3s5-client`, `bin/s3s5-server`, and `bin/s3s5-doctor`.
- `go test ./...`: runs all unit and memory-store integration tests. In restricted sandboxes this may require loopback socket permission.
- `make test`: wrapper for `go test ./...`.
- `make test-race`: runs the Go race detector.
- `make minio-up`, `make minio-test`, `make minio-down`: start MinIO, run the S3-compatible doctor smoke test, and stop services.
- `make yandex-s3-smoke`: run the doctor smoke test against Yandex Object Storage using `S3S5_PROVIDER=yandex`.
- `make clean`: removes generated build output.

## Coding Style & Naming Conventions

Use `gofmt` for all Go files. Keep package responsibilities narrow: protocol encoding belongs in `internal/protocol`, destination safety in `internal/policy`, object-store APIs in `internal/objectstore`, and stream orchestration in `internal/tunnel`. Prefer explicit names such as `OpenRequest`, `DataKey`, or `CheckTarget` over generic helpers.

## Testing Guidelines

Add tests with behavior changes. Keep unit tests next to packages as `*_test.go`. Cover IPv4, IPv6, domain targets, unsafe destination policy, crypto tamper/wrong-key failures, protocol key layout, ACK/window behavior, and memory-store SOCKS5 roundtrips. Normal tests must not require real AWS S3 credentials.

## Security & Configuration Tips

Do not log or commit AWS/S3 secrets. Real credentials must come from environment variables or untracked config. Server egress must be explicit: use `--allow-target` rules, or `--allow-unrestricted-egress` only in controlled environments. `--insecure-no-crypto` and `NoopCodec` are for local tests only.

## Commit & Pull Request Guidelines

Use short, imperative commit subjects, for example `Add memory tunnel test`. Pull requests should describe behavior changes, list validation commands, and call out security or compatibility impacts.
