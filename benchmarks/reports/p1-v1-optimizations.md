# s3s5 Performance Report

## Environment

- Commit: `4eda91f1c713`
- Dirty worktree: `true`
- Timestamp: `2026-07-13T15:06:11Z`
- Go: `go1.22.2`
- OS/arch: `linux/amd64`
- Profile/provider: `memory` / `memory`

## Protocol Configuration

| Chunk size | Flush delay | Poll min | Poll max | Active poll | Window chunks | Close misses | Idle timeout |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 65536 | 10ms | 1ms | 20ms | 500ms | 16 | 4 | 30s |

## Scenario Summary

| Scenario | Status | Bytes sent | Bytes received | Duration ms | PUT | GET hit | GET miss | HEAD | LIST | DELETE | Ops/session | Ops/MiB |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| one-byte-echo-active | passed | 1 | 1 | 94 | 6 | 4 | 35 | 8 | 9 | 1 | 63.00 | 66060288.00 |
| one-byte-echo-after-idle | passed | 1 | 1 | 10094 | 6 | 4 | 1015 | 252 | 499 | 1 | 1777.00 | 1863319552.00 |
| small-chatty-writes | passed | 22400 | 22400 | 10090 | 508 | 476 | 5038 | 1028 | 494 | 1 | 7545.00 | 353192.23 |
| bulk-one-mib | passed | 1048576 | 1048576 | 73 | 40 | 34 | 54 | 9 | 8 | 1 | 146.00 | 146.00 |
| short-connections | passed | 160 | 160 | 916 | 120 | 80 | 730 | 160 | 113 | 20 | 61.15 | 8015052.80 |
| concurrent-idle-sessions | passed | 0 | 0 | 10250 | 80 | 40 | 19793 | 4938 | 536 | 20 | 1270.35 | 0.00 |
| mixed-traffic | passed | 331244 | 331244 | 10138 | 1588 | 1488 | 18255 | 3726 | 496 | 7 | 3651.43 | 80911.96 |

## Idle Scenario Normalization

| Scenario | Sessions | Idle duration ms | GET/s | HEAD/s | LIST/s | Ops/session/s |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| concurrent-idle-sessions | 20 | 10000 | 1983.30 | 493.80 | 53.60 | 127.03 |

## one-byte-echo-active

Open one SOCKS connection, send one byte, and read one echoed byte without a long idle period.

- Status: `passed`
- Connections: `1`
- Requests: `1`
- Chunks sent/received: `2/2`
- Plaintext bytes sent/received: `2/2`
- Sealed/plaintext ratio: `41.000`
- Time to open result p50/p95/p99 ms: `3.835/3.835/3.835`

| Operation | Key class | Result | Count | p50 ms | p95 ms | p99 ms | Max ms | Request bytes | Response bytes |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| delete | open | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| get | data-c2s | not_found | 16 | 0.001 | 0.004 | 0.005 | 0.005 | 0 | 0 |
| get | data-c2s | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 41 |
| get | data-s2c | not_found | 17 | 0.001 | 0.003 | 0.005 | 0.005 | 0 | 0 |
| get | data-s2c | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 41 |
| get | open | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 354 |
| get | open-result | not_found | 2 | 0.002 | 0.002 | 0.002 | 0.002 | 0 | 0 |
| get | open-result | success | 1 | 0.003 | 0.003 | 0.003 | 0.003 | 0 | 298 |
| head | close-client | not_found | 3 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-client | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-server | not_found | 3 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-server | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| list | list-open | success | 9 | 0.002 | 0.008 | 0.008 | 0.008 | 0 | 71 |
| put | close-client | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 258 | 0 |
| put | close-server | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 258 | 0 |
| put | data-c2s | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 41 | 0 |
| put | data-s2c | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 41 | 0 |
| put | open | success | 1 | 0.002 | 0.002 | 0.002 | 0.002 | 354 | 0 |
| put | open-result | success | 1 | 0.002 | 0.002 | 0.002 | 0.002 | 298 | 0 |

Observations:

- GET not_found polling misses are at least half of observed object-store operations.
- HEAD close-marker checks are visible in the request mix.
- LIST open-session polling is part of baseline request volume.

