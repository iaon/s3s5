# Android Porting Notes

The current implementation target is Linux, but the client-side core is intentionally shaped so it can be ported to Android later.

## What should carry over

These parts are already portable in spirit and should stay platform-neutral:

- SOCKS5 parsing and reply logic
- S3 mailbox protocol encoding
- encryption envelope handling
- policy evaluation
- object-key layout
- session state rules

## What is Linux-specific today

These pieces are tied to the current Linux MVP:

- the local TCP listener used by `s3s5-client`
- process signal handling in the CLI wrappers
- direct use of the host network stack for outbound TCP
- desktop-style stdout/stderr operational behavior

An Android build will need a different app shell, lifecycle handling, and background execution strategy around the same core protocol logic.

## Porting shape

The cleanest Android path is:

1. keep protocol and policy logic in a reusable core module
2. keep object-store and crypto APIs stable
3. replace the Linux listener/CLI wrapper with Android networking and service code
4. preserve session IDs, key layout, and envelope compatibility so both clients can talk to the same server

## IPv6

IPv6 should remain supported on Android as well:

- SOCKS5 target parsing already accepts IPv6
- protocol targets preserve IPv6 addresses
- policy rules understand IPv6 CIDRs

The Android port should not reintroduce IPv4-only assumptions into the shared logic.

## Practical constraints

Android introduces constraints the Linux MVP does not handle:

- background execution limits
- battery impact
- network transitions and captive portals
- different TLS and proxy expectations if the app later talks to S3 through a mobile SDK

None of these change the protocol design, but they do affect the transport wrapper and app lifecycle.

## Porting risk

The largest risk is accidental coupling between portable protocol logic and Linux-only socket/CLI code.

Keep those boundaries strict now so the later port is mostly an integration job, not a protocol rewrite.
