# s3s5 Performance Baseline

## Environment

- Commit: `ee5f95b81c82`
- Dirty worktree: `true`
- Timestamp: `2026-07-13T13:41:53Z`
- Go: `go1.22.2`
- OS/arch: `linux/amd64`
- Profile/provider: `memory` / `memory`

## Protocol Configuration

| Chunk size | Poll min | Poll max | Window chunks | Idle timeout |
| ---: | ---: | ---: | ---: | ---: |
| 65536 | 1ms | 20ms | 16 | 30s |

## Scenario Summary

| Scenario | Status | Bytes sent | Bytes received | Duration ms | PUT | GET hit | GET miss | HEAD | LIST | DELETE | Ops/session | Ops/MiB |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| one-byte-echo-active | passed | 1 | 1 | 28 | 8 | 4 | 16 | 6 | 6 | 1 | 41.00 | 42991616.00 |
| one-byte-echo-after-idle | passed | 1 | 1 | 10027 | 8 | 4 | 994 | 250 | 494 | 1 | 1751.00 | 1836056576.00 |
| small-chatty-writes | passed | 117900 | 117900 | 10024 | 2658 | 4686 | 7156 | 2362 | 490 | 1 | 17353.00 | 154333.67 |
| bulk-one-mib | passed | 1048576 | 1048576 | 58 | 46 | 42 | 22 | 6 | 8 | 1 | 125.00 | 125.00 |
| short-connections | passed | 160 | 160 | 353 | 160 | 80 | 379 | 122 | 80 | 20 | 42.05 | 5511577.60 |
| concurrent-idle-sessions | passed | 0 | 0 | 10279 | 80 | 40 | 19765 | 4960 | 540 | 20 | 1270.25 | 0.00 |
| mixed-traffic | passed | 523044 | 523044 | 10110 | 5915 | 10365 | 20643 | 5984 | 494 | 7 | 6201.14 | 87022.48 |

## Idle Scenario Normalization

| Scenario | Sessions | Idle duration ms | GET/s | HEAD/s | LIST/s | Ops/session/s |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| concurrent-idle-sessions | 20 | 10000 | 1980.50 | 496.00 | 54.00 | 127.03 |

## one-byte-echo-active

Open one SOCKS connection, send one byte, and read one echoed byte without a long idle period.

- Status: `passed`
- Connections: `1`
- Requests: `1`
- Chunks sent/received: `2/2`
- Plaintext bytes sent/received: `2/2`
- Sealed/plaintext ratio: `94.000`
- Time to open result p50/p95/p99 ms: `3.916/3.916/3.916`

| Operation | Key class | Result | Count | p50 ms | p95 ms | p99 ms | Max ms | Request bytes | Response bytes |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| delete | open | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| get | data-c2s | not_found | 7 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| get | data-c2s | success | 1 | 0.002 | 0.002 | 0.002 | 0.002 | 0 | 94 |
| get | data-s2c | not_found | 7 | 0.001 | 0.006 | 0.006 | 0.006 | 0 | 0 |
| get | data-s2c | success | 1 | 0.002 | 0.002 | 0.002 | 0.002 | 0 | 94 |
| get | open | success | 1 | 0.003 | 0.003 | 0.003 | 0.003 | 0 | 310 |
| get | open-result | not_found | 2 | 0.002 | 0.002 | 0.002 | 0.002 | 0 | 0 |
| get | open-result | success | 1 | 0.008 | 0.008 | 0.008 | 0.008 | 0 | 254 |
| head | close-client | not_found | 2 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-client | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-server | not_found | 2 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-server | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| list | list-open | success | 6 | 0.002 | 0.015 | 0.015 | 0.015 | 0 | 355 |
| put | ack-c2s | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 278 | 0 |
| put | ack-s2c | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 278 | 0 |
| put | close-client | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 258 | 0 |
| put | close-server | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 258 | 0 |
| put | data-c2s | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 94 | 0 |
| put | data-s2c | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 94 | 0 |
| put | open | success | 1 | 0.007 | 0.007 | 0.007 | 0.007 | 310 | 0 |
| put | open-result | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 254 | 0 |

Observations:

- HEAD close-marker checks are visible in the request mix.
- LIST open-session polling is part of baseline request volume.

