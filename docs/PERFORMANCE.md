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
