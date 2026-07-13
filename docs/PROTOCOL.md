# S3 Mailbox Protocol v1

This repository uses an S3-object mailbox protocol for control messages, chunk data, acknowledgements, and session close signals.

The current implementation uses these key families:

```text
<prefix>/v1/open/<session-id>.json
<prefix>/v1/open-result/<session-id>.json
<prefix>/v1/data/c2s/<session-id>/<seq>.bin
<prefix>/v1/data/s2c/<session-id>/<seq>.bin
<prefix>/v1/ack/c2s/<session-id>.json
<prefix>/v1/ack/s2c/<session-id>.json
<prefix>/v1/close/client/<session-id>.json
<prefix>/v1/close/server/<session-id>.json
<prefix>/v1/heartbeat/client/<session-id>.json
<prefix>/v1/heartbeat/server/<session-id>.json
```

The implementation currently uses the `v1` prefix and zero-padded sequence numbers.

## Session model

- `session-id` is random and unpredictable
- one SOCKS5 `CONNECT` request maps to one session
- the client writes an open request object
- the server polls for open requests, validates policy, and writes an open-result object
- once open succeeds, both sides stream chunk objects until close or timeout

## Message types

The current control structs are:

- `OpenRequest`
- `OpenResult`
- `Ack`
- `Close`
- `Heartbeat`

The code keeps these messages versioned and JSON encoded before they are sealed into S3 objects.

## Sequencing

Data chunks are numbered monotonically from `0`.

Chunk keys use 20-digit zero-padded decimal sequence numbers so lexical ordering matches numeric ordering.

The receiver fetches the next expected chunk by exact key rather than re-listing the whole prefix on every poll.

## Backpressure

The protocol uses ACK objects to prevent unbounded object creation when one side outruns the other.

The default window is conservative. The sender waits until:

`seq < nextAck + windowChunks`

before creating the next chunk.

That keeps the protocol simple but still bounds in-flight data in S3.

## Close and idle behavior

Each side may write a close object when it finishes or encounters an error.

The peer stops waiting when it observes the corresponding close marker.

Idle timeout is enforced while waiting for data or control progression. If the peer stops moving and no close marker appears, the session expires.

## Encryption

Payloads are encrypted by default with PSK-derived AES-256-GCM.

The current envelope format is JSON with:

- version
- algorithm name
- random nonce
- base64 ciphertext

Associated data binds the ciphertext to:

- protocol version
- object type
- session-id
- direction
- sequence number

That means a chunk cannot be replayed or retargeted to a different session or direction without failing authentication.

`--insecure-no-crypto` disables this protection for local development only.

Implementation note: the tunnel layer requires an explicit codec instance. The CLI passes the PSK-backed codec by default, and `NoopCodec` is reserved for local insecure tests.

## Polling and consistency

The implementation does not require S3 event notifications, SQS, Lambda, or WebSockets.

It assumes plain S3-compatible `PUT`, `GET`, `HEAD`, `LIST`, and `DELETE`.

Polling is adaptive:

- low delay while active
- higher delay while idle

The code also tolerates missing objects during polling, which is important because the two peers will not observe the same timeline.

## Compatibility notes

- protocol versioning is explicit in object keys and message bodies
- data objects are not overwritten
- ACK and close objects may be overwritten if that simplifies state progression
- the `--insecure-no-crypto` mode is only for local tests
- the server still needs an explicit allow policy: `--allow-target` or `--allow-unrestricted-egress`
- P1 keeps the `v1` key layout but is wire-incompatible with older clients and servers because open handshake chunk limits are mandatory and data objects use a binary encryption envelope.

## Protocol v1 P1 Extensions

`OpenRequest` and `OpenResult` now both require `max_receive_chunk_size`.

In `OpenRequest`, `max_receive_chunk_size` is the maximum plaintext data chunk the client accepts in the `s2c` direction. In `OpenResult`, it is the maximum plaintext data chunk the server accepts in the `c2s` direction. The effective send size is:

```text
min(local configured chunk-size, peer max_receive_chunk_size)
```

Valid chunk sizes are bounded by `MinChunkSize = 1024` and `MaxChunkSize = 16 MiB`. Missing, zero, negative, or oversized values are protocol errors. The limit is plaintext size before encryption and envelope overhead.

Data object keys and sequence semantics are unchanged. ACK format is unchanged, but senders cache cumulative ACK state locally and only read ACK objects when the cached window is full.

Data objects now use a compact binary AES-256-GCM envelope instead of JSON plus Base64. Control objects continue to use the JSON crypto envelope. The binary data envelope starts with `S5D1`, includes envelope version, algorithm id, nonce length, flags, ciphertext length, nonce, and ciphertext including the GCM tag. AAD still binds object type, session ID, direction, sequence, and protocol version.

Upload uses size-or-deadline aggregation. `chunk-size` is the local desired maximum plaintext chunk, capped by the peer receive limit. `flush-delay=0` disables waiting for additional bytes and flushes each non-empty read immediately; positive values allow small reads to coalesce until size, deadline, EOF, error, or cancellation.

## Limitations

- this is not a general-purpose message bus
- latency is dominated by object storage round trips
- request counts increase quickly with small chunks or short poll intervals
- the protocol is designed for controlled tunnels, not anonymous public relay use
