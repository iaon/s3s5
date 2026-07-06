# s3s5

`s3s5` is a SOCKS5-over-S3 MVP for Linux and Android.

The current shape is:

`SOCKS5 client -> S3-compatible object store -> TCP server`

The Linux command-line tools are written in Go. The Android client is a Kotlin-native app under `android-client/` that speaks the same S3 mailbox protocol as the Go server.

## Status

This repository is still an MVP. The implementation intentionally favors correctness, explicit policy, and observability over speed.

Current limits:

- Linux client and server
- Android client MVP with local SOCKS5 listener only
- SOCKS5 `CONNECT` only
- no UDP ASSOCIATE
- no SOCKS5 BIND
- Android has no VPNService, no device-wide proxying, and no boot autostart
- `s3s5-server` requires `--allow-target` or `--allow-unrestricted-egress`
- polling-based transport, so latency is inherently high
- S3 request volume directly affects cost
- `--insecure-no-crypto` is for local development only

## What it uses

- `cmd/s3s5-client`
- `cmd/s3s5-server`
- `cmd/s3s5-doctor`
- `android-client/`
- `internal/socks5`, `internal/protocol`, `internal/crypto`, `internal/policy`, `internal/tunnel`

Design notes:

- protocol objects are versioned and key names include `v1`
- payloads are encrypted by default with PSK-derived AES-256-GCM
- the server rejects unsafe destinations by default
- IPv6 is supported in the protocol and policy layer

See:

- [docs/ADR-0001-stack.md](docs/ADR-0001-stack.md)
- [docs/PROTOCOL.md](docs/PROTOCOL.md)
- [docs/SECURITY.md](docs/SECURITY.md)
- [docs/PERFORMANCE.md](docs/PERFORMANCE.md)
- [docs/CI.md](docs/CI.md)
- [docs/RELEASE.md](docs/RELEASE.md)
- [docs/ANDROID_PORTING.md](docs/ANDROID_PORTING.md)
- [docs/reference-survey.md](docs/reference-survey.md)
- [android-client/README.md](android-client/README.md)
- [android-client/docs/PROTOCOL_COMPATIBILITY.md](android-client/docs/PROTOCOL_COMPATIBILITY.md)
- [CONTRIBUTING.md](CONTRIBUTING.md)
- [SECURITY.md](SECURITY.md)

## Quick start

Set a pre-shared key and the S3 variables first:

```bash
export S3S5_PROVIDER=aws
export AWS_ACCESS_KEY_ID=...
export AWS_SECRET_ACCESS_KEY=...
export AWS_REGION=us-east-1
export S3S5_BUCKET=your-bucket
export S3S5_PREFIX=s3s5-dev
export S3S5_PSK='use-a-long-random-passphrase'
```

Run the server:

```bash
./bin/s3s5-server --bucket "$S3S5_BUCKET" --prefix "$S3S5_PREFIX" --region "$AWS_REGION" --allow-target example.com:443
```

Run the client:

```bash
./bin/s3s5-client --listen 127.0.0.1:1080 --bucket "$S3S5_BUCKET" --prefix "$S3S5_PREFIX" --region "$AWS_REGION"
```

Test through SOCKS5:

```bash
curl --socks5-hostname 127.0.0.1:1080 https://example.com/
```

## MinIO example

MinIO is useful for local development because it avoids AWS request charges and keeps latency predictable.

Typical environment:

```bash
export S3S5_PROVIDER=minio
export AWS_ACCESS_KEY_ID=minioadmin
export AWS_SECRET_ACCESS_KEY=minioadmin
export S3S5_BUCKET=s3s5
export S3S5_PREFIX=s3s5-dev
export S3S5_PSK='use-a-long-random-passphrase'
```

With MinIO running locally, start the server and client with the same bucket, prefix, and PSK. The `doctor` command should then verify put/head/get/list/delete behavior against the same endpoint.
When the server is expected to forward traffic, include an explicit allow policy such as `--allow-target example.com:443`; use `--allow-unrestricted-egress` only in fully controlled environments.

To clear a test prefix with the doctor CLI, use `--cleanup-prefix`.

## Yandex Object Storage

Yandex Cloud is supported with a provider preset:

```bash
export S3S5_PROVIDER=yandex
export AWS_ACCESS_KEY_ID=<yandex static key id>
export AWS_SECRET_ACCESS_KEY=<yandex static secret>
export S3S5_REGION=ru-central1
export S3S5_BUCKET=your-bucket
export S3S5_PSK='use-a-long-random-passphrase'
./scripts/yandex-s3-smoke.sh
```

The preset uses `https://storage.yandexcloud.net`, path-style URLs, and signing region `ru-central1`. If `AWS_REGION=ru-central1-a` is present, the Yandex preset normalizes it to `ru-central1`.

For another S3-compatible provider, use `S3S5_PROVIDER=custom` with `S3S5_ENDPOINT`, `S3S5_REGION`, and `S3S5_FORCE_PATH_STYLE=true` when the provider requires path-style URLs.

## Real S3 caveats

Every polling round, control object, ACK object, and data chunk is an S3 request. That means:

- more aggressive polling increases request cost
- smaller chunks increase request count
- tighter windows can reduce burst size but also reduce throughput
- latency is dominated by object-store round trips, not by TCP socket latency

This is suitable for bulk-ish TCP traffic and controlled environments. It is not a low-latency replacement for SSH tunnels, VPNs, or direct TCP relays.

## Testing

`go test ./...` may need permission to bind loopback sockets in restricted sandboxes because the integration tests start local listeners.

Useful targets:

```bash
make build
go test ./...
make test-race
```

## Versioning

