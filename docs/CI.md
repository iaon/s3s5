# Continuous Integration

GitHub Actions are defined under `.github/workflows/`.

## Basic CI

`.github/workflows/ci.yml` runs on pull requests and pushes to `main`.

Pull requests run:

- `make build`
- `make test`
- `make lint`

Pushes to `main` run the same Go checks and also build server package
artifacts with:

```sh
make server-package
```

The package artifacts are uploaded as workflow artifacts for inspection.

## Android CI

`.github/workflows/android.yml` runs:

- manually through `workflow_dispatch`
- on pull requests that touch `android-client/**` or the Android workflow

It uses the Docker-based Android toolchain:

```sh
make android-docker-build
make android-docker-test
```

The debug APK is uploaded as a workflow artifact. It is not a release APK and
is signed with Android's debug key.

## Release CI

`.github/workflows/release.yml` runs on tags matching `v*` and can also be
started manually with a tag input.

It verifies that the pushed tag matches the root `VERSION` file. For example,
if `VERSION` contains `0.1.0`, the release tag must be `v0.1.0`.

Release CI builds:

- Go checks: build, test, vet
- Linux binary tarballs for `linux/amd64` and `linux/arm64`
- server `.deb` and `.rpm` packages for `amd64` and `arm64`
- Android debug APK and Android JVM tests
- `SHA256SUMS`

It then publishes those files to a GitHub Release.

The GitHub Release includes `s3s5_<version>_android_debug.apk`. It is signed
with Android's debug key and is intended for test installs until release signing
is configured.
