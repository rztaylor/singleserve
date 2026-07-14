# Security Policy

Singleserve has not released a supported version yet. Security-sensitive reports about the specification or future implementation should be sent privately through GitHub Security Advisories once the public repository exists. Until then, contact the maintainer through a private channel already known to you; do not include live tokens, private application data, or exploit details in a public issue.

The v0.1 security boundary is intentionally narrow:

- loopback listeners only;
- fresh 256-bit launch tokens;
- constant-time token comparison;
- no permissive CORS;
- same-origin enforcement for browser-originated unsafe requests;
- launch-unique, session-only authentication cookies;
- no remote access, TLS, accounts, telemetry, or persistent toolkit state.

Security fixes may change pre-1.0 APIs when necessary. They must be called out under `Security` in `CHANGELOG.md`.
