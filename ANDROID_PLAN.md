# s3s5 Android client MVP

You are working in the existing project:

```text
~/s3socks/s3s5
```

The Linux MVP already exists and works. It implements:

```text
Linux client app -> S3-compatible object storage API -> Linux server app
```

Now build the Android client app:

```text
Android client app -> S3-compatible object storage API -> existing Linux server app
```

The Android client must be protocol-compatible with the existing Linux `s3s5-server`. The existing Linux server and protocol are the source of truth. Before implementing Android code, inspect the actual repository, especially:

- `README.md`
- `docs/PROTOCOL.md`
- `docs/SECURITY.md`
- `docs/ANDROID_PORTING.md`
- `docs/PERFORMANCE.md`
- `internal/protocol/`
- `internal/crypto/`
- `internal/tunnel/`
- `internal/objectstore/`
- `internal/socks5/`
- tests and fixtures, if present

Do not assume the original prompt is perfectly synchronized with the code. Match the actual Linux MVP behavior and update docs if you find inconsistencies.

## Product naming

Use the existing product name:

- product: `s3s5`
- Android app display name: `s3s5`
- Android package name: `io.s3s5.android` unless a better existing namespace is already present
- Android project directory: `android-client/`

Do not rename the Linux project.

Do not use the old name `s3socks` except in historical/reference documentation.

## High-level Android scope

Build an Android client MVP with:

- Kotlin-native implementation.
- Android app with minimal UI.
- Foreground service that runs the client.
- Local SOCKS5 listener on Android, default `127.0.0.1:1080`.
- S3-compatible transport to the existing Linux `s3s5-server`.
- Compatible protocol, encryption, object keys, chunking, ACK/window behavior, close semantics, and error handling.
- No Android VPNService in phase 1.
- No device-wide traffic capture in phase 1.
- No root, no accessibility service, no stealth behavior, no boot persistence by default.

The phase 1 use model is:

1. User opens app.
2. User enters/imports S3 config and PSK.
3. User taps Start.
4. App starts a foreground service.
5. Foreground service listens on `127.0.0.1:1080`.
6. User configures a specific app/tool/browser to use SOCKS5 `127.0.0.1:1080`, or uses adb forwarding for tests.
7. Android client talks to S3 and the existing Linux server consumes the S3 mailbox.

Future phase, documented only:

```text
Android VpnService / tun2socks -> s3s5 tunnel -> S3 -> Linux server
```

Do not implement VPNService in this MVP.

## Safety and allowed use

This is for authorized use with buckets, accounts, servers, devices, and networks controlled by the user.

Do not implement:

- stealth
- evasion
- hidden persistence
- malware-like behavior
- credential harvesting
- root escalation
- device-wide traffic interception without Android VPN consent
- boot autostart by default
- accessibility service
- notification hiding
- Play policy bypass
- proxy exposure to LAN by default

The Android client must default to localhost-only listening.

If a LAN listen mode is implemented, it must be explicit, off by default, and show a warning that it exposes a proxy endpoint to the network.

## Model and subagent budget policy

The master/root session may run on `gpt-5.5`, but minimize expensive model use.

Create or update project-scoped custom agents under `.codex/agents/` if they do not already exist. Reuse existing agents if compatible.

Use these model assignments:

### android_repo_surveyor

- model: `gpt-5.3-codex-spark`
- reasoning: medium
- sandbox: read-only
- task:
  - inspect existing Linux MVP
  - identify actual protocol/crypto/objectstore behavior
  - identify Android-portability notes
  - summarize compatibility constraints
- no code changes

### android_planner

- model: `gpt-5.4`
- reasoning: high
- task:
  - design Android architecture
  - decide Gradle/module layout
  - plan compatibility tests

### kotlin_implementation_worker

- model: `gpt-5.4`
- reasoning: medium/high as needed
- task:
  - write Android/Kotlin implementation
  - keep changes focused
  - run tests/builds

### android_test_worker

