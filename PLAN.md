
# s3s5: SOCKS5-over-S3 API MVP

You are working in ~/s3socks/s3s5.

Build a working MVP product called `s3s5`: “SOCKS5-over-S3 API”.

Current local reference repositories are available at:

- ~/s3socks/File-Tunnel
- ~/s3socks/hev-socks5-server
- ~/s3socks/socks5

Do not modify those repositories. Treat them as references/examples only. Create a new standalone product in the current directory. Before reusing or copying any code, inspect licenses and document compatibility. Prefer a fresh implementation unless there is a clear, license-compatible reason to reuse code.

The final target architecture is:

Android client app -> S3-compatible object storage API -> Linux server app

The current implementation target is:

Linux client app -> S3-compatible object storage API -> Linux server app

The implementation must be designed so that the client-side core can later be ported to Android. Avoid Linux-specific assumptions in core packages. Keep transport, protocol, SOCKS5, encryption, policy, object-store, and CLI layers separated.

## Model allocation and subagent policy

The master/root session may run on `gpt-5.5`, but it must minimize unnecessary expensive model use.

Create project-scoped Codex custom agents under `.codex/agents/` and use explicit model choices:

- `repo_surveyor`
  - model: `gpt-5.3-codex-spark`
  - reasoning: low or medium
  - sandbox: read-only
  - task: inspect reference repositories, licenses, architecture patterns, and summarize findings.
  - no code changes.

- `docs_planner`
  - model: `gpt-5.4-mini`
  - reasoning: medium
  - task: draft/update docs from implementation facts.
  - may edit docs only.

- `protocol_designer`
  - model: `gpt-5.4`
  - reasoning: high
  - task: design S3 mailbox protocol, key layout, chunking, ACK/backpressure, close semantics, and versioning.
  - may edit docs and protocol package only after parent approval.

- `implementation_worker`
  - model: `gpt-5.4`
  - reasoning: medium/high as needed
  - task: implementation and fixes.
  - may edit code and tests.

- `test_worker`
  - model: `gpt-5.4-mini`
  - reasoning: medium
  - task: write tests, inspect failures, summarize logs.
  - may edit tests/scripts; code edits only for obvious test harness fixes.

- `security_reviewer`
  - model: `gpt-5.5`
  - reasoning: high
  - sandbox: read-only unless parent explicitly asks for fixes.
  - task: review crypto, secret handling, destination policy, SSRF/open-relay risks, and safe defaults.

- `android_portability_reviewer`
  - model: `gpt-5.4-mini`
  - reasoning: medium
  - sandbox: read-only
  - task: review portability to Android and document constraints.

Only spawn subagents when useful. Prefer subagents for read-heavy exploration, documentation review, security review, and test-log triage. Avoid parallel write-heavy agents editing the same files. For implementation, prefer one active implementation worker at a time unless work is clearly file-disjoint.

Use the cheapest adequate model:
- Use `gpt-5.3-codex-spark` for quick read-only surveys, command-output summarization, and simple text-only notes.
- Use `gpt-5.4-mini` for docs, tests, simple refactors, and log analysis.
- Use `gpt-5.4` for main coding and protocol implementation.
- Use `gpt-5.5` only for master planning, architecture decisions, difficult blockers, security review, crypto review, and final acceptance.

At the end of each subagent task, require a concise summary with:
- files inspected or changed
- decisions made
- risks/unknowns
- recommended next action

## Safety and scope assumptions

This is for authorized use on systems, buckets, accounts, and networks controlled by the user.

Do not add:
- stealth behavior
- persistence
- evasion
- malware-like behavior
- credential harvesting
- unauthorized bypass features

The server must have safe defaults so it does not accidentally become an unrestricted open relay or SSRF primitive.

The server must default to denying:
- private IPv4 and IPv6 ranges
- loopback IPv4 and IPv6 ranges
- link-local IPv4 and IPv6 ranges
- multicast IPv4 and IPv6 ranges
- unspecified addresses
- cloud metadata addresses, especially 169.254.169.254 and IPv6 equivalents where applicable

