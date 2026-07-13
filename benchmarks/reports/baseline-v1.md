# s3s5 Performance Baseline

## Environment

- Commit: `efd651ba0843`
- Dirty worktree: `true`
- Timestamp: `2026-07-13T13:20:59Z`
- Go: `go1.22.2`
- OS/arch: `linux/amd64`
- Profile/provider: `memory` / `memory`

## Protocol Configuration

| Chunk size | Poll min | Poll max | Window chunks | Idle timeout |
| ---: | ---: | ---: | ---: | ---: |
| 65536 | 1ms | 20ms | 16 | 3s |

## Scenario Summary

| Scenario | Status | Bytes sent | Bytes received | Duration ms | PUT | GET hit | GET miss | HEAD | LIST | DELETE | Ops/session | Ops/MiB |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| one-byte-echo-active | passed | 1 | 1 | 29 | 8 | 4 | 16 | 6 | 6 | 1 | 41.00 | 42991616.00 |
| one-byte-echo-after-idle | passed | 1 | 1 | 99 | 8 | 4 | 26 | 8 | 10 | 1 | 57.00 | 59768832.00 |
| small-chatty-writes | passed | 10000 | 10000 | 968 | 230 | 370 | 639 | 204 | 52 | 1 | 1496.00 | 156866.97 |
| bulk-one-mib | passed | 1048576 | 1048576 | 47 | 50 | 50 | 22 | 7 | 7 | 1 | 137.00 | 137.00 |
| short-connections | passed | 160 | 160 | 327 | 160 | 80 | 369 | 121 | 77 | 20 | 41.35 | 5419827.20 |
| concurrent-idle-sessions | passed | 0 | 0 | 387 | 80 | 40 | 667 | 185 | 65 | 20 | 52.85 | 0.00 |
| mixed-traffic | passed | 262414 | 262414 | 271 | 114 | 86 | 255 | 106 | 18 | 7 | 83.71 | 2341.59 |

## Idle Scenario Normalization

| Scenario | Sessions | Idle duration ms | GET/s | HEAD/s | LIST/s | Ops/session/s |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| concurrent-idle-sessions | 20 | 100 | 7070.00 | 1850.00 | 650.00 | 528.50 |

## one-byte-echo-active

Open one SOCKS connection, send one byte, and read one echoed byte without a long idle period.

- Status: `passed`
- Connections: `1`
- Requests: `1`
- Chunks sent/received: `2/2`
- Plaintext bytes sent/received: `2/2`
- Sealed/plaintext ratio: `94.000`
- Time to open result p50/p95/p99 ms: `4.226/4.226/4.226`

| Operation | Key class | Result | Count | p50 ms | p95 ms | p99 ms | Max ms | Request bytes | Response bytes |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| delete | open | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| get | data-c2s | not_found | 7 | 0.006 | 0.006 | 0.006 | 0.006 | 0 | 0 |
| get | data-c2s | success | 1 | 0.006 | 0.006 | 0.006 | 0.006 | 0 | 94 |
| get | data-s2c | not_found | 7 | 0.006 | 0.007 | 0.007 | 0.007 | 0 | 0 |
| get | data-s2c | success | 1 | 0.003 | 0.003 | 0.003 | 0.003 | 0 | 94 |
| get | open | success | 1 | 0.002 | 0.002 | 0.002 | 0.002 | 0 | 310 |
| get | open-result | not_found | 2 | 0.002 | 0.002 | 0.002 | 0.002 | 0 | 0 |
| get | open-result | success | 1 | 0.007 | 0.007 | 0.007 | 0.007 | 0 | 258 |
| head | close-client | not_found | 2 | 0.004 | 0.004 | 0.004 | 0.004 | 0 | 0 |
| head | close-client | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-server | not_found | 2 | 0.003 | 0.003 | 0.003 | 0.003 | 0 | 0 |
| head | close-server | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| list | list-open | success | 6 | 0.010 | 0.030 | 0.030 | 0.030 | 0 | 355 |
| put | ack-c2s | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 278 | 0 |
| put | ack-s2c | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 278 | 0 |
| put | close-client | success | 1 | 0.006 | 0.006 | 0.006 | 0.006 | 258 | 0 |
| put | close-server | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 254 | 0 |
| put | data-c2s | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 94 | 0 |
| put | data-s2c | success | 1 | 0.006 | 0.006 | 0.006 | 0.006 | 94 | 0 |
| put | open | success | 1 | 0.007 | 0.007 | 0.007 | 0.007 | 310 | 0 |
| put | open-result | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 258 | 0 |