## one-byte-echo-after-idle

Open one SOCKS connection, hold it idle for the configured duration, then send and echo one byte.

- Status: `passed`
- Connections: `1`
- Requests: `1`
- Chunks sent/received: `2/2`
- Plaintext bytes sent/received: `2/2`
- Sealed/plaintext ratio: `94.000`
- Time to open result p50/p95/p99 ms: `3.637/3.637/3.637`

| Operation | Key class | Result | Count | p50 ms | p95 ms | p99 ms | Max ms | Request bytes | Response bytes |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| delete | open | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| get | data-c2s | not_found | 496 | 0.002 | 0.003 | 0.016 | 0.022 | 0 | 0 |
| get | data-c2s | success | 1 | 0.002 | 0.002 | 0.002 | 0.002 | 0 | 94 |
| get | data-s2c | not_found | 496 | 0.001 | 0.003 | 0.018 | 0.038 | 0 | 0 |
| get | data-s2c | success | 1 | 0.002 | 0.002 | 0.002 | 0.002 | 0 | 94 |
| get | open | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 310 |
| get | open-result | not_found | 2 | 0.002 | 0.002 | 0.002 | 0.002 | 0 | 0 |
| get | open-result | success | 1 | 0.002 | 0.002 | 0.002 | 0.002 | 0 | 258 |
| head | close-client | not_found | 124 | 0.001 | 0.001 | 0.002 | 0.004 | 0 | 0 |
| head | close-client | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-server | not_found | 124 | 0.001 | 0.001 | 0.021 | 0.027 | 0 | 0 |
| head | close-server | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| list | list-open | success | 494 | 0.003 | 0.010 | 0.021 | 0.069 | 0 | 36900 |
| put | ack-c2s | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 278 | 0 |
| put | ack-s2c | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 278 | 0 |
| put | close-client | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 258 | 0 |
| put | close-server | success | 1 | 0.002 | 0.002 | 0.002 | 0.002 | 258 | 0 |
| put | data-c2s | success | 1 | 0.003 | 0.003 | 0.003 | 0.003 | 94 | 0 |
| put | data-s2c | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 94 | 0 |
| put | open | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 310 | 0 |
| put | open-result | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 258 | 0 |

Observations:

- GET not_found polling misses are at least half of observed object-store operations.
- HEAD close-marker checks are visible in the request mix.
- LIST open-session polling is part of baseline request volume.

## small-chatty-writes

Send ordered 100-byte writes for the configured duration and verify echoed order.

- Status: `passed`
- Connections: `1`
- Requests: `1179`
- Chunks sent/received: `2358/2358`
- Plaintext bytes sent/received: `235800/235800`
- Sealed/plaintext ratio: `2.260`
- Time to open result p50/p95/p99 ms: `3.795/3.795/3.795`

| Operation | Key class | Result | Count | p50 ms | p95 ms | p99 ms | Max ms | Request bytes | Response bytes |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| delete | open | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| get | ack-c2s | success | 1163 | 0.001 | 0.004 | 0.008 | 0.030 | 0 | 323934 |
| get | ack-s2c | success | 1163 | 0.001 | 0.003 | 0.007 | 0.173 | 0 | 323966 |
| get | data-c2s | not_found | 3577 | 0.001 | 0.003 | 0.006 | 0.066 | 0 | 0 |
| get | data-c2s | success | 1179 | 0.001 | 0.003 | 0.010 | 0.128 | 0 | 266454 |
| get | data-s2c | not_found | 3577 | 0.001 | 0.003 | 0.006 | 0.031 | 0 | 0 |
| get | data-s2c | success | 1179 | 0.001 | 0.003 | 0.007 | 0.029 | 0 | 266454 |
| get | open | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 310 |
| get | open-result | not_found | 2 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| get | open-result | success | 1 | 0.002 | 0.002 | 0.002 | 0.002 | 0 | 254 |
| head | close-client | not_found | 1180 | 0.001 | 0.001 | 0.003 | 0.010 | 0 | 0 |
| head | close-client | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-server | not_found | 1180 | 0.001 | 0.001 | 0.003 | 0.027 | 0 | 0 |
| head | close-server | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| list | list-open | success | 490 | 0.063 | 0.134 | 0.165 | 0.244 | 0 | 34230 |
| put | ack-c2s | success | 148 | 0.001 | 0.005 | 0.011 | 0.067 | 41228 | 0 |
| put | ack-s2c | success | 148 | 0.001 | 0.005 | 0.063 | 0.082 | 41232 | 0 |
| put | close-client | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 258 | 0 |
| put | close-server | success | 1 | 0.002 | 0.002 | 0.002 | 0.002 | 258 | 0 |
| put | data-c2s | success | 1179 | 0.001 | 0.005 | 0.069 | 0.494 | 266454 | 0 |
| put | data-s2c | success | 1179 | 0.001 | 0.005 | 0.035 | 0.096 | 266454 | 0 |
| put | open | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 310 | 0 |
| put | open-result | success | 1 | 0.002 | 0.002 | 0.002 | 0.002 | 254 | 0 |

