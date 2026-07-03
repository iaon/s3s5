# Future VPNService Work

Phase 1 intentionally avoids `VpnService`. It only exposes a local SOCKS5
listener and requires each client app/tool to opt into that proxy.

Future `VpnService` work should be a separate design and threat-model change:

- explicit user consent through Android's VPN permission flow
- visible persistent notification
- clear per-app or route policy
- no boot autostart unless deliberately requested and documented
- no hidden persistence or accessibility-service coupling
- UDP behavior designed separately from SOCKS5 CONNECT
- careful battery, metering, and captive-portal handling