Observations:

- HEAD close-marker checks are visible in the request mix.
- LIST open-session polling is part of baseline request volume.

## one-byte-echo-after-idle

Open one SOCKS connection, wait for polling backoff, then send and echo one byte.

- Status: `passed`
- Connections: `1`
- Requests: `1`
- Chunks sent/received: `2/2`
- Plaintext bytes sent/received: `2/2`
- Sealed/plaintext ratio: `94.000`
- Time to open result p50/p95/p99 ms: `3.845/3.845/3.845`

| Operation | Key class | Result | Count | p50 ms | p95 ms | p99 ms | Max ms | Request bytes | Response bytes |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| delete | open | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| get | data-c2s | not_found | 12 | 0.001 | 0.002 | 0.002 | 0.002 | 0 | 0 |
| get | data-c2s | success | 1 | 0.002 | 0.002 | 0.002 | 0.002 | 0 | 94 |
| get | data-s2c | not_found | 12 | 0.001 | 0.002 | 0.004 | 0.004 | 0 | 0 |
| get | data-s2c | success | 1 | 0.002 | 0.002 | 0.002 | 0.002 | 0 | 94 |
| get | open | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 310 |
| get | open-result | not_found | 2 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| get | open-result | success | 1 | 0.002 | 0.002 | 0.002 | 0.002 | 0 | 254 |
| head | close-client | not_found | 3 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-client | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-server | not_found | 3 | 0.000 | 0.000 | 0.000 | 0.000 | 0 | 0 |
| head | close-server | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| list | list-open | success | 10 | 0.002 | 0.004 | 0.004 | 0.004 | 0 | 600 |
| put | ack-c2s | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 278 | 0 |
| put | ack-s2c | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 278 | 0 |
| put | close-client | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 258 | 0 |
| put | close-server | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 258 | 0 |
| put | data-c2s | success | 1 | 0.003 | 0.003 | 0.003 | 0.003 | 94 | 0 |
| put | data-s2c | success | 1 | 0.010 | 0.010 | 0.010 | 0.010 | 94 | 0 |
| put | open | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 310 | 0 |
| put | open-result | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 254 | 0 |

Observations:

- HEAD close-marker checks are visible in the request mix.
- LIST open-session polling is part of baseline request volume.

## small-chatty-writes

Send 100 ordered 100-byte writes with 5ms spacing and verify echoed order.

- Status: `passed`
- Connections: `1`
- Requests: `100`
- Chunks sent/received: `200/200`
- Plaintext bytes sent/received: `20000/20000`
- Sealed/plaintext ratio: `2.260`
- Time to open result p50/p95/p99 ms: `3.672/3.672/3.672`