## one-byte-echo-after-idle

Open one SOCKS connection, hold it idle for the configured duration, then send and echo one byte.

- Status: `passed`
- Connections: `1`
- Requests: `1`
- Chunks sent/received: `2/2`
- Plaintext bytes sent/received: `2/2`
- Sealed/plaintext ratio: `41.000`
- Time to open result p50/p95/p99 ms: `3.700/3.700/3.700`

| Operation | Key class | Result | Count | p50 ms | p95 ms | p99 ms | Max ms | Request bytes | Response bytes |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| delete | open | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| get | data-c2s | not_found | 506 | 0.002 | 0.004 | 0.016 | 0.029 | 0 | 0 |
| get | data-c2s | success | 1 | 0.006 | 0.006 | 0.006 | 0.006 | 0 | 41 |
| get | data-s2c | not_found | 507 | 0.001 | 0.003 | 0.015 | 0.058 | 0 | 0 |
| get | data-s2c | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 41 |
| get | open | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 354 |
| get | open-result | not_found | 2 | 0.002 | 0.002 | 0.002 | 0.002 | 0 | 0 |
| get | open-result | success | 1 | 0.007 | 0.007 | 0.007 | 0.007 | 0 | 298 |
| head | close-client | not_found | 125 | 0.001 | 0.001 | 0.001 | 0.010 | 0 | 0 |
| head | close-client | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-server | not_found | 125 | 0.001 | 0.001 | 0.011 | 0.016 | 0 | 0 |
| head | close-server | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| list | list-open | success | 499 | 0.003 | 0.006 | 0.021 | 0.027 | 0 | 75 |
| put | close-client | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 258 | 0 |
| put | close-server | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 258 | 0 |
| put | data-c2s | success | 1 | 0.003 | 0.003 | 0.003 | 0.003 | 41 | 0 |
| put | data-s2c | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 41 | 0 |
| put | open | success | 1 | 0.006 | 0.006 | 0.006 | 0.006 | 354 | 0 |
| put | open-result | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 298 | 0 |

Observations:

- GET not_found polling misses are at least half of observed object-store operations.
- HEAD close-marker checks are visible in the request mix.
- LIST open-session polling is part of baseline request volume.

## small-chatty-writes

Send ordered 100-byte writes for the configured duration and verify echoed order.

- Status: `passed`
- Connections: `1`
- Requests: `224`
- Chunks sent/received: `448/448`
- Plaintext bytes sent/received: `44800/44800`
- Sealed/plaintext ratio: `1.400`
- Time to open result p50/p95/p99 ms: `3.569/3.569/3.569`

| Operation | Key class | Result | Count | p50 ms | p95 ms | p99 ms | Max ms | Request bytes | Response bytes |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| delete | open | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| get | ack-c2s | success | 13 | 0.002 | 0.004 | 0.005 | 0.005 | 0 | 3614 |
| get | ack-s2c | success | 13 | 0.002 | 0.003 | 0.004 | 0.004 | 0 | 3614 |
| get | data-c2s | not_found | 2480 | 0.001 | 0.003 | 0.006 | 0.030 | 0 | 0 |
| get | data-c2s | success | 224 | 0.002 | 0.004 | 0.014 | 0.032 | 0 | 31360 |
| get | data-s2c | not_found | 2556 | 0.001 | 0.003 | 0.005 | 0.026 | 0 | 0 |
| get | data-s2c | success | 224 | 0.002 | 0.004 | 0.015 | 0.027 | 0 | 31360 |
| get | open | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 354 |
| get | open-result | not_found | 2 | 0.002 | 0.002 | 0.002 | 0.002 | 0 | 0 |
| get | open-result | success | 1 | 0.002 | 0.002 | 0.002 | 0.002 | 0 | 298 |
| head | close-client | not_found | 461 | 0.001 | 0.001 | 0.002 | 0.025 | 0 | 0 |
| head | close-client | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-server | not_found | 565 | 0.001 | 0.001 | 0.002 | 0.006 | 0 | 0 |
| head | close-server | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| list | list-open | success | 494 | 0.017 | 0.037 | 0.050 | 0.086 | 0 | 70 |
| put | ack-c2s | success | 28 | 0.002 | 0.003 | 0.003 | 0.003 | 7784 | 0 |
| put | ack-s2c | success | 28 | 0.001 | 0.004 | 0.023 | 0.023 | 7784 | 0 |
| put | close-client | success | 1 | 0.003 | 0.003 | 0.003 | 0.003 | 258 | 0 |
| put | close-server | success | 1 | 0.003 | 0.003 | 0.003 | 0.003 | 258 | 0 |
| put | data-c2s | success | 224 | 0.002 | 0.006 | 0.019 | 0.031 | 31360 | 0 |
| put | data-s2c | success | 224 | 0.002 | 0.005 | 0.016 | 0.027 | 31360 | 0 |
| put | open | success | 1 | 0.002 | 0.002 | 0.002 | 0.002 | 354 | 0 |
| put | open-result | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 298 | 0 |

