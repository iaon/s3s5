# Android Protocol Compatibility

Source of truth checked in this implementation pass:

- `internal/protocol/protocol.go`
- `internal/crypto/codec.go`
- `internal/tunnel/tunnel.go`
- `internal/socks5/socks5.go`
- `internal/objectstore/s3/s3.go`
- `internal/config/config.go`

## Object Keys

The Android client uses the same normalized prefix behavior as Go: trim leading
and trailing slashes; empty prefix becomes `s3s5`.

- Open request: `<prefix>/v1/open/<session>.json`
- Open result: `<prefix>/v1/open-result/<session>.json`
- Data: `<prefix>/v1/data/<direction>/<session>/<seq>.bin`
- ACK: `<prefix>/v1/ack/<direction>/<session>.json`
- Close: `<prefix>/v1/close/<side>/<session>.json`
- Heartbeat: `<prefix>/v1/heartbeat/<side>/<session>.json`

Sessions are 16 random bytes encoded as 32 lowercase hex characters. Sequences
are decimal strings padded to 20 digits.

## JSON Messages

Control JSON field names match Go struct tags:

- `OpenRequest`: `version`, `session_id`, `target`, `created_at`
- `Target`: `type`, `host`, `port`
- `OpenResult`: `version`, `session_id`, `accepted`, `error`, `created_at`
- `Ack`: `version`, `session_id`, `direction`, `next_seq`, `updated_at`
- `Close`: `version`, `session_id`, `side`, `reason`, `created_at`

Timestamps are emitted as UTC RFC3339 strings via `Instant`.

## Crypto

PSK crypto matches the Go `PSKCodec`:

- minimum PSK length: 16 characters
- key derivation: HKDF-SHA256
- salt: `s3s5/v1/<session>`
- info: `payload/<direction>`
- output key length: 32 bytes
- cipher: AES-256-GCM with 12-byte nonce
- envelope JSON: `v`, `alg`, `nonce`, `ciphertext`
- `alg`: `AES-256-GCM`

AAD format is:

```text
s3s5/v1|<type>|<session>|<direction>|<20-digit-seq>
```

Control objects use sequence `0`. Control direction is `control`. Data objects
use directions `c2s` and `s2c`.

## Tunnel Behavior

The Android client opens sessions in the same order as the Go client:

1. Write encrypted `open`.
2. Poll encrypted `open-result`.
3. Reply SOCKS success only when accepted.
4. Stream client bytes to `data/c2s`.
5. Stream server bytes from `data/s2c`.

Defaults match Go:

- chunk size: 64 KiB
- poll minimum: 50 ms
- poll maximum: 2 seconds
- window: 16 chunks
- idle timeout: 2 minutes
- ACK interval: `1` when window is <= 2, otherwise `window / 2`

The receive path checks peer close markers every fourth miss and writes a final
ACK when needed before returning.
