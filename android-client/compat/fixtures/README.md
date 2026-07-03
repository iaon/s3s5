# Compatibility Fixture Plan

Fixtures should be generated from the Go implementation and consumed by Android
JVM tests when the Android toolchain is available.

Planned fixtures:

- open request JSON with IPv4, IPv6, and domain targets
- open result JSON for accepted and rejected sessions
- ACK JSON with `next_seq`
- close JSON with and without `reason`
- PSK encrypted envelope for `open`, `open-result`, and `data`
- AAD input cases for `control`, `c2s`, and `s2c`
- derived HKDF key hex for a fixed PSK/session/direction

The current tests already pin deterministic values for key layout, AAD shape,
sequence formatting, and HKDF RFC5869 behavior.
