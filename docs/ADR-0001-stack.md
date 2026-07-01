# ADR-0001: Go for the Linux MVP

## Status

Accepted.

## Context

The product target is an S3-backed SOCKS5 tunnel with a Linux MVP and a later Android client port.

The implementation needs:

- TCP and SOCKS5 socket handling
- S3-compatible object-store access
- protocol framing and encryption
- destination policy checks
- a small CLI surface
- tests that can run without real S3

The codebase also needs to stay portable enough that the client-side core can be lifted to Android later.

## Decision

Use Go for the Linux MVP.

## Rationale

Go fits the current requirements with the least friction:

- good standard-library TCP and context support
- straightforward static binaries for Linux
- simple concurrency for polling and bidirectional stream bridging
- good test support
- easy S3 SDK integration if the implementation grows in that direction
- `net/netip` and the standard library already model IPv4, IPv6, and CIDRs cleanly

Go also supports a clean separation between:

- SOCKS5 parsing
- protocol encoding
- crypto envelopes
- policy enforcement
- object-store access
- tunnel orchestration

That separation matters more than any one language feature because the Android port should be able to reuse the protocol rules and policy model even if the transport wrapper changes.

## Consequences

Positive:

- faster MVP delivery
- simpler Linux distribution
- easier unit and integration testing
- clearer line between portable core logic and platform-specific transport code

Tradeoffs:

- the Android client will still need a separate shell around the portable core
- the S3 transport is inherently high-latency and request-heavy
- the first implementation is optimized for explicitness rather than raw throughput

## Scope limits

This ADR only covers the MVP stack choice. It does not commit the project to Go forever, but it does set the implementation baseline for the current repository.
