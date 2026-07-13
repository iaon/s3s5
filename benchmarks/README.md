# Benchmarks

This directory contains reproducible performance runs for the protocol v1
baseline. The benchmark harness exercises the real path:

```text
SOCKS5 client -> s3s5 client -> ObjectStore -> s3s5 server -> TCP target
```

The P0 benchmark work does not optimize protocol v1 and does not change key
layout, ACK behavior, close markers, chunking, polling, or Android
compatibility.

## Profiles

- `memory`: in-memory object store with no artificial delay. This is the fast
  CI-friendly profile for logical request counts.
- `simulated-s3`: memory store wrapped with deterministic operation delays and
  jitter. Delay values are configurable.
- `real-s3`: opt-in only. It uses the configured S3-compatible provider and a
  random benchmark prefix, then attempts cleanup.

## Commands

```sh
make perf-test
make perf-test-simulated
make perf-report
make perf-baseline
```

`perf-test` and `perf-test-simulated` write local timestamp-free results under
`benchmarks/results/local/`, which is ignored by Git.

Idle and chatty low-traffic scenarios are duration-driven. The default hold
time is 10 seconds for `one-byte-echo-after-idle`,
`small-chatty-writes`, `concurrent-idle-sessions`, and the idle/chatty parts of
`mixed-traffic`. Use `-idle-duration`, `-chatty-duration`, and
`-chatty-interval` for shorter smoke runs. `perf-test-simulated` intentionally
uses short durations so deterministic-delay smoke tests remain practical.

`perf-baseline` updates committed baseline files:

```text
benchmarks/results/baseline-v1-memory.json
benchmarks/results/baseline-v1-simulated-s3.json
benchmarks/reports/baseline-v1.md
```

Run a specific scenario:

```sh
go run ./cmd/s3s5-perf run -profile memory -scenario bulk-one-mib -out benchmarks/results/local/bulk.json
```

Run all memory scenarios with shorter idle/chatty holds:

```sh
go run ./cmd/s3s5-perf run -profile memory -idle-duration 200ms -chatty-duration 200ms -out benchmarks/results/local/quick.json
```

Generate a report from any JSON result:

```sh
PERF_JSON=benchmarks/results/local/bulk.json \
PERF_REPORT=benchmarks/results/local/bulk.md \
make perf-report
```

## Scenarios

- `one-byte-echo-active`
- `one-byte-echo-after-idle`
- `small-chatty-writes`
- `bulk-one-mib`
- `short-connections`
- `concurrent-idle-sessions`
- `mixed-traffic`

## Interpreting Results

Wall-clock timings from different machines are not directly comparable. Prefer
request counts, polling misses, chunks, bytes, and derived ratios when comparing
protocol changes.

Reliable comparison points include:

- object-store operation counts by key class and result
- data GET hit/miss counts
- ACK GET/PUT counts
- close HEAD counts
- open LIST counts
- operations per session
- operations per MiB
- sealed/plaintext size ratio

## Real S3

Real S3 runs are never used by normal CI. They require explicit opt-in:

```sh
S3S5_ACCESS_KEY_ID=... \
S3S5_SECRET_ACCESS_KEY=... \
S3S5_BUCKET=... \
go run ./cmd/s3s5-perf run -profile real-s3 -real-s3-opt-in -out benchmarks/results/local/real-s3.json
```

Provider presets match the rest of the project: `aws`, `yandex`, `minio`, and
`custom`. Credentials are read from environment variables and are never written
to JSON or Markdown output.