- model: `gpt-5.4-mini`
- reasoning: medium
- task:
  - write unit tests
  - write instrumented tests where practical
  - triage Gradle/test failures
  - edit tests/scripts; code edits only for obvious harness fixes

### protocol_compat_worker

- model: `gpt-5.4`
- reasoning: high
- task:
  - generate/verify cross-language protocol and crypto compatibility fixtures
  - compare Kotlin Android output with Go server expectations
  - identify mismatch precisely

### android_security_reviewer

- model: `gpt-5.5`
- reasoning: high
- sandbox: read-only unless parent explicitly asks for fixes
- task:
  - review credential storage
  - review PSK handling
  - review no-plaintext S3 objects
  - review logging
  - review foreground service / proxy exposure risks
  - review unsafe defaults

### docs_worker

- model: `gpt-5.4-mini`
- reasoning: medium
- task:
  - update Android docs, README sections, scripts docs
  - docs only

Use the cheapest adequate model:

- `gpt-5.3-codex-spark` for quick read-only repo surveys.
- `gpt-5.4-mini` for docs, tests, Gradle log triage, simple scripts.
- `gpt-5.4` for Kotlin implementation and protocol compatibility.
- `gpt-5.5` only for master planning, difficult blockers, security review, crypto review, and final acceptance.

Do not run multiple write-heavy agents that edit the same files in parallel.

Each subagent summary must include:

- files inspected or changed
- decisions made
- risks/unknowns
- recommended next action

## Implementation language and stack

Use Kotlin for the Android app.

Preferred stack:

- Kotlin
- Gradle Kotlin DSL
- Android app module
- Kotlin coroutines
- AWS SDK for Kotlin for S3-compatible API, if it works with Android and MinIO endpoint override
- Java/Kotlin standard crypto APIs for AES-GCM/HKDF compatibility
- Android Keystore for protecting locally stored secrets/config encryption key
- Jetpack Compose for UI if the local Android toolchain supports it cleanly
- If Compose setup causes toolchain/version friction, use a simple native Android View-based UI instead; working MVP is more important than UI framework choice

Do not use `gomobile` for the MVP unless you first document a strong reason in `android-client/docs/ADR-ANDROID-0001-stack.md` and the parent approves.

Rationale:

- Kotlin-native Android app is easier to maintain as an Android app.
- The existing Go server remains unchanged.
- Protocol compatibility is enforced through fixtures/tests.
- Future gomobile reuse can be revisited later.

## Android project layout

Create:

```text
android-client/
  README.md
  settings.gradle.kts
  build.gradle.kts
  gradle.properties
  app/
    build.gradle.kts
    src/main/
      AndroidManifest.xml
      java/io/s3s5/android/
        MainActivity.kt
        S3S5Application.kt
        service/
          S3S5ForegroundService.kt
          ServiceController.kt
          NotificationHelper.kt
        ui/
          MainScreen.kt
          ConfigScreen.kt
          StatusScreen.kt
          LogScreen.kt
        core/
          config/
          socks5/
          protocol/
          crypto/
          objectstore/
          objectstore/s3/
          objectstore/memory/
          tunnel/
          metrics/
          logging/
        storage/
          SecureConfigStore.kt
          AndroidKeystoreSecrets.kt
        debug/
          DebugConfigImportExport.kt
    src/test/
      java/io/s3s5/android/
    src/androidTest/
      java/io/s3s5/android/
  docs/
    ANDROID_ARCHITECTURE.md
    ANDROID_SECURITY.md
    ANDROID_TESTING.md
    ANDROID_VPNSERVICE_FUTURE.md
    PROTOCOL_COMPATIBILITY.md
  scripts/
    android-build.sh
    android-unit-test.sh
    android-install-debug.sh
    android-emulator-smoke.sh
    android-adb-forward-socks.sh
    android-real-s3-smoke.md
  compat/
    fixtures/
      README.md
```

Adjust structure if needed, but keep clean separation between:

- Android UI
- Android service/lifecycle
- core protocol/tunnel logic
- object store abstraction
- S3 implementation
- crypto
- config storage

## Core Android behavior

### App UI

Implement a minimal but usable UI:

- Connection status:
  - stopped
  - starting
  - running
  - stopping
  - error
- Local SOCKS listen address/port.
- S3 config fields:
  - bucket
  - prefix
  - region
  - endpoint URL optional
  - force path style
  - access key ID
  - secret access key
  - session token optional
  - PSK
- Tunnel settings:
  - chunk size
  - poll min
  - poll max
  - window chunks
  - idle timeout
- Buttons:
  - Save config
  - Start
  - Stop
  - Test S3 / Doctor
  - Copy SOCKS proxy address
  - Clear logs
- Display counters:
  - active sessions
  - bytes sent/received
  - chunks sent/received
  - S3 PUT/GET/HEAD/LIST/DELETE counts
  - last error
  - approximate object round-trip latency if doctor was run

Do not show secrets after save. Use password fields for secrets. Provide a “reveal” toggle only if simple.

### Foreground service

Implement `S3S5ForegroundService`.

Requirements:

- Runs the SOCKS listener and S3 tunnel client.
- Uses an Android foreground notification while running.
- Uses appropriate foreground service declaration and permissions for target SDK.
- For Android 14+ target, declare foreground service type, likely `dataSync`, unless Android docs/tooling indicate a better type.
- Stop service cleanly when user taps Stop.
- Notification must include:
  - running/stopped/error state
  - local listen address
  - stop action
- Service must not start automatically on boot in MVP.
- Service must not hide from user.
- All long-running operations must be coroutine/context cancellable.
- Cleanly close server sockets, active sessions, S3 clients, and coroutines on stop.

### SOCKS5 listener

Implement client-side SOCKS5 listener in Kotlin.

Requirements:

- Default listen: `127.0.0.1:1080`.
- Support RFC1928 no-auth.
- Support CONNECT only.
- Support target address types:
  - IPv4
  - IPv6
  - domain name
- Reject BIND with correct SOCKS5 error.
- Reject UDP ASSOCIATE with correct SOCKS5 error.
- Preserve domain target and send domain to Linux server.
- Do not force Android-side DNS resolution for domain targets unless explicitly configured later.
- For every accepted CONNECT:
  - create a protocol session over S3
  - wait for server open-result
  - only then send SOCKS success/failure reply to local app
  - stream bytes bidirectionally
  - handle close and error propagation

IPv6 must be supported at parser/protocol/policy level. Full public IPv6 e2e is not required in phase 1.

### Protocol compatibility

The Android client must exactly match the existing Linux protocol.

Before coding protocol, inspect actual Go implementation and docs.

Implement Kotlin equivalents for:

- session id format/length
- key layout
- object names
- open request message
- open result message
- data chunk format
- ACK/state format
- close messages
- heartbeat messages if present
- cleanup semantics if relevant
- error codes
- encoding/decoding
- sequence number formatting
- direction labels
- object type labels
- protocol version labels

If docs and Go code disagree, treat Go code/tests as source of truth, update docs, and document the discrepancy in `android-client/docs/PROTOCOL_COMPATIBILITY.md`.

### Crypto compatibility

The Android client must be able to talk to the existing Linux server with encryption enabled.

Requirements:

- PSK encryption enabled by default.
- No plaintext target host/port in S3 objects when crypto is enabled.
- No plaintext traffic data in S3 objects when crypto is enabled.
- Match Linux MVP algorithm exactly.
- Expected design, if Linux matches original plan:
  - HKDF-SHA256 from `S3S5_PSK`
  - per-session/per-direction keys
  - AES-256-GCM or the actual Linux AEAD
  - unique nonce per encrypted object
  - associated data includes protocol version, object type, session id, direction, sequence number where relevant