Observations:

- HEAD close-marker checks are visible in the request mix.
- LIST open-session polling is part of baseline request volume.

## bulk-one-mib

Send and echo one contiguous MiB, verifying SHA-256 and exact byte count.

- Status: `passed`
- Connections: `1`
- Requests: `1`
- Chunks sent/received: `36/36`
- Plaintext bytes sent/received: `2097152/2097152`
- Sealed/plaintext ratio: `1.335`
- Time to open result p50/p95/p99 ms: `3.723/3.723/3.723`

| Operation | Key class | Result | Count | p50 ms | p95 ms | p99 ms | Max ms | Request bytes | Response bytes |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| delete | open | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| get | ack-c2s | not_found | 2 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| get | ack-c2s | success | 2 | 0.002 | 0.002 | 0.002 | 0.002 | 0 | 556 |
| get | ack-s2c | success | 2 | 0.002 | 0.002 | 0.002 | 0.002 | 0 | 556 |
| get | data-c2s | not_found | 9 | 0.003 | 0.008 | 0.008 | 0.008 | 0 | 0 |
| get | data-c2s | success | 18 | 0.011 | 0.052 | 0.096 | 0.096 | 0 | 1399764 |
| get | data-s2c | not_found | 9 | 0.003 | 0.012 | 0.012 | 0.012 | 0 | 0 |
| get | data-s2c | success | 18 | 0.014 | 0.088 | 0.103 | 0.103 | 0 | 1399764 |
| get | open | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 310 |
| get | open-result | not_found | 2 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| get | open-result | success | 1 | 0.008 | 0.008 | 0.008 | 0.008 | 0 | 258 |
| head | close-client | not_found | 2 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-client | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-server | not_found | 2 | 0.012 | 0.012 | 0.012 | 0.012 | 0 | 0 |
| head | close-server | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| list | list-open | success | 8 | 0.009 | 0.014 | 0.014 | 0.014 | 0 | 378 |
| put | ack-c2s | success | 3 | 0.002 | 0.002 | 0.002 | 0.002 | 834 | 0 |
| put | ack-s2c | success | 3 | 0.001 | 0.001 | 0.001 | 0.001 | 834 | 0 |
| put | close-client | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 258 | 0 |
| put | close-server | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 258 | 0 |
| put | data-c2s | success | 18 | 0.016 | 0.037 | 0.049 | 0.049 | 1399764 | 0 |
| put | data-s2c | success | 18 | 0.008 | 0.048 | 0.053 | 0.053 | 1399764 | 0 |
| put | open | success | 1 | 0.002 | 0.002 | 0.002 | 0.002 | 310 | 0 |
| put | open-result | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 258 | 0 |

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
- Sealed/plaintext ratio: `12.750`
- Time to open result p50/p95/p99 ms: `8.350/16.649/16.763`

