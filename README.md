# s3s5

`s3s5` is a SOCKS5-over-S3 MVP for Linux.

The current shape is:

`SOCKS5 client -> S3-compatible object store -> TCP server`

The code is written in Go and is structured so the client-side core can later be ported to Android without dragging Linux-only assumptions into the protocol, crypto, or policy layers.

## Status

This repository is still an MVP. The implementation intentionally favors correctness, explicit policy, and observability over speed.

Current limits:

- Linux client and server only
- SOCKS5 `CONNECT` only
- no UDP ASSOCIATE
- no SOCKS5 BIND
- `s3s5-server` requires `--allow-target` or `--allow-unrestricted-egress`
- polling-based transport, so latency is inherently high
- S3 request volume directly affects cost
- `--insecure-no-crypto` is for local development only

## What it uses

- `cmd/s3s5-client`
- `cmd/s3s5-server`
- `cmd/s3s5-doctor`
- `internal/socks5`, `internal/protocol`, `internal/crypto`, `internal/policy`, `internal/tunnel`

Design notes:

- protocol objects are versioned and key names include `v1`
- payloads are encrypted by default with PSK-derived AES-256-GCM
- the server rejects unsafe destinations by default
- IPv6 is supported in the protocol and policy layer

See:

- [docs/ADR-0001-stack.md](docs/ADR-0001-stack.md)
- [docs/PROTOCOL.md](docs/PROTOCOL.md)
- [docs/SECURITY.md](docs/SECURITY.md)
- [docs/PERFORMANCE.md](docs/PERFORMANCE.md)
- [docs/ANDROID_PORTING.md](docs/ANDROID_PORTING.md)
- [docs/reference-survey.md](docs/reference-survey.md)

## Quick start

Set a pre-shared key and the S3 variables first:

```bash
export AWS_ACCESS_KEY_ID=...
export AWS_SECRET_ACCESS_KEY=...
export AWS_REGION=us-east-1
export S3S5_BUCKET=your-bucket
export S3S5_PREFIX=s3s5-dev
export S3S5_PSK='use-a-long-random-passphrase'
```

Run the server:

```bash
./bin/s3s5-server --bucket "$S3S5_BUCKET" --prefix "$S3S5_PREFIX" --region "$AWS_REGION" --allow-target example.com:443
```

Run the client:

```bash
./bin/s3s5-client --listen 127.0.0.1:1080 --bucket "$S3S5_BUCKET" --prefix "$S3S5_PREFIX" --region "$AWS_REGION"
```

Test through SOCKS5:

```bash
curl --socks5-hostname 127.0.0.1:1080 https://example.com/
```

## MinIO example

MinIO is useful for local development because it avoids AWS request charges and keeps latency predictable.

Typical environment:

```bash
export AWS_ACCESS_KEY_ID=minioadmin
export AWS_SECRET_ACCESS_KEY=minioadmin
export AWS_REGION=us-east-1
export S3S5_BUCKET=s3s5
export S3S5_PREFIX=s3s5-dev
export S3S5_PSK='use-a-long-random-passphrase'
export S3S5_ENDPOINT=http://127.0.0.1:9000
export S3S5_FORCE_PATH_STYLE=true
```

With MinIO running locally, start the server and client with the same bucket, prefix, and PSK. The `doctor` command should then verify put/head/get/list/delete behavior against the same endpoint.
When the server is expected to forward traffic, include an explicit allow policy such as `--allow-target example.com:443`; use `--allow-unrestricted-egress` only in fully controlled environments.

To clear a test prefix with the doctor CLI, use `--cleanup-prefix`.

## Real S3 caveats

Every polling round, control object, ACK object, and data chunk is an S3 request. That means:

- more aggressive polling increases request cost
- smaller chunks increase request count
- tighter windows can reduce burst size but also reduce throughput
- latency is dominated by object-store round trips, not by TCP socket latency

This is suitable for bulk-ish TCP traffic and controlled environments. It is not a low-latency replacement for SSH tunnels, VPNs, or direct TCP relays.

## Testing

`go test ./...` may need permission to bind loopback sockets in restricted sandboxes because the integration tests start local listeners.

## IPv6

IPv6 is in scope for the protocol and policy code:

- SOCKS5 parsing accepts IPv6 targets
- protocol targets preserve IPv6 addresses and domains
- policy checks understand IPv6 CIDRs
- unsafe IPv6 ranges are denied by default

Automated e2e tests do not need public IPv6 connectivity. The code should still behave correctly when the local network and the object-store endpoint support IPv6.

## Reference survey

The reference repos under `~/s3socks/` were surveyed before implementation. The survey notes are in [docs/reference-survey.md](docs/reference-survey.md).

The short version: all three reference repos appear MIT licensed, and no code was copied into this repo.
