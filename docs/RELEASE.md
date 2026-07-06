# Release Process

`VERSION` is the source of truth for release versions.

Release tags should use the same value with a `v` prefix. For example, when
`VERSION` contains `0.1.0`, tag the release as `v0.1.0`.

Releases are tag-driven. The normal flow is:

1. Merge release-prep changes into `main`.
2. Create and push an annotated `vX.Y.Z` tag from that commit.
3. Let GitHub Actions build and publish the GitHub Release.

## Prepare

1. Update `VERSION`.
2. Move changelog entries from `Unreleased` to the release version.
3. Run validation locally when practical:

```sh
make build
make test
make lint
make android-docker-build
make android-docker-test
make server-package
```

4. Check CLI build metadata:

```sh
./bin/s3s5-client --version
./bin/s3s5-server --version
./bin/s3s5-doctor --version
```

## Tag

Create the tag only after the release-prep commit is on `main`:

```sh
git tag -a "v$(cat VERSION)" -m "Release v$(cat VERSION)"
git push origin "v$(cat VERSION)"
```

The release workflow rejects tags that do not match `VERSION`.

## Release Artifacts

The release workflow publishes:

- `s3s5_<version>_linux_amd64.tar.gz`
- `s3s5_<version>_linux_arm64.tar.gz`
- `s3s5_<version>_android_debug.apk`
- server `.deb` packages for `amd64` and `arm64`
- server `.rpm` packages for `x86_64` and `aarch64`
- `SHA256SUMS`

Local Linux tarballs can be built with:

```sh
make release-artifacts
```

Server packages are written to `dist/packages/`.

The Android debug APK is written to:

```text
android-client/app/build/outputs/apk/debug/app-debug.apk
```

GitHub releases publish that APK as
`s3s5_<version>_android_debug.apk`. It is signed with Android's debug key.
Release-signed Android APKs require release signing configuration.

## Manual Release Workflow

The release workflow can be started manually from GitHub Actions with a tag
input. This is mainly for retrying a failed release job after fixing workflow
infrastructure. The tag still must match `VERSION`.