| Operation | Key class | Result | Count | p50 ms | p95 ms | p99 ms | Max ms | Request bytes | Response bytes |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| delete | open | success | 20 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| get | data-c2s | not_found | 157 | 0.001 | 0.002 | 0.005 | 0.007 | 0 | 0 |
| get | data-c2s | success | 20 | 0.001 | 0.002 | 0.006 | 0.006 | 0 | 2040 |
| get | data-s2c | not_found | 162 | 0.001 | 0.002 | 0.005 | 0.017 | 0 | 0 |
| get | data-s2c | success | 20 | 0.001 | 0.002 | 0.003 | 0.003 | 0 | 2040 |
| get | open | success | 20 | 0.001 | 0.002 | 0.004 | 0.004 | 0 | 6200 |
| get | open-result | not_found | 60 | 0.002 | 0.003 | 0.003 | 0.017 | 0 | 0 |
| get | open-result | success | 20 | 0.002 | 0.006 | 0.015 | 0.015 | 0 | 5152 |
| head | close-client | not_found | 40 | 0.001 | 0.002 | 0.017 | 0.017 | 0 | 0 |
| head | close-client | success | 20 | 0.001 | 0.001 | 0.002 | 0.002 | 0 | 0 |
| head | close-server | not_found | 42 | 0.001 | 0.001 | 0.002 | 0.002 | 0 | 0 |
| head | close-server | success | 20 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| list | list-open | success | 80 | 0.004 | 0.008 | 0.011 | 0.013 | 0 | 10268 |
| put | ack-c2s | success | 20 | 0.001 | 0.002 | 0.005 | 0.005 | 5544 | 0 |
| put | ack-s2c | success | 20 | 0.001 | 0.001 | 0.003 | 0.003 | 5552 | 0 |
| put | close-client | success | 20 | 0.001 | 0.002 | 0.003 | 0.003 | 5160 | 0 |
| put | close-server | success | 20 | 0.001 | 0.002 | 0.002 | 0.002 | 5148 | 0 |
| put | data-c2s | success | 20 | 0.001 | 0.002 | 0.008 | 0.008 | 2040 | 0 |
| put | data-s2c | success | 20 | 0.001 | 0.004 | 0.005 | 0.005 | 2040 | 0 |
| put | open | success | 20 | 0.001 | 0.005 | 0.007 | 0.007 | 6200 | 0 |
| put | open-result | success | 20 | 0.001 | 0.004 | 0.006 | 0.006 | 5152 | 0 |

Observations:

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
- Time to open result p50/p95/p99 ms: `3.782/8.764/8.767`

| Operation | Key class | Result | Count | p50 ms | p95 ms | p99 ms | Max ms | Request bytes | Response bytes |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| delete | open | success | 20 | 0.001 | 0.001 | 0.006 | 0.006 | 0 | 0 |
| get | data-c2s | not_found | 9828 | 0.001 | 0.003 | 0.007 | 0.055 | 0 | 0 |
| get | data-s2c | not_found | 9892 | 0.001 | 0.003 | 0.007 | 0.071 | 0 | 0 |
| get | open | success | 20 | 0.001 | 0.002 | 0.002 | 0.002 | 0 | 6200 |
| get | open-result | not_found | 45 | 0.001 | 0.002 | 0.023 | 0.023 | 0 | 0 |
| get | open-result | success | 20 | 0.001 | 0.003 | 0.003 | 0.003 | 0 | 5156 |
| head | close-client | not_found | 2452 | 0.001 | 0.001 | 0.003 | 0.019 | 0 | 0 |
| head | close-client | success | 20 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-server | not_found | 2468 | 0.001 | 0.001 | 0.003 | 0.027 | 0 | 0 |
| head | close-server | success | 20 | 0.001 | 0.002 | 0.022 | 0.022 | 0 | 0 |
| list | list-open | success | 540 | 0.008 | 0.025 | 0.037 | 0.098 | 0 | 766275 |
| put | close-client | success | 20 | 0.002 | 0.010 | 0.017 | 0.017 | 5156 | 0 |
| put | close-server | success | 20 | 0.001 | 0.005 | 0.010 | 0.010 | 5156 | 0 |
| put | open | success | 20 | 0.001 | 0.003 | 0.003 | 0.003 | 6200 | 0 |
| put | open-result | success | 20 | 0.001 | 0.002 | 0.002 | 0.002 | 5156 | 0 |

Observations:

- GET not_found polling misses are at least half of observed object-store operations.
- HEAD close-marker checks are visible in the request mix.
- LIST open-session polling is part of baseline request volume.

## mixed-traffic

Run one bulk stream, small request/response streams, and idle streams at the same time.

