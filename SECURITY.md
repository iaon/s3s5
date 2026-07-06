# Security Policy

`s3s5` is an MVP and should be treated as experimental software.

## Supported Versions

No stable release line exists yet. Security fixes apply to the current `main`
branch until versioned releases are introduced.

## Reporting a Vulnerability

Do not open a public GitHub issue for vulnerabilities that could expose secrets,
disable crypto, bypass egress policy, or enable unauthorized traffic forwarding.

Until a private security advisory channel is configured, contact the maintainers
privately through the repository owner. Include:

- affected commit or release
- impact and attack scenario
- reproduction steps
- whether credentials, payloads, or private infrastructure were involved

## Security Expectations

- Payload encryption is enabled by default.
- `--insecure-no-crypto` is for local development only.
- Server egress must be explicitly allowed with `--allow-target` rules, or with
  `--allow-unrestricted-egress` only in controlled environments.
- Android release builds must not use arbitrary cleartext endpoints.
- Secrets must come from environment variables, Android Keystore-backed storage,
  or untracked local configuration.