- Implement compatibility tests using fixtures generated from the Go implementation.
- Tests must cover:
  - decrypt Go-generated object in Kotlin
  - encrypt Kotlin-generated object and decrypt/verify with Go fixture/tool if practical
  - wrong PSK fails
  - tampered ciphertext fails
  - wrong associated data fails
  - wrong session id/direction/object type fails

If the Linux MVP has no fixture generator, create a small Go test/helper under the Linux project only if non-invasive, or add documented fixtures produced by existing tests. Do not break Linux build.

### Object store abstraction

Create Android/Kotlin object store interface similar to Linux:

- putObject
- getObject
- headObject
- listPrefix
- deleteObject
- optional deletePrefix helper

Implement:

- in-memory object store for unit tests
- S3-compatible object store for AWS S3 and MinIO

S3 requirements:

- AWS S3 support.
- MinIO endpoint support.
- Endpoint URL override.
- Force path style option.
- Region config.
- Optional session token.
- Static credentials from saved Android config.
- No secrets in logs.
- Operation counters for PUT/GET/HEAD/LIST/DELETE.
- Timeouts and cancellation.
- Conservative polling defaults.

If AWS SDK for Kotlin cannot meet MinIO/path-style/Android constraints cleanly, document the issue and either:

1. implement a narrow S3-compatible client using OkHttp + AWS SigV4 only for required operations, or
2. use AWS SDK for Java/Kotlin-compatible dependency that works on Android,

but prefer the official AWS SDK for Kotlin first.

### Android config and secret storage

Do not hardcode S3 credentials or PSK.

Do not put secrets in:

- source code
- Gradle files
- BuildConfig
- logs
- crash messages
- screenshots by default
- committed test config

Implement local config storage:

- Non-secret config may be stored in DataStore or SharedPreferences.
- Secrets must be encrypted at rest using a key protected by Android Keystore, or another clearly documented Android-secure approach.
- Store:
  - AWS access key ID
  - AWS secret access key
  - session token
  - PSK
- User must be able to clear saved secrets.
- If secure storage is not fully implemented in first pass, app must not persist secrets by default and must document limitation. But the goal is to implement secure storage.

Use Android Keystore appropriately; avoid using deprecated/fragile crypto wrappers unless justified.

### Network security

Requirements:

- Release builds should not allow arbitrary cleartext HTTP by default.
- Debug builds may allow HTTP endpoints for local MinIO testing.
- If endpoint URL starts with `http://`, show warning in UI.
- Do not bypass TLS validation.
- Do not implement custom trust-all certificate behavior.
- If self-signed MinIO HTTPS support is added, document it and make it explicit.

### Logging

Implement internal logs visible in UI.

Requirements:

- No secrets.
- No traffic payloads.
- Normal logs should avoid full target host unless debug mode is enabled.
- Debug mode may log target host/port but never credentials or PSK.
- Session ids should be shortened.
- Include S3 operation counts.
- Include recent errors.
- Include service lifecycle events.

### Doctor/test connection

Implement Android-side doctor function:

- Validate config fields.
- Try S3 Head/List/Put/Get/Delete under configured prefix.
- Measure approximate round-trip object latency.
- Validate crypto config locally.
- Show success/failure in UI.
- Never print secrets.

## Testing requirements

Implement tests in layers.

### JVM unit tests

Must include:

- SOCKS5 no-auth handshake.
- CONNECT IPv4 parser.
- CONNECT IPv6 parser.
- CONNECT domain parser.
- BIND rejected.
- UDP ASSOCIATE rejected.
- protocol key generation compatibility.
- protocol open request encode/decode.
- protocol open result encode/decode.
- data chunk key formatting.
- sequence number formatting.
- ACK/window behavior.
- close message encode/decode.
- crypto decrypt known Go fixture.
- crypto encrypt fixture or roundtrip equivalent.
- wrong PSK failure.
- tamper failure.
- wrong associated data failure.
- in-memory object store behavior.
- S3 operation counter wrapper behavior.

### Android instrumented tests

Add if practical:

- service starts/stops.
- localhost SOCKS listener binds on ephemeral port.
- memory-store e2e with fake server or local echo logic.