Observations:

- GET not_found polling misses are at least half of observed object-store operations.
- HEAD close-marker checks are visible in the request mix.
- LIST open-session polling is part of baseline request volume.

## bulk-one-mib

Send and echo one contiguous MiB, verifying SHA-256 and exact byte count.

- Status: `passed`
- Connections: `1`
- Requests: `1`
- Chunks sent/received: `32/32`
- Plaintext bytes sent/received: `2097152/2097152`
- Sealed/plaintext ratio: `1.001`
- Time to open result p50/p95/p99 ms: `3.580/3.580/3.580`

| Operation | Key class | Result | Count | p50 ms | p95 ms | p99 ms | Max ms | Request bytes | Response bytes |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| delete | open | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| get | data-c2s | not_found | 25 | 0.001 | 0.003 | 0.009 | 0.009 | 0 | 0 |
| get | data-c2s | success | 16 | 0.009 | 0.126 | 0.170 | 0.170 | 0 | 1049216 |
| get | data-s2c | not_found | 27 | 0.001 | 0.002 | 0.002 | 0.002 | 0 | 0 |
| get | data-s2c | success | 16 | 0.009 | 0.103 | 0.116 | 0.116 | 0 | 1049216 |
| get | open | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 354 |
| get | open-result | not_found | 2 | 0.002 | 0.002 | 0.002 | 0.002 | 0 | 0 |
| get | open-result | success | 1 | 0.002 | 0.002 | 0.002 | 0.002 | 0 | 298 |
| head | close-client | not_found | 3 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-client | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-server | not_found | 4 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-server | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| list | list-open | success | 8 | 0.003 | 0.008 | 0.008 | 0.008 | 0 | 63 |
| put | ack-c2s | success | 2 | 0.001 | 0.001 | 0.001 | 0.001 | 556 | 0 |
| put | ack-s2c | success | 2 | 0.001 | 0.001 | 0.001 | 0.001 | 556 | 0 |
| put | close-client | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 258 | 0 |
| put | close-server | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 258 | 0 |
| put | data-c2s | success | 16 | 0.007 | 0.118 | 0.136 | 0.136 | 1049216 | 0 |
| put | data-s2c | success | 16 | 0.007 | 0.118 | 0.146 | 0.146 | 1049216 | 0 |
| put | open | success | 1 | 0.002 | 0.002 | 0.002 | 0.002 | 354 | 0 |
| put | open-result | success | 1 | 0.002 | 0.002 | 0.002 | 0.002 | 298 | 0 |

Observations:

- HEAD close-marker checks are visible in the request mix.
- LIST open-session polling is part of baseline request volume.

## short-connections

Open sequential short SOCKS connections and exchange a small request/response on each.

- Status: `passed`
- Connections: `20`
- Requests: `20`
- Chunks sent/received: `40/40`
- Plaintext bytes sent/received: `320/320`
- Sealed/plaintext ratio: `6.000`
- Time to open result p50/p95/p99 ms: `8.293/31.892/32.661`