The server must support configurable allowlists/denylists for:
- hostnames
- domain suffixes
- CIDRs, IPv4 and IPv6
- ports

Never print AWS/S3 secrets in logs. Never commit test credentials. All real S3 credentials must be read from env vars or config files excluded from git.

## Primary implementation language

Use Go for the Linux MVP unless a better choice is strongly justified in `docs/ADR-0001-stack.md` before coding.

Reasons for Go:
- good TCP/socket support
- easy Linux CLI distribution
- pure userspace, no root required
- good testing support
- AWS SDK availability
- future Android reuse is possible via gomobile or by keeping a protocol-compatible Android/Kotlin implementation

## IPv6 policy

IPv6 is desirable and the architecture must not block it.

MVP IPv6 requirements:
- SOCKS5 parser must support ATYP IPv6.
- Protocol target representation must support IPv4, IPv6, and domain names.
- Policy engine must understand IPv6 CIDRs and deny unsafe IPv6 ranges by default.
- Server dial path should be able to dial IPv6 targets when the host OS/network supports it.
- Unit tests must cover IPv6 address parsing, protocol encoding/decoding, and IPv6 policy allow/deny behavior.

Minimal first-stage IPv6 testing is acceptable:
- Do not require public IPv6 internet availability in automated integration tests.
- Do not require real-S3 IPv6 e2e testing.
- Add optional/manual IPv6 smoke test docs if straightforward.

Use Go types that handle IPv4 and IPv6 cleanly, preferably `net/netip` where appropriate.

## Repository structure

Create a clean Go module with roughly this structure, adjusting if needed:

- cmd/s3s5-client/main.go
- cmd/s3s5-server/main.go
- cmd/s3s5-doctor/main.go
- internal/socks5/
- internal/protocol/
- internal/objectstore/
- internal/objectstore/s3/
- internal/objectstore/memory/
- internal/tunnel/
- internal/crypto/
- internal/policy/
- internal/config/
- docs/
- scripts/
- examples/
- .codex/
- docker-compose.yml
- Makefile

Binaries must be named:

- bin/s3s5-client
- bin/s3s5-server
- bin/s3s5-doctor

## Core behavior

### s3s5-client

The client must:

- Listen on a local SOCKS5 address, default `127.0.0.1:1080`.
- Support SOCKS5 RFC1928 no-auth method.
- Support CONNECT only for MVP.
- Support target address types:
  - IPv4
  - IPv6
  - domain name
- Reject UDP ASSOCIATE and BIND with correct SOCKS5 error replies.
- Preserve domain targets and send the domain to the server. Do not force local DNS resolution unless explicitly configured.
- For each accepted SOCKS5 CONNECT, create a tunnel session over S3.
- Wait for server open-result before replying success/failure to the SOCKS5 client.
- Stream bytes bidirectionally over S3 chunks.
- Handle close, timeout, and error propagation cleanly.

### s3s5-server

The server must:

- Poll the configured S3 prefix for new open sessions.
- For each open request, validate destination policy.
- Open outbound TCP connection to the requested target.
- Send open-result accepted/rejected back through S3.
- Stream bytes bidirectionally between TCP socket and S3 chunks.
- Enforce safe defaults:
  - deny private IPv4/IPv6 ranges by default
  - deny loopback
  - deny link-local
  - deny multicast
  - deny unspecified addresses
  - deny metadata-service IPs
  - configurable allowlist/denylist for hostnames, CIDRs, and ports
  - max concurrent sessions
  - max bytes per session
  - idle timeout
  - connect timeout
- Avoid becoming an unrestricted open relay by accident.

### s3s5-doctor

The doctor CLI must:

- Verify S3 credentials.
- Verify bucket access.
- Verify prefix read/write/delete.
- Verify encryption config.
- Measure approximate object round-trip latency.
- Work with AWS S3 and MinIO-compatible endpoints.
- Never reveal secrets.
- Support JSON output.

## Object store abstraction

