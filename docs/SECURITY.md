# Security

`s3s5` is designed for authorized environments only.

## Safe defaults

The server denies these destination classes by default:

- private IPv4 and IPv6 ranges
- loopback IPv4 and IPv6 ranges
- link-local IPv4 and IPv6 ranges
- multicast IPv4 and IPv6 ranges
- unspecified addresses
- cloud metadata addresses, including `169.254.169.254` and `fd00:ec2::254`

The server also supports allowlists and denylists for:

- hostnames
- domain suffixes
- CIDRs
- ports

These defaults are there to reduce open-relay and SSRF risk. They are not a substitute for network policy at the bucket, endpoint, or firewall layer.

The server also requires an explicit allow policy before it forwards traffic: either repeat `--allow-target` rules or `--allow-unrestricted-egress` for fully controlled deployments.

## Crypto

Payload encryption is enabled by default through a PSK-derived AES-256-GCM envelope.

Security properties:

- plaintext target data and traffic are not stored in S3 when crypto is enabled
- per-session keys are derived with HKDF-SHA256
- each object uses a random nonce
- associated data binds the ciphertext to the protocol context

Operational guidance:

- use a long random `S3S5_PSK`
- do not reuse test PSKs in production
- do not enable `--insecure-no-crypto` outside local development
- the tunnel layer requires an explicit codec; `NoopCodec` is only for local insecure tests

## Credentials and logs

- read real S3 credentials from environment variables or untracked config
- never print AWS or S3 secrets in logs
- never commit test credentials
- avoid logging full payloads
- keep debug logs conservative even when diagnosing transport failures

## Relay risk

The server can become a dangerous relay if policy is loosened too far.

The main risk flags are:

- `--allow-private`
- `--allow-target` with broad hostname, suffix, CIDR, or port scope
- `--allow-unrestricted-egress`

Use those only when the deployment is fully controlled and the target reachability is intentional.

## Current gaps

This MVP documents the intended safety posture, but it does not provide a full enterprise security story yet.

Missing pieces include:

- secret rotation workflows
- centralized policy distribution
- audit-grade logging
- key management integration beyond a PSK environment variable

Those are out of scope for the current repo state.