| Operation | Key class | Result | Count | p50 ms | p95 ms | p99 ms | Max ms | Request bytes | Response bytes |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| delete | open | success | 20 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| get | data-c2s | not_found | 324 | 0.001 | 0.003 | 0.011 | 0.017 | 0 | 0 |
| get | data-c2s | success | 20 | 0.002 | 0.004 | 0.021 | 0.021 | 0 | 960 |
| get | data-s2c | not_found | 346 | 0.001 | 0.002 | 0.004 | 0.022 | 0 | 0 |
| get | data-s2c | success | 20 | 0.001 | 0.002 | 0.004 | 0.004 | 0 | 960 |
| get | open | success | 20 | 0.001 | 0.002 | 0.003 | 0.003 | 0 | 7076 |
| get | open-result | not_found | 60 | 0.002 | 0.002 | 0.003 | 0.006 | 0 | 0 |
| get | open-result | success | 20 | 0.002 | 0.003 | 0.003 | 0.003 | 0 | 5956 |
| head | close-client | not_found | 60 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-client | success | 20 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-server | not_found | 60 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-server | success | 20 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| list | list-open | success | 113 | 0.004 | 0.010 | 0.024 | 0.024 | 0 | 1360 |
| put | close-client | success | 20 | 0.001 | 0.003 | 0.003 | 0.003 | 5152 | 0 |
| put | close-server | success | 20 | 0.001 | 0.005 | 0.006 | 0.006 | 5152 | 0 |
| put | data-c2s | success | 20 | 0.001 | 0.003 | 0.004 | 0.004 | 960 | 0 |
| put | data-s2c | success | 20 | 0.001 | 0.002 | 0.003 | 0.003 | 960 | 0 |
| put | open | success | 20 | 0.001 | 0.002 | 0.002 | 0.002 | 7076 | 0 |
| put | open-result | success | 20 | 0.001 | 0.002 | 0.002 | 0.002 | 5956 | 0 |

Observations:

- GET not_found polling misses are at least half of observed object-store operations.
- HEAD close-marker checks are visible in the request mix.
- LIST open-session polling is part of baseline request volume.

## concurrent-idle-sessions

Open idle SOCKS sessions for a bounded period and measure background polling.

- Status: `passed`
- Connections: `20`
- Requests: `0`
- Chunks sent/received: `0/0`
- Plaintext bytes sent/received: `0/0`
- Sealed/plaintext ratio: `0.000`
- Time to open result p50/p95/p99 ms: `3.702/3.959/8.209`

| Operation | Key class | Result | Count | p50 ms | p95 ms | p99 ms | Max ms | Request bytes | Response bytes |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| delete | open | success | 20 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| get | data-c2s | not_found | 9840 | 0.001 | 0.002 | 0.004 | 0.326 | 0 | 0 |
| get | data-s2c | not_found | 9912 | 0.001 | 0.002 | 0.004 | 0.048 | 0 | 0 |
| get | open | success | 20 | 0.001 | 0.002 | 0.002 | 0.002 | 0 | 7068 |
| get | open-result | not_found | 41 | 0.001 | 0.002 | 0.004 | 0.004 | 0 | 0 |
| get | open-result | success | 20 | 0.001 | 0.003 | 0.004 | 0.004 | 0 | 5960 |
| head | close-client | not_found | 2440 | 0.001 | 0.001 | 0.002 | 0.020 | 0 | 0 |
| head | close-client | success | 20 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-server | not_found | 2458 | 0.001 | 0.001 | 0.001 | 0.021 | 0 | 0 |
| head | close-server | success | 20 | 0.001 | 0.001 | 0.021 | 0.021 | 0 | 0 |
| list | list-open | success | 536 | 0.003 | 0.006 | 0.013 | 0.026 | 0 | 1500 |
| put | close-client | success | 20 | 0.001 | 0.006 | 0.029 | 0.029 | 5152 | 0 |
| put | close-server | success | 20 | 0.001 | 0.005 | 0.013 | 0.013 | 5152 | 0 |
| put | open | success | 20 | 0.001 | 0.002 | 0.045 | 0.045 | 7068 | 0 |
| put | open-result | success | 20 | 0.001 | 0.002 | 0.002 | 0.002 | 5960 | 0 |

Observations:

- GET not_found polling misses are at least half of observed object-store operations.
- HEAD close-marker checks are visible in the request mix.
- LIST open-session polling is part of baseline request volume.

