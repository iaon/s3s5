# Contributing

Thanks for considering a contribution to `s3s5`.

This is a security-sensitive networking project. Changes should be small,
explicit, and easy to review.

## Development

Run the normal Go checks:

```sh
make build
make test
```

In restricted sandboxes, `make test` may need permission to bind loopback
sockets because SOCKS5 and tunnel tests start local listeners.

Android checks can run through Docker:

```sh
make android-docker-build
make android-docker-test
```

Server package builds can also run through Docker:

```sh
make server-package
```

## Pull Requests

Please include:

- a short description of behavior changes
- validation commands run
- security or compatibility impact
- notes about provider-specific behavior when touching S3-compatible storage

## Security-sensitive Changes

Do not weaken destination policy, crypto defaults, TLS validation, foreground
service visibility, or credential handling without calling it out explicitly in
the pull request.

Do not commit real credentials, tokens, buckets containing private data, or
payload captures.