| Operation | Key class | Result | Count | p50 ms | p95 ms | p99 ms | Max ms | Request bytes | Response bytes |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| delete | open | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| get | ack-c2s | success | 84 | 0.001 | 0.006 | 0.007 | 0.009 | 0 | 23352 |
| get | ack-s2c | success | 84 | 0.001 | 0.006 | 0.016 | 0.027 | 0 | 23352 |
| get | data-c2s | not_found | 318 | 0.001 | 0.003 | 0.008 | 0.014 | 0 | 0 |
| get | data-c2s | success | 100 | 0.001 | 0.007 | 0.008 | 0.017 | 0 | 22600 |
| get | data-s2c | not_found | 319 | 0.001 | 0.004 | 0.006 | 0.009 | 0 | 0 |
| get | data-s2c | success | 100 | 0.001 | 0.006 | 0.009 | 0.021 | 0 | 22600 |
| get | open | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 310 |
| get | open-result | not_found | 2 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| get | open-result | success | 1 | 0.002 | 0.002 | 0.002 | 0.002 | 0 | 258 |
| head | close-client | not_found | 101 | 0.001 | 0.001 | 0.005 | 0.058 | 0 | 0 |
| head | close-client | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-server | not_found | 101 | 0.001 | 0.001 | 0.003 | 0.003 | 0 | 0 |
| head | close-server | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| list | list-open | success | 52 | 0.009 | 0.018 | 0.038 | 0.039 | 0 | 3500 |
| put | ack-c2s | success | 13 | 0.001 | 0.003 | 0.006 | 0.006 | 3610 | 0 |
| put | ack-s2c | success | 13 | 0.001 | 0.001 | 0.002 | 0.002 | 3614 | 0 |
| put | close-client | success | 1 | 0.002 | 0.002 | 0.002 | 0.002 | 258 | 0 |
| put | close-server | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 258 | 0 |
| put | data-c2s | success | 100 | 0.001 | 0.007 | 0.013 | 0.015 | 22600 | 0 |
| put | data-s2c | success | 100 | 0.001 | 0.006 | 0.017 | 0.031 | 22600 | 0 |
| put | open | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 310 | 0 |
| put | open-result | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 258 | 0 |

Observations:

- HEAD close-marker checks are visible in the request mix.
- LIST open-session polling is part of baseline request volume.

## bulk-one-mib

Send and echo one contiguous MiB, verifying SHA-256 and exact byte count.

- Status: `passed`
- Connections: `1`
- Requests: `1`
- Chunks sent/received: `40/40`
- Plaintext bytes sent/received: `2097152/2097152`
- Sealed/plaintext ratio: `1.335`
- Time to open result p50/p95/p99 ms: `3.741/3.741/3.741`

| Operation | Key class | Result | Count | p50 ms | p95 ms | p99 ms | Max ms | Request bytes | Response bytes |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| delete | open | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| get | ack-c2s | not_found | 3 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| get | ack-c2s | success | 4 | 0.002 | 0.002 | 0.002 | 0.002 | 0 | 1112 |
| get | ack-s2c | success | 4 | 0.001 | 0.002 | 0.002 | 0.002 | 0 | 1108 |
| get | data-c2s | not_found | 8 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| get | data-c2s | success | 20 | 0.008 | 0.054 | 0.074 | 0.074 | 0 | 1399944 |
| get | data-s2c | not_found | 9 | 0.001 | 0.029 | 0.029 | 0.029 | 0 | 0 |
| get | data-s2c | success | 20 | 0.007 | 0.069 | 0.148 | 0.148 | 0 | 1399944 |
| get | open | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 310 |
| get | open-result | not_found | 2 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| get | open-result | success | 1 | 0.002 | 0.002 | 0.002 | 0.002 | 0 | 258 |
| head | close-client | not_found | 2 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-client | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-server | not_found | 3 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-server | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| list | list-open | success | 7 | 0.002 | 0.004 | 0.004 | 0.004 | 0 | 378 |
| put | ack-c2s | success | 3 | 0.001 | 0.001 | 0.001 | 0.001 | 834 | 0 |
| put | ack-s2c | success | 3 | 0.001 | 0.001 | 0.001 | 0.001 | 830 | 0 |
| put | close-client | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 254 | 0 |
| put | close-server | success | 1 | 0.001 | 0.001 | 0.001 | 0.001 | 258 | 0 |
| put | data-c2s | success | 20 | 0.007 | 0.089 | 0.099 | 0.099 | 1399944 | 0 |
| put | data-s2c | success | 20 | 0.006 | 0.078 | 0.132 | 0.132 | 1399944 | 0 |
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
- Time to open result p50/p95/p99 ms: `8.264/16.532/16.956`

