# Android Testing

Unit tests are JVM-focused where possible:

```sh
./gradlew :app:testDebugUnitTest
```

Expected coverage:

- protocol key layout and 20-digit sequence formatting
- session ID shape
- HKDF-SHA256 known vector
- AES-GCM encrypt/decrypt, wrong AAD failure
- SOCKS5 no-auth negotiation and request parsing
- memory object-store CRUD/list behavior
- provider default handling

Build:

```sh
./gradlew :app:assembleDebug
```

Docker build without local Android tooling:

```sh
make android-docker-build
make android-docker-test
```

These commands build and use the local Docker image
`s3s5-android-build:35`, mounting `.cache/gradle` and `.cache/android-sdk` for
repeatable dependency caches.

Manual emulator smoke:

```sh
./scripts/install-debug.sh
./scripts/adb-forward.sh
curl --socks5-hostname 127.0.0.1:1080 https://example.com/
```

Real S3/Yandex credentials should be entered manually in the app. Do not commit
credentials or generated config files.
