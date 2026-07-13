# Changelog

All notable changes to this project should be documented in this file.

The format is based on Keep a Changelog, and this project intends to use
semantic versioning once it starts publishing tagged releases.

## Unreleased

## 0.2.0 - 2026-07-13

- Implement P1 S3 mailbox protocol optimizations.
- Add cumulative ACK send-window caching, size-or-deadline aggregation, binary encrypted data envelopes, activity-triggered polling wake-up, close-check cadence tuning, paginated open discovery, and mandatory directional chunk-limit negotiation.
- Update Go client/server, Android client, protocol documentation, performance documentation, and benchmark fixtures for the P1 wire-incompatible protocol update.
- Add final P1 memory and simulated-S3 benchmark results and optimization report.

## 0.1.2 - 2026-07-10

- Publish Android debug APKs from GitHub releases.
- Add arm64 server package artifacts for releases.
- Update GitHub Actions, Android, and packaging dependencies.

## 0.1.1 - 2026-07-06

- Add Linux Go client, server, and doctor MVP.
- Add Android client MVP.
- Add Docker-based Android and server package builds.
- Add root `VERSION` tracking and CLI `--version` output.
