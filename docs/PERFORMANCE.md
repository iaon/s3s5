# Performance

`s3s5` is a correctness-first tunnel built on object storage. Expect it to be much slower than direct TCP.

## Why it is slow

Each session depends on repeated S3 operations:

- open request
- open result
- data chunk uploads
- data chunk reads
- ACK state updates
- close markers
- polling for progress

That means latency is driven by object-store round trips, not by raw socket throughput.

## Cost caveat

On real AWS S3, every request costs money.

The main cost multipliers are:

- shorter poll intervals
- smaller chunk sizes
- more active sessions
- more chatty traffic patterns

For that reason, the defaults are conservative and the docs favor controlled use over aggressive throughput tuning.

## Tuning levers

### Chunk size

Larger chunks reduce request count and improve throughput, but they increase latency for the first byte of a chunk and can make retries more expensive.

The current default is 64 KiB.

### Poll interval

Shorter polls improve responsiveness but increase request volume.

The implementation uses adaptive backoff:

- low delay while traffic is active
- longer delay while idle

### Window size

The ACK window caps how much data can be in flight without confirmation.

Larger windows improve throughput on higher-latency links.
Smaller windows reduce burst size and limit S3 object growth when a peer stalls.

The implementation batches ACK writes: receivers ACK roughly every half-window
and write a final ACK when they observe peer close. Writers also avoid reading
ACK state while they are still inside the initial window. For short transfers,
this keeps ACK objects from dominating the request count.

### Polling misses

When a peer waits for the next data object, missing-object polls are expected.
The reader checks close markers less often than data keys so a quiet period does
not double every poll into both `GET data` and `HEAD close`. Increasing
`--poll-min` and `--poll-max` reduces read request volume at the cost of slower
reaction time.

## Consistency assumptions

The protocol assumes plain S3-compatible behavior and tolerates missing objects while polling.

It does not require event notifications or strong coordination primitives. That keeps it portable across AWS S3 and MinIO-compatible endpoints.

## Practical use

This transport is best understood as a controlled tunnel for bulk-ish TCP streams, not as a low-latency interactive link.

It can be usable for:

- smoke testing
- controlled admin access
- coarse remote connectivity
- development environments

It is a poor fit for:

- latency-sensitive interactive sessions
- large fan-out session counts
- anonymous public relay use

## Benchmarking note

Use MinIO for local profiling when possible. It removes external network variance and avoids AWS request charges while still exercising the same S3-compatible code paths.

## Baseline Before v1 Optimizations

P0 performance work records the current protocol v1 baseline without changing
wire behavior. The benchmark harness exercises the full local path:

```text
SOCKS5 client -> s3s5 client -> ObjectStore -> s3s5 server -> TCP target
```

Run the fast memory profile:

```sh
make perf-test
```

Run a simulated object-store profile with deterministic delays:

```sh
make perf-test-simulated
```

Idle and chatty low-traffic scenarios are held by time instead of by a fixed
small request count. The default is 10 seconds for idle holds and chatty writes;
override it with `-idle-duration`, `-chatty-duration`, and `-chatty-interval`
when a shorter smoke run is needed:

```sh
go run ./cmd/s3s5-perf run -profile memory -idle-duration 200ms -chatty-duration 200ms -out benchmarks/results/local/quick.json
```

Update committed baseline files explicitly:

```sh
make perf-baseline
```

Baseline files are stored under:

```text
benchmarks/results/baseline-v1-memory.json
benchmarks/results/baseline-v1-simulated-s3.json
benchmarks/reports/baseline-v1.md
```

Local ad hoc results go under `benchmarks/results/local/`, which is ignored by
Git.

The stable comparison points are request counts, polling misses, key classes,
chunks, bytes, and derived ratios. Wall-clock timings from different machines
or CI runs should not be compared directly.

The generated report includes separate counts for:

- data GET hits and misses
- ACK GET and PUT operations
- close HEAD checks
- open LIST polling
- operations per session
- operations per MiB
- sealed/plaintext size ratio

Real S3 runs are opt-in only and require credentials in environment variables:

```sh
S3S5_ACCESS_KEY_ID=... \
S3S5_SECRET_ACCESS_KEY=... \
S3S5_BUCKET=... \
go run ./cmd/s3s5-perf run -profile real-s3 -real-s3-opt-in -out benchmarks/results/local/real-s3.json
```

The real-S3 profile uses a random benchmark prefix and attempts cleanup at the
end of each scenario. It never writes credentials, PSK values, session IDs,
target hostnames, or full object keys to JSON or Markdown output.
