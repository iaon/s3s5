# Release Process

`VERSION` is the source of truth for release versions.

Release tags should use the same value with a `v` prefix. For example, when
`VERSION` contains `0.1.0`, tag the release as `v0.1.0`.

## Prepare

1. Update `VERSION`.
2. Move changelog entries from `Unreleased` to the release version.
3. Run validation:

```sh
make build
make test
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

```sh
git tag -a "v$(cat VERSION)" -m "Release v$(cat VERSION)"
git push origin "v$(cat VERSION)"
```

## Artifacts

Server packages are written to `dist/packages/`.

The Android debug APK is written to:

```text
android-client/app/build/outputs/apk/debug/app-debug.apk
```

Release Android APKs require release signing configuration before publication.