- Status: `passed`
- Connections: `7`
- Requests: `2610`
- Chunks sent/received: `5229/5229`
- Plaintext bytes sent/received: `1046088/1046088`
- Sealed/plaintext ratio: `1.797`
- Time to open result p50/p95/p99 ms: `4.342/4.442/4.442`

| Operation | Key class | Result | Count | p50 ms | p95 ms | p99 ms | Max ms | Request bytes | Response bytes |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| delete | open | success | 7 | 0.004 | 0.005 | 0.005 | 0.005 | 0 | 0 |
| get | ack-c2s | success | 2561 | 0.001 | 0.006 | 0.045 | 0.250 | 0 | 711958 |
| get | ack-s2c | success | 2561 | 0.001 | 0.005 | 0.030 | 0.928 | 0 | 711958 |
| get | data-c2s | not_found | 10305 | 0.001 | 0.003 | 0.006 | 0.641 | 0 | 0 |
| get | data-c2s | success | 2615 | 0.001 | 0.004 | 0.012 | 1.069 | 0 | 939714 |
| get | data-s2c | not_found | 10324 | 0.001 | 0.003 | 0.007 | 0.927 | 0 | 0 |
| get | data-s2c | success | 2614 | 0.001 | 0.004 | 0.010 | 0.821 | 0 | 939620 |
| get | open | success | 7 | 0.001 | 0.002 | 0.002 | 0.002 | 0 | 2170 |
| get | open-result | not_found | 14 | 0.001 | 0.002 | 0.162 | 0.162 | 0 | 0 |
| get | open-result | success | 7 | 0.001 | 0.002 | 0.002 | 0.002 | 0 | 1802 |
| head | close-client | not_found | 2983 | 0.001 | 0.001 | 0.003 | 1.087 | 0 | 0 |
| head | close-client | success | 7 | 0.002 | 0.003 | 0.003 | 0.003 | 0 | 0 |
| head | close-server | not_found | 2987 | 0.001 | 0.001 | 0.004 | 0.143 | 0 | 0 |
| head | close-server | success | 7 | 0.001 | 0.004 | 0.004 | 0.004 | 0 | 0 |
| list | list-open | success | 494 | 0.138 | 0.302 | 0.652 | 1.221 | 0 | 188096 |
| put | ack-c2s | success | 329 | 0.001 | 0.006 | 0.147 | 0.591 | 91462 | 0 |
| put | ack-s2c | success | 329 | 0.001 | 0.006 | 0.079 | 0.233 | 91462 | 0 |
| put | close-client | success | 7 | 0.001 | 0.002 | 0.002 | 0.002 | 1798 | 0 |
| put | close-server | success | 7 | 0.004 | 0.008 | 0.008 | 0.008 | 1802 | 0 |
| put | data-c2s | success | 2615 | 0.001 | 0.008 | 0.153 | 0.949 | 939714 | 0 |
| put | data-s2c | success | 2614 | 0.001 | 0.007 | 0.119 | 0.993 | 939620 | 0 |
| put | open | success | 7 | 0.002 | 0.025 | 0.025 | 0.025 | 2170 | 0 |
| put | open-result | success | 7 | 0.002 | 0.023 | 0.023 | 0.023 | 1802 | 0 |

Observations:

- HEAD close-marker checks are visible in the request mix.
- LIST open-session polling is part of baseline request volume.

## Operations By Key Class

| Key class | Operation | Count |
| --- | --- | ---: |
| ack-c2s | get | 3728 |
| ack-c2s | put | 502 |
| ack-s2c | get | 3726 |
| ack-s2c | put | 502 |
| close-client | head | 6834 |
| close-client | put | 51 |
| close-server | head | 6856 |
| close-server | put | 51 |
| data-c2s | get | 28213 |
| data-c2s | put | 3834 |
| data-s2c | get | 28300 |
| data-s2c | put | 3833 |
| list-open | list | 2112 |
| open | delete | 51 |
| open | get | 51 |
| open | put | 51 |
| open-result | get | 178 |
| open-result | put | 51 |

## Baseline Before v1 Optimizations

This report records protocol v1 behavior before optimization work. It does not change key layout, ACK behavior, close markers, chunking, polling, or Android compatibility.