Define an `ObjectStore` interface supporting at minimum:

- `PutObject(ctx, key, bytes, options)`
- `GetObject(ctx, key) -> bytes`
- `HeadObject(ctx, key) -> exists/metadata`
- `ListPrefix(ctx, prefix, options) -> keys`
- `DeleteObject(ctx, key)`
- Optional `DeletePrefix` or batch delete helper.

Implement:

- memory object store for unit/integration tests
- S3-compatible object store for AWS S3 and MinIO

S3-compatible requirements:

- Support AWS S3.
- Support MinIO through endpoint URL.
- Support path-style option where needed.
- Support region.
- Support optional session token.
- Credentials/config from env vars:
  - `AWS_ACCESS_KEY_ID`
  - `AWS_SECRET_ACCESS_KEY`
  - `AWS_SESSION_TOKEN` optional
  - `AWS_REGION` or `S3S5_REGION`
  - `S3S5_BUCKET`
  - `S3S5_PREFIX`
  - `S3S5_ENDPOINT` optional
  - `S3S5_FORCE_PATH_STYLE` optional
  - `S3S5_PSK` required by default
- Do not hardcode credentials.
- Do not require real S3 for normal unit tests.

## S3 mailbox protocol

Design and implement protocol v1 around immutable data chunk objects plus small mutable ACK/state objects.

Suggested key layout:

```text
<prefix>/v1/open/<session-id>.json
<prefix>/v1/open-result/<session-id>.json
<prefix>/v1/data/c2s/<session-id>/<seq>.bin
<prefix>/v1/data/s2c/<session-id>/<seq>.bin
<prefix>/v1/ack/c2s/<session-id>.json
<prefix>/v1/ack/s2c/<session-id>.json
<prefix>/v1/close/client/<session-id>.json
<prefix>/v1/close/server/<session-id>.json
<prefix>/v1/heartbeat/client/<session-id>.json
<prefix>/v1/heartbeat/server/<session-id>.json

You may refine this after writing docs/PROTOCOL.md, but keep it simple and testable.

Protocol requirements:

session-id must be random and unpredictable.
object keys must include protocol version.
data chunks must be sequence-numbered, zero-padded, monotonic.
reader should fetch expected next seq by exact key using Head/Get instead of listing for every chunk.
listing is acceptable for server discovery of new sessions.
implement ACK/backpressure so a fast writer cannot create unlimited S3 objects if the other side stalls.
implement configurable window size, default conservative.
implement configurable chunk size, default 64 KiB or 128 KiB.
implement idle timeout.
implement session close semantics.
implement stale session cleanup tooling.
tolerate missing objects during polling.
do not rely on overwriting data objects.
ACK/state objects may be overwritten if that simplifies design.
do not require S3 event notifications, SQS, Lambda, or WebSockets.
must work with plain S3-compatible API only.
Encryption and authentication

Implement PSK-based payload encryption by default.

Requirements:

S3 object contents must not contain plaintext target host, port, or traffic data when PSK is enabled.
Use modern authenticated encryption: AES-256-GCM or ChaCha20-Poly1305.
Derive per-session/per-direction keys from S3S5_PSK using HKDF-SHA256.
Use unique nonces per encrypted object.
Authenticate associated data such as:
protocol version
object type
session-id
direction
sequence number
Provide explicit --insecure-no-crypto only for local development tests.
--insecure-no-crypto must print a warning and must not be default.
Unit test encryption/decryption, wrong-key failure, tamper failure, and replay/key-swap resistance as far as practical.
CLI and config

Implement usable flags and env config.

s3s5-client flags
--listen, default 127.0.0.1:1080
--bucket
--prefix, default s3s5
--region
--endpoint optional
--force-path-style
--psk-env, default S3S5_PSK
--chunk-size
--poll-min
--poll-max
--window-chunks
--idle-timeout
--log-level
--insecure-no-crypto for dev only
s3s5-server flags
--bucket
--prefix, default s3s5
--region
--endpoint optional
--force-path-style
--psk-env, default S3S5_PSK
--poll-min
--poll-max
--window-chunks
--connect-timeout
--idle-timeout
--max-sessions
--max-bytes-per-session
--allow-target, repeatable or config file
--deny-target, repeatable or config file
--allow-private, default false
--log-level
--insecure-no-crypto for dev only
s3s5-doctor flags
same S3 config
--prefix
--cleanup
--latency-rounds
--json
Logging and metrics

Logging requirements:

Use structured logs if easy.
Log session IDs shortened.
Do not log secrets.
Avoid logging full payloads.
Debug logs may include target host/port.
Normal logs should be conservative.

Metrics/logging counters:

chunks sent/received
bytes sent/received
S3 PUT/GET/HEAD/LIST/DELETE counts
approximate latency
active sessions
Testing

Implement tests at multiple levels.

Unit tests:

SOCKS5 handshake parser/response.
IPv4, IPv6, and domain target parsing.
unsupported SOCKS5 command handling.
protocol message encoding/decoding.
object key generation.
sequence handling.
ACK/window behavior.
encryption success/failure.
policy allow/deny logic, including IPv6 CIDRs and unsafe IPv6 ranges.

Integration tests without real S3:

Use memory object store.
Start server and client in same test process.
Start local TCP echo server.
Connect to client SOCKS5 listener.
Verify data roundtrip through the S3 transport abstraction.
Test open failure when policy denies target.
Test timeout/close.

MinIO tests:

docker-compose.yml with MinIO.
scripts/minio-up.sh
scripts/minio-test.sh
scripts/minio-down.sh
Tests should create a temporary bucket/prefix and clean it.
Document how to run.

Real AWS S3 manual test:

scripts/real-s3-smoke.sh
Must read env vars.
Must not print secrets.
Must create a unique test prefix.
Must clean up best-effort.
README must explain that S3 requests cost money and polling/chunk size affects cost.

Manual smoke test target:

Terminal 1:

export AWS_ACCESS_KEY_ID=...
export AWS_SECRET_ACCESS_KEY=...
export AWS_REGION=...
export S3S5_BUCKET=...
export S3S5_PREFIX=s3s5-dev-$(date +%s)
export S3S5_PSK='a long random test passphrase'
./bin/s3s5-server --bucket "$S3S5_BUCKET" --prefix "$S3S5_PREFIX" --region "$AWS_REGION"

Terminal 2:

export AWS_ACCESS_KEY_ID=...
export AWS_SECRET_ACCESS_KEY=...
export AWS_REGION=...
export S3S5_BUCKET=...
export S3S5_PREFIX=...
export S3S5_PSK='same long random test passphrase'
./bin/s3s5-client --listen 127.0.0.1:1080 --bucket "$S3S5_BUCKET" --prefix "$S3S5_PREFIX" --region "$AWS_REGION"

Terminal 3:

curl --socks5-hostname 127.0.0.1:1080 https://example.com/

Also provide a local MinIO example.

Performance expectations

This will be slow and high-latency by design.

Optimize for correctness and observability first.

Polling should use adaptive backoff:

low delay when active
higher delay when idle

Defaults should avoid excessive request costs.

Add docs/PERFORMANCE.md explaining tradeoffs:

chunk size
poll interval
request cost
throughput
latency
S3 consistency assumptions
why S3-only transport is slow but usable for bulk-ish TCP streams
Required files

Create these files at minimum:

README.md
docs/PROTOCOL.md
docs/SECURITY.md
docs/ANDROID_PORTING.md
docs/PERFORMANCE.md
docs/reference-survey.md
docs/ADR-0001-stack.md
Makefile
docker-compose.yml
scripts/minio-up.sh
scripts/minio-down.sh
scripts/minio-test.sh
scripts/real-s3-smoke.sh
.gitignore
.codex/config.toml
.codex/agents/repo-surveyor.toml
.codex/agents/docs-planner.toml
.codex/agents/protocol-designer.toml
.codex/agents/implementation-worker.toml
.codex/agents/test-worker.toml
.codex/agents/security-reviewer.toml
.codex/agents/android-portability-reviewer.toml
go.mod
cmd/s3s5-client/main.go
cmd/s3s5-server/main.go
cmd/s3s5-doctor/main.go
internal packages as needed

Makefile targets:

make build
make test
make test-race if practical
make lint if a linter is added
make minio-up
make minio-test
make minio-down
make clean
Quality bar
go test ./... must pass.
make build must produce binaries under ./bin:
bin/s3s5-client
bin/s3s5-server
bin/s3s5-doctor
The memory-store end-to-end test must prove that SOCKS5 CONNECT data can traverse:
client
object store
server
TCP target
back
Code must have clear package boundaries.
Protocol behavior must be commented where non-obvious.
README must be sufficient for a new developer to run MinIO and real-S3 smoke tests.
Docs must explicitly state MVP limitations:
CONNECT only
TCP only
no UDP ASSOCIATE yet
no Android UI yet
not optimized for low latency
IPv6 supported in protocol/parser/policy, but minimally e2e-tested in phase 1
single-server consumer for MVP unless safe claiming is implemented
Before finishing, run all available tests and builds. If anything fails, fix it or document exact failure and reason.
Implementation phases
Phase 0: Initialize
Create AGENTS.md with project instructions based on this plan.
Create .codex/config.toml.
Create .codex/agents/*.toml.
Create docs/ADR-0001-stack.md.
Create module skeleton, Makefile, README stub.
Phase 1: Survey references

Use repo_surveyor where possible.

Inspect the three reference repos.
Document relevant lessons in docs/reference-survey.md.
Include license notes.
Do not copy code at this stage.
Phase 2: Protocol and design docs

Use protocol_designer, security_reviewer, and android_portability_reviewer where useful.

Write docs/PROTOCOL.md.
Write docs/SECURITY.md.
Write docs/ANDROID_PORTING.md.
Write docs/PERFORMANCE.md.
Keep docs aligned with code as implementation evolves.
Phase 3: Core libraries

Implement:

config/env parsing
objectstore interface
memory object store
crypto envelope
protocol keys/messages
target representation for IPv4, IPv6, and domains
policy validation for IPv4/IPv6/domain/port rules
Phase 4: Tunnel and SOCKS

Implement:

SOCKS5 CONNECT listener
client session lifecycle
server session lifecycle
bidirectional chunk streaming
ACK/backpressure
close/error handling
Phase 5: S3/MinIO

Implement:

S3-compatible object store
docker-compose MinIO
scripts
doctor CLI
Phase 6: Tests

Implement:

unit tests
memory e2e tests
MinIO integration tests
manual real-S3 smoke script
minimal IPv6 unit tests and optional manual IPv6 smoke docs
Phase 7: Hardening

Implement/review:

safe defaults
log sanitization
metrics counters
cleanup command/script
race/deadlock review
documentation pass
Autonomy rules
Make reasonable engineering decisions without asking the user unless blocked by missing secrets for real S3 tests.
Do not wait for S3 credentials to implement the product.
Use memory store and MinIO for automated tests.
When real S3 credentials are needed, use env vars and provide commands for the user to run locally.
If a design tradeoff is uncertain, choose the simpler MVP design, document the tradeoff, and keep interfaces extensible.
Prefer working code over over-engineered abstractions.
Keep changes reviewable.
Never commit secrets.
Keep reference repos unmodified.
Completion criteria

The goal is complete only when:

make build succeeds.
go test ./... succeeds.
A memory-store e2e SOCKS5 test passes.
MinIO workflow is documented and scripted.
README contains working local and real-S3 examples.
Security defaults prevent accidental open relay behavior.
IPv6 is supported in parser/protocol/policy and minimally unit-tested.
Android-porting notes are written.
No credentials or secrets are committed.
Reference repos remain unmodified.

At final response, summarize:

what was built
model/subagent usage
how to build
how to run MinIO test
how to run real S3 smoke test
known limitations
next steps for Android client app