The root [VERSION](VERSION) file is the source of truth for project release
versions. It is used by:

- Linux CLI build metadata
- server `.deb` and `.rpm` package versions
- Android `versionName` and derived `versionCode`

Linux binaries support `--version`:

```bash
./bin/s3s5-server --version
./bin/s3s5-client --version
./bin/s3s5-doctor --version
```

Release tags should use the same value with a `v` prefix, for example
`v0.1.0`.

## Server packages

The repository can build server-only `.deb` and `.rpm` packages. The packages
install:

- `/usr/bin/s3s5-server`
- `/usr/lib/s3s5/s3s5-server-start`
- `s3s5-server.service`
- `/etc/s3s5/s3s5-server.env`

The service is intentionally disabled after installation. Package scripts create
the `s3s5` system user/group and run `systemctl daemon-reload`, but they do not
run `systemctl enable` or `systemctl start`.

Build both package formats with Docker:

```bash
make server-package
```

Build ARM packages:

```bash
make server-package-arm64
```

Build all supported server package architectures:

```bash
make server-package-all-arch
```

Build only one format:

```bash
make server-package-deb
make server-package-rpm
make server-package-deb-all-arch
make server-package-rpm-all-arch
```

Rebuild the packaging image explicitly:

```bash
make server-package-image
```

Artifacts are written to:

```bash
dist/packages/s3s5-server_0.1.0-1_amd64.deb
dist/packages/s3s5-server-0.1.0-1.x86_64.rpm
dist/packages/s3s5-server_0.1.0-1_arm64.deb
dist/packages/s3s5-server-0.1.0-1.aarch64.rpm
```

Override version, release, or architecture:

```bash
VERSION=0.2.0 RELEASE=1 GOARCH=amd64 make server-package
```

After installing a package, edit the environment file:

```bash
sudo editor /etc/s3s5/s3s5-server.env
```

The env template includes S3/S3-compatible provider settings, AWS/Yandex
credential variables, PSK settings, polling/window tuning, and server egress
policy variables such as `S3S5_ALLOW_TARGETS`,
`S3S5_ALLOW_UNRESTRICTED_EGRESS`, `S3S5_MAX_SESSIONS`, and
`S3S5_CONNECT_TIMEOUT`.

Enable and start the service explicitly when configuration is ready:

```bash
sudo systemctl enable --now s3s5-server
```

## Android client

The Android client is a phase-1 Kotlin app compatible with the existing Go `s3s5-server`.

It runs a visible foreground service, listens on `127.0.0.1:1080` by default, and tunnels SOCKS5 `CONNECT` sessions through the same S3 object protocol. It does not implement `VpnService`; apps or tools must explicitly use the local SOCKS5 proxy.

Android app features:

- local SOCKS5 no-auth `CONNECT` listener
- foreground service with persistent notification and Stop action
- config screen for provider, bucket, prefix, region, endpoint, listen address, and credentials
- Doctor button for S3 put/get/head/list/delete roundtrip
- AWS, Yandex Object Storage, MinIO, and custom S3-compatible provider presets
- PSK-derived AES-256-GCM payload encryption
- Android Keystore-backed storage for S3 secrets and PSK
- status, counters, and in-app logs

### Android build without local SDK

You can build the APK with Docker, without installing Gradle or the Android SDK on the host.

First build the Docker image:

```bash
make android-docker-image
```

Build the debug APK:

```bash
make android-docker-build
```

Run Android JVM tests:

```bash
make android-docker-test
```

The debug APK is written to:

```bash
android-client/app/build/outputs/apk/debug/app-debug.apk
```

GitHub releases publish the same debug APK as:

```text
s3s5_<version>_android_debug.apk
```

The Docker image defaults to `s3s5-android-build:36`. Override it when needed:

```bash
S3S5_ANDROID_DOCKER_IMAGE=my-android-build:local make android-docker-build
```

The first Docker build downloads the Android SDK base image and Gradle dependencies. Gradle caches are stored under `.cache/gradle`; Android/adb state is stored under `.cache/android-sdk`.

### Android build with local tooling

If Gradle and Android SDK are installed locally, use:

```bash
make android-build
make android-test
```

or directly:

```bash
cd android-client
./gradlew :app:assembleDebug
./gradlew :app:testDebugUnitTest
```

In this repo, `android-client/gradlew` is a shim that calls a system `gradle`; the Docker image provides that executable.

### Debug vs release APK

`make android-docker-build` builds the debug APK with `:app:assembleDebug`.

Debug builds:

- are signed with Android's debug key
- are suitable for emulator/device testing
- allow cleartext `http://` endpoints for local MinIO development

Release builds should be produced separately with release signing configured:

```bash
./android-client/scripts/docker-gradle.sh :app:assembleRelease
```

Release builds keep cleartext traffic disabled by default and should not be used with arbitrary `http://` S3 endpoints.

## Contributing and security

See [CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request.

Do not report vulnerabilities in public issues. See [SECURITY.md](SECURITY.md)
for the current reporting policy.

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE).

## IPv6

IPv6 is in scope for the protocol and policy code:

- SOCKS5 parsing accepts IPv6 targets
- protocol targets preserve IPv6 addresses and domains
- policy checks understand IPv6 CIDRs
- unsafe IPv6 ranges are denied by default

Automated e2e tests do not need public IPv6 connectivity. The code should still behave correctly when the local network and the object-store endpoint support IPv6.

## Reference survey

The reference repos under `~/s3socks/` were surveyed before implementation. The survey notes are in [docs/reference-survey.md](docs/reference-survey.md).

The short version: all three reference repos appear MIT licensed, and no code was copied into this repo.