Instrumented tests may be optional in CI if no device/emulator is present, but scripts/docs must explain how to run them.

### Cross-language compatibility fixtures

Create `android-client/compat/fixtures/`.

Include fixtures for:

- open request
- open result
- data chunk c2s
- data chunk s2c
- ACK
- close
- encrypted object examples

Each fixture should include:

- plaintext JSON or binary description where safe
- encrypted bytes/base64 if crypto enabled
- associated data fields
- session id
- object key
- expected decode result
- notes on how it was generated

Do not include real secrets. Use a test-only PSK like:

```text
test-only-psk-do-not-use
```

### Android to Linux server smoke test

Add scripts/docs for an emulator/device smoke test.

Preferred emulator flow:

Terminal 1:

- start MinIO using existing Linux MVP scripts
- start existing Linux `s3s5-server`
- start a local echo server or use `example.com:443` for curl smoke

Terminal 2:

- build and install Android debug APK
- configure Android app with:
  - bucket
  - prefix
  - region
  - endpoint
  - force path style
  - access key
  - secret key
  - PSK
- start Android service

Terminal 3:

- forward Android SOCKS port to host:

```bash
adb forward tcp:1080 tcp:1080
```

- test from host:

```bash
curl --socks5-hostname 127.0.0.1:1080 https://example.com/
```

For emulator endpoint documentation:

- Android emulator can reach host machine as `10.0.2.2`.
- Physical device needs host LAN IP or real S3 endpoint.

Create:

- `android-client/scripts/android-build.sh`
- `android-client/scripts/android-unit-test.sh`
- `android-client/scripts/android-install-debug.sh`
- `android-client/scripts/android-adb-forward-socks.sh`
- `android-client/scripts/android-emulator-smoke.sh` if practical
- `android-client/scripts/android-real-s3-smoke.md`

Do not require real AWS S3 for unit tests.

Real S3 testing must be manual and documented. Never commit credentials.

## Existing Linux MVP protection

Do not break existing Linux MVP.

Before Android work starts, record current status:

```bash
make build
go test ./...
```

If these already fail before Android changes, document the baseline failure and avoid making it worse.

After Android work, verify:

```bash
make build
go test ./...
```

The Linux server must remain compatible with existing Linux client.

Do not modify Linux server protocol unless absolutely necessary for Android compatibility, and only after documenting why. Prefer Android matching the existing protocol.

## Build commands

Android build should support:

```bash
cd android-client
./gradlew :app:assembleDebug
./gradlew :app:testDebugUnitTest
```

If Gradle wrapper is created, use:

```bash
./gradlew
```

If no wrapper is created, document required Gradle/Android Studio commands.

Root project may optionally add Makefile targets:

```make
android-build
android-test
android-install-debug
```

But do not break existing Makefile targets.

## Documentation requirements

Create/update:

- `android-client/README.md`
- `android-client/docs/ANDROID_ARCHITECTURE.md`
- `android-client/docs/ANDROID_SECURITY.md`
- `android-client/docs/ANDROID_TESTING.md`
- `android-client/docs/ANDROID_VPNSERVICE_FUTURE.md`
- `android-client/docs/PROTOCOL_COMPATIBILITY.md`

README must explain:

- what Android MVP does
- what it does not do
- how to build debug APK
- how to configure S3/MinIO
- how to run Android app against Linux server
- how to test with `adb forward`
- how to use real S3 manually
- security warnings about storing S3 credentials on device
- warning that localhost SOCKS does not automatically proxy all Android app traffic
- future VPNService plan

`ANDROID_VPNSERVICE_FUTURE.md` must describe future work:

- VpnService permission flow
- TUN interface
- packet parsing
- TCP stack/tun2socks requirement
- per-app VPN possibility
- always-on VPN considerations
- why this is not part of MVP

## UI acceptance criteria

The debug APK should let a user:

1. Open app.
2. Enter S3/MinIO config.
3. Save config.
4. Run Doctor.
5. Start service.
6. See foreground notification.
7. See status “running”.
8. See local SOCKS address.
9. Stop service.
10. Clear config/secrets.

## Functional acceptance criteria

Completion requires:

1. Existing Linux `make build` still succeeds.
2. Existing Linux `go test ./...` still succeeds.
3. Android debug APK builds.
4. Android JVM unit tests pass.
5. Kotlin protocol key/message tests match Go protocol fixtures.
6. Kotlin crypto tests can read/decrypt at least one Go-generated fixture or equivalent cross-language compatibility artifact.
7. Android SOCKS5 parser supports IPv4, IPv6, and domain targets.
8. Android rejects BIND and UDP ASSOCIATE.
9. Android foreground service can start and stop cleanly.
10. Android local SOCKS listener can bind to localhost.
11. Android client can connect to existing Linux server through S3-compatible object store in at least one documented smoke path.
12. No secrets are committed.
13. No logs print secrets or payloads.
14. Docs explain limitations and testing steps.

## MVP limitations to document

Document clearly:

- Android phase 1 is local SOCKS only.
- It does not automatically proxy all device traffic.
- No VPNService yet.
- No UDP ASSOCIATE.
- No BIND.
- TCP CONNECT only.
- S3 transport is slow and high-latency by design.
- S3 request costs depend on polling, chunk size, number of sessions, and retries.
- IPv6 is supported in parser/protocol, but public IPv6 e2e may be minimally tested.
- Static S3 credentials on a mobile device are risky; this is for controlled testing.
- Production should use scoped credentials, temporary credentials, or a safer provisioning flow.

## Implementation phases

### Phase A0: Baseline and survey

- Run/check Linux `make build` and `go test ./...`.
- Inspect docs and Go implementation.
- Create `android-client/docs/PROTOCOL_COMPATIBILITY.md` initial notes.
- Identify actual protocol/crypto details.
- Do not code Android protocol until survey is complete.

### Phase A1: Android scaffold

- Create `android-client`.
- Configure Gradle/Kotlin/Android app.
- Add minimal MainActivity.
- Add app icon/name if simple.
- Add README stub.
- Ensure debug APK builds.

Gate:

```bash
cd android-client
./gradlew :app:assembleDebug
```

### Phase A2: Core Kotlin protocol and crypto

- Implement target model.
- Implement protocol keys/messages.
- Implement crypto envelope compatible with Go.
- Add fixtures.
- Add unit tests.

Gate:

```bash
cd android-client
./gradlew :app:testDebugUnitTest
```

### Phase A3: Object store

- Implement object store interface.
- Implement memory object store.
- Implement S3 object store.
- Add S3 operation counters.
- Add unit tests for memory store and config validation.

### Phase A4: SOCKS5 and tunnel client

- Implement SOCKS5 listener.
- Implement session lifecycle.
- Implement S3 mailbox tunnel writer/reader.
- Implement ACK/window/backpressure.
- Implement close/error handling.
- Add unit tests and memory e2e tests where practical.

### Phase A5: Android service and UI

- Implement foreground service.
- Implement notifications.
- Implement service controller.
- Implement UI config/status/logs.
- Implement secure config storage.
- Implement Doctor.

### Phase A6: Smoke scripts and docs

- Add adb forward script.
- Add emulator smoke docs/script.
- Add real S3 smoke doc.
- Update Linux README with Android section if appropriate.
- Add Android limitations docs.

### Phase A7: Security review and final verification

- Use android_security_reviewer.
- Fix findings.
- Run:
  - Linux `make build`
  - Linux `go test ./...`
  - Android `assembleDebug`
  - Android unit tests
- Document any skipped integration test and exact reason.

## Final response requirements

At completion, report:

- what was built
- Android architecture
- protocol compatibility status
- crypto compatibility status
- which subagents/models were used
- how to build APK
- how to run unit tests
- how to run emulator/adb smoke test
- how to run real S3 smoke test
- known limitations
- next steps for VPNService phase