| Operation | Key class | Result | Count | p50 ms | p95 ms | p99 ms | Max ms | Request bytes | Response bytes |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| delete | open | success | 20 | 0.001 | 0.001 | 0.002 | 0.002 | 0 | 0 |
| get | data-c2s | not_found | 155 | 0.001 | 0.003 | 0.006 | 0.006 | 0 | 0 |
| get | data-c2s | success | 20 | 0.001 | 0.002 | 0.005 | 0.005 | 0 | 2040 |
| get | data-s2c | not_found | 158 | 0.001 | 0.005 | 0.008 | 0.021 | 0 | 0 |
| get | data-s2c | success | 20 | 0.001 | 0.006 | 0.009 | 0.009 | 0 | 2040 |
| get | open | success | 20 | 0.001 | 0.003 | 0.004 | 0.004 | 0 | 6200 |
| get | open-result | not_found | 56 | 0.002 | 0.003 | 0.005 | 0.008 | 0 | 0 |
| get | open-result | success | 20 | 0.002 | 0.003 | 0.009 | 0.009 | 0 | 5152 |
| head | close-client | not_found | 40 | 0.001 | 0.001 | 0.003 | 0.003 | 0 | 0 |
| head | close-client | success | 20 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-server | not_found | 41 | 0.001 | 0.003 | 0.003 | 0.003 | 0 | 0 |
| head | close-server | success | 20 | 0.001 | 0.001 | 0.004 | 0.004 | 0 | 0 |
| list | list-open | success | 77 | 0.004 | 0.012 | 0.015 | 0.015 | 0 | 10472 |
| put | ack-c2s | success | 20 | 0.001 | 0.001 | 0.002 | 0.002 | 5536 | 0 |
| put | ack-s2c | success | 20 | 0.001 | 0.005 | 0.007 | 0.007 | 5552 | 0 |
| put | close-client | success | 20 | 0.001 | 0.004 | 0.031 | 0.031 | 5156 | 0 |
| put | close-server | success | 20 | 0.001 | 0.003 | 0.003 | 0.003 | 5156 | 0 |
| put | data-c2s | success | 20 | 0.001 | 0.004 | 0.005 | 0.005 | 2040 | 0 |
| put | data-s2c | success | 20 | 0.001 | 0.003 | 0.006 | 0.006 | 2040 | 0 |
| put | open | success | 20 | 0.001 | 0.006 | 0.006 | 0.006 | 6200 | 0 |
| put | open-result | success | 20 | 0.001 | 0.003 | 0.036 | 0.036 | 5152 | 0 |

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
- Time to open result p50/p95/p99 ms: `3.760/8.650/8.792`

| Operation | Key class | Result | Count | p50 ms | p95 ms | p99 ms | Max ms | Request bytes | Response bytes |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| delete | open | success | 20 | 0.001 | 0.001 | 0.004 | 0.004 | 0 | 0 |
| get | data-c2s | not_found | 296 | 0.001 | 0.002 | 0.006 | 0.009 | 0 | 0 |
| get | data-s2c | not_found | 324 | 0.001 | 0.003 | 0.009 | 0.036 | 0 | 0 |
| get | open | success | 20 | 0.001 | 0.002 | 0.072 | 0.072 | 0 | 6200 |
| get | open-result | not_found | 47 | 0.001 | 0.002 | 0.002 | 0.002 | 0 | 0 |
| get | open-result | success | 20 | 0.001 | 0.003 | 0.003 | 0.003 | 0 | 5156 |
| head | close-client | not_found | 69 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| head | close-client | success | 20 | 0.001 | 0.001 | 0.003 | 0.003 | 0 | 0 |
| head | close-server | not_found | 76 | 0.001 | 0.001 | 0.001 | 0.007 | 0 | 0 |
| head | close-server | success | 20 | 0.001 | 0.003 | 0.003 | 0.003 | 0 | 0 |
| list | list-open | success | 65 | 0.004 | 0.008 | 0.027 | 0.104 | 0 | 53700 |
| put | close-client | success | 20 | 0.002 | 0.026 | 0.027 | 0.027 | 5148 | 0 |
| put | close-server | success | 20 | 0.001 | 0.002 | 0.006 | 0.006 | 5156 | 0 |
| put | open | success | 20 | 0.001 | 0.003 | 0.003 | 0.003 | 6200 | 0 |
| put | open-result | success | 20 | 0.001 | 0.005 | 0.006 | 0.006 | 5156 | 0 |

Observations:

- GET not_found polling misses are at least half of observed object-store operations.
- HEAD close-marker checks are visible in the request mix.
- LIST open-session polling is part of baseline request volume.

## mixed-traffic

Run one bulk stream, small request/response streams, and idle streams at the same time.

- Status: `passed`
- Connections: `7`
- Requests: `31`
- Chunks sent/received: `72/72`
- Plaintext bytes sent/received: `524828/524828`
- Sealed/plaintext ratio: `1.346`
- Time to open result p50/p95/p99 ms: `3.483/3.628/3.628`

| Operation | Key class | Result | Count | p50 ms | p95 ms | p99 ms | Max ms | Request bytes | Response bytes |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| delete | open | success | 7 | 0.001 | 0.002 | 0.002 | 0.002 | 0 | 0 |
| get | data-c2s | not_found | 112 | 0.001 | 0.003 | 0.017 | 0.316 | 0 | 0 |
| get | data-c2s | success | 36 | 0.001 | 0.010 | 0.037 | 0.037 | 0 | 353260 |
| get | data-s2c | not_found | 129 | 0.001 | 0.003 | 0.007 | 0.008 | 0 | 0 |
| get | data-s2c | success | 36 | 0.001 | 0.010 | 0.038 | 0.038 | 0 | 353260 |
| get | open | success | 7 | 0.001 | 0.002 | 0.002 | 0.002 | 0 | 2170 |
| get | open-result | not_found | 14 | 0.001 | 0.002 | 0.002 | 0.002 | 0 | 0 |
| get | open-result | success | 7 | 0.001 | 0.002 | 0.002 | 0.002 | 0 | 1802 |
| head | close-client | not_found | 44 | 0.001 | 0.002 | 0.010 | 0.010 | 0 | 0 |
| head | close-client | success | 7 | 0.001 | 0.014 | 0.014 | 0.014 | 0 | 0 |
| head | close-server | not_found | 48 | 0.001 | 0.001 | 0.002 | 0.002 | 0 | 0 |
| head | close-server | success | 7 | 0.001 | 0.001 | 0.001 | 0.001 | 0 | 0 |
| list | list-open | success | 18 | 0.011 | 0.017 | 0.036 | 0.036 | 0 | 3968 |
| put | ack-c2s | success | 7 | 0.001 | 0.001 | 0.001 | 0.001 | 1946 | 0 |
| put | ack-s2c | success | 7 | 0.001 | 0.002 | 0.002 | 0.002 | 1942 | 0 |
| put | close-client | success | 7 | 0.001 | 0.003 | 0.003 | 0.003 | 1806 | 0 |
| put | close-server | success | 7 | 0.001 | 0.015 | 0.015 | 0.015 | 1806 | 0 |
| put | data-c2s | success | 36 | 0.001 | 0.026 | 0.270 | 0.270 | 353260 | 0 |
| put | data-s2c | success | 36 | 0.001 | 0.010 | 0.075 | 0.075 | 353260 | 0 |
| put | open | success | 7 | 0.021 | 0.042 | 0.042 | 0.042 | 2170 | 0 |
| put | open-result | success | 7 | 0.004 | 0.263 | 0.263 | 0.263 | 1802 | 0 |

Observations:

- HEAD close-marker checks are visible in the request mix.
- LIST open-session polling is part of baseline request volume.

## Operations By Key Class

| Key class | Operation | Count |
| --- | --- | ---: |
| ack-c2s | get | 91 |
| ack-c2s | put | 45 |
| ack-s2c | get | 88 |
| ack-s2c | put | 45 |
| close-client | head | 312 |
| close-client | put | 51 |
| close-server | head | 325 |
| close-server | put | 51 |
| data-c2s | get | 1086 |
| data-c2s | put | 178 |
| data-s2c | get | 1136 |
| data-s2c | put | 178 |
| list-open | list | 235 |
| open | delete | 51 |
| open | get | 51 |
| open | put | 51 |
| open-result | get | 176 |
| open-result | put | 51 |

## Baseline Before v1 Optimizations

This report records protocol v1 behavior before optimization work. It does not change key layout, ACK behavior, close markers, chunking, polling, or Android compatibility.
