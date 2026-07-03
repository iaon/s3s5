# Android Security Notes

- The default SOCKS listener is `127.0.0.1:1080`.
- Binding to any non-localhost address requires explicit `Allow LAN listen`.
- The app uses a visible foreground service notification while the listener is
  running.
- S3 credentials and PSK are encrypted at rest with an Android Keystore AES-GCM
  key.
- Non-secret configuration is stored in SharedPreferences.
- The service and Doctor do not log credentials, PSK values, or payload bytes.
- Release builds reject cleartext `http://` endpoints in config validation.
- Debug builds allow cleartext endpoints for MinIO development.
- The app does not install a CA, bypass TLS validation, request root, use
  accessibility services, start on boot, hide notifications, or implement
  persistence behavior.
