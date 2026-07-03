# s3s5 Android Client

This is a phase-1 Android client for the existing Go `s3s5-server`.

The app runs a visible foreground service and exposes a local SOCKS5 listener on
`127.0.0.1:1080` by default. It does not use `VpnService`, does not proxy device
traffic globally, and does not start on boot.

## Build

Open `android-client/` in Android Studio, or install Gradle plus Android SDK
command-line tools and run:

```sh
./gradlew :app:assembleDebug
./gradlew :app:testDebugUnitTest
```

This repository environment currently has Java but no Gradle executable and no
Android SDK, so Android build verification must happen on a machine with those
tools installed.

## Docker Build

To build without installing Gradle or Android SDK locally, use Docker:

```sh
make android-docker-build
make android-docker-test
```

The first run builds `s3s5-android-build:35` from
`android-client/Dockerfile`, pulls the Android SDK base image, and downloads
Gradle dependencies into `.cache/gradle`. Override the image name with:

```sh
S3S5_ANDROID_DOCKER_IMAGE=my-android-build:local make android-docker-build
```

## Manual Smoke

1. Start an `s3s5-server` using the same bucket, prefix, provider, and PSK.
2. Install the debug APK on an emulator or device.
3. Configure the app. Keep listen host as `127.0.0.1` unless LAN access is
   intentionally needed.
4. Tap `Doctor` to verify S3 credentials and object roundtrip.
5. Tap `Start`.
6. For an emulator, forward the host port:

```sh
adb forward tcp:1080 tcp:1080
curl --socks5-hostname 127.0.0.1:1080 https://example.com/
```

## Provider Presets

- `aws`: defaults region to `us-east-1` when empty and uses virtual-hosted AWS
  URLs unless an endpoint/path-style setting requires otherwise.
- `yandex`: endpoint `https://storage.yandexcloud.net`, region `ru-central1`,
  path-style URLs.
- `minio`: endpoint `http://127.0.0.1:9000`, region `us-east-1`, path-style URLs.
- `custom`: explicit endpoint/region/path-style settings.
