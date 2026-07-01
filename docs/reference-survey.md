# Reference Survey Notes

This repository was developed against three local reference repos:

- `~/s3socks/File-Tunnel`
- `~/s3socks/hev-socks5-server`
- `~/s3socks/socks5`

## License notes

All three reference repos appear MIT licensed.

- `File-Tunnel` contains an explicit MIT license
- `hev-socks5-server` contains an MIT license
- `socks5` contains an MIT license

## What was learned

### File-Tunnel

- useful as a tunnel-organization reference
- its S3-related mode is a mount-style preset, not a direct S3 API design
- it is not the protocol model for `s3s5`

### hev-socks5-server

- useful for SOCKS5 server behavior and reply handling
- useful for keeping the command surface small and predictable

### socks5

- useful for SOCKS5 protocol handling patterns
- useful for command and reply semantics

## Reuse decision

No code was copied from the reference repos.

The repo survey informed the shape of the implementation and documentation, but the current code in this repository was written fresh.