## mixed-traffic

Run one bulk stream, small request/response streams, and idle streams at the same time.

- Status: `passed`
- Connections: `7`
- Requests: `692`
- Chunks sent/received: `1390/1390`
- Plaintext bytes sent/received: `662488/662488`
- Sealed/plaintext ratio: `1.084`
- Time to open result p50/p95/p99 ms: `3.967/4.042/4.042`

| Operation | Key class | Result | Count | p50 ms | p95 ms | p99 ms | Max ms | Request bytes | Response bytes |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| delete | open | success | 7 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| get | ack-c2s | success | 42 | 0.002 | 0.004 | 0.009 | 0.009 | 0 | 11676 |
| get | ack-s2c | success | 42 | 0.002 | 0.004 | 0.005 | 0.005 | 0 | 11676 |
| get | data-c2s | not_found | 9093 | 0.001 | 0.002 | 0.005 | 0.193 | 0 | 0 |
| get | data-c2s | success | 695 | 0.002 | 0.004 | 0.018 | 0.116 | 0 | 359044 |
| get | data-s2c | not_found | 9148 | 0.001 | 0.002 | 0.004 | 0.087 | 0 | 0 |
| get | data-s2c | success | 695 | 0.002 | 0.003 | 0.023 | 0.093 | 0 | 359044 |
| get | open | success | 7 | 0.002 | 0.002 | 0.002 | 0.002 | 0 | 2478 |
| get | open-result | not_found | 14 | 0.001 | 0.002 | 0.010 | 0.010 | 0 | 0 |
| get | open-result | success | 7 | 0.002 | 0.003 | 0.003 | 0.003 | 0 | 2086 |
| head | close-client | not_found | 1770 | 0.001 | 0.001 | 0.002 | 0.158 | 0 | 0 |
| head | close-client | success | 7 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-server | not_found | 1942 | 0.001 | 0.001 | 0.002 | 0.039 | 0 | 0 |
| head | close-server | success | 7 | 0.001 | 0.002 | 0.002 | 0.002 | 0 | 0 |
| list | list-open | success | 496 | 0.041 | 0.082 | 0.117 | 0.313 | 0 | 448 |
| put | ack-c2s | success | 85 | 0.001 | 0.003 | 0.006 | 0.032 | 23630 | 0 |
| put | ack-s2c | success | 85 | 0.001 | 0.006 | 0.036 | 0.068 | 23630 | 0 |
| put | close-client | success | 7 | 0.001 | 0.004 | 0.004 | 0.004 | 1802 | 0 |
| put | close-server | success | 7 | 0.001 | 0.003 | 0.003 | 0.003 | 1802 | 0 |
| put | data-c2s | success | 695 | 0.002 | 0.004 | 0.032 | 0.102 | 359044 | 0 |
| put | data-s2c | success | 695 | 0.002 | 0.005 | 0.060 | 0.142 | 359044 | 0 |
| put | open | success | 7 | 0.002 | 0.003 | 0.003 | 0.003 | 2478 | 0 |
| put | open-result | success | 7 | 0.002 | 0.004 | 0.004 | 0.004 | 2086 | 0 |

Observations:

- GET not_found polling misses are at least half of observed object-store operations.
- HEAD close-marker checks are visible in the request mix.
- LIST open-session polling is part of baseline request volume.

## Operations By Key Class

| Key class | Operation | Count |
| --- | --- | ---: |
| ack-c2s | get | 55 |
| ack-c2s | put | 115 |
| ack-s2c | get | 55 |
| ack-s2c | put | 115 |
| close-client | head | 4913 |
| close-client | put | 51 |
| close-server | head | 5208 |
| close-server | put | 51 |
| data-c2s | get | 23241 |
| data-c2s | put | 957 |
| data-s2c | get | 23470 |
| data-s2c | put | 957 |
| list-open | list | 2155 |
| open | delete | 51 |
| open | get | 51 |
| open | put | 51 |
| open-result | get | 174 |
| open-result | put | 51 |

## Baseline Before v1 Optimizations

This report records protocol v1 behavior before optimization work. It does not change key layout, ACK behavior, close markers, chunking, polling, or Android compatibility.
