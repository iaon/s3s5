# Architecture

The Android client is split into small Kotlin packages:

- `protocol`: key layout, session IDs, sequence formatting, and JSON message
  encoding/decoding.
- `crypto`: Noop and PSK codecs compatible with the Go implementation.
- `objectstore`: object-store interface, counters, and memory store.
- `s3`: OkHttp S3-compatible client with narrow SigV4 support.
- `socks5`: localhost SOCKS5 parser/listener, no-auth CONNECT only.
- `tunnel`: SOCKS session orchestration over S3 mailbox objects.
- `service`: foreground service, notification, status bus, Doctor.
- `config`: SharedPreferences config and Android Keystore encrypted secrets.
- `ui`: minimal native View-based activity.

The foreground service owns the SOCKS listener and all tunnel coroutines. The UI
starts/stops the service through normal Android service intents and observes
status through an in-process `StateFlow`.
