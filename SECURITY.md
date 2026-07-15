# Security Policy

## Supported versions

| Version | Status |
| --- | --- |
| v0.2.x | Supported |
| v0.1.x | Unsupported; see the browser-client limitations below |
| Earlier | Unsupported |

`v0.2.0` is the first release of the hardened browser-authentication contract. Published tags are immutable. If a supported release is unsafe, the project will document the impact and publish a new corrective version rather than moving or silently replacing a tag.

## Reporting a vulnerability

Report suspected vulnerabilities privately through GitHub Security Advisories for `github.com/rztaylor/singleserve`. Do not open a public issue and do not include live launch URLs, tokens, application data, browser profiles, or other secrets in chat, public logs, or test fixtures.

Include the affected version or commit, operating system and browser when relevant, reproduction conditions, expected impact, and whether the report involves an untrusted web page, sibling loopback service, local process, or consumer application code. Use synthetic credentials only.

## Security boundary

Singleserve is loopback-only and owns per-launch authentication, browser bootstrap, control endpoints, tab presence, shutdown guards, and graceful drain around a consumer-owned `http.Handler`.

The supported boundary trusts the Singleserve process, the selected browser, and consumer code intentionally supplied as the handler. It does not trust remote web pages, sibling loopback services, or unrelated local processes merely because they run on the same machine. Network-only local processes do not receive a credential. OS administrator access, process-memory inspection, permission to inspect the short-lived browser-launch command line, a compromised browser or extension, and arbitrary code execution inside trusted consumer code are outside the preventable boundary.

Security is a release requirement. Known credential exposure, origin/Host confusion, cookie-isolation failure, non-loopback access, shutdown-integrity failure, or missing required security evidence blocks a release. Before 1.0, security fixes may make documented breaking changes and must be called out under `Security` in `CHANGELOG.md` with migration guidance.

## v0.1.0 browser-client limitation

The v0.1.0 canonical browser client stores the reusable launch capability in per-tab `sessionStorage` and returns it as `session.token`. Although shorter-lived than `localStorage`, this makes the credential readable to JavaScript executing in the origin. The initial capability also remains in the browser URL until application JavaScript calls `connect()`.

Additionally, v0.1.0 uses a host-scoped session cookie on a random loopback port. Cookie names are launch-unique, but HTTP cookies do not provide port isolation, so another service on the same loopback host can receive the cookie if the browser visits it. Consumers requiring a no-Web-Storage or mutually distrusting sibling-loopback contract should not treat v0.1.0 as satisfying that requirement.

## v0.2 hardening

The released v0.2 contract removes browser-readable credential persistence and public token exposure. It introduces a one-time fragment bootstrap owned by Singleserve, owner-controlled rotation through `Launch.NewBootstrapURL`, independent browser-session and programmatic credentials, a high-entropy per-launch `.localhost` origin, a Secure host-only `__Host-` session cookie, exact Host and authentication-mode-specific Origin validation, a no-proxy origin-bound `Launch.Client`, non-shell opener commands, bounded control requests, and mandatory browser/vulnerability/static-analysis release gates. Renewal replaces the single active unconsumed capability for two minutes; it never makes the initial URL indefinite or extends server lifetime. No legacy Web Storage mode exists.

The session cookie is `HttpOnly`, which prevents direct JavaScript reads but cannot prevent trusted same-origin consumer JavaScript from issuing authenticated requests. Consumer code intentionally supplied through `Options.Handler`, compromised browsers/extensions, process-memory readers, and actors who already possess the complete launch URL are outside the preventable boundary. The high-entropy hostname isolates the cookie from ordinary sibling loopback origins, while the `__Host-` rules reject parent-domain injection; the hostname is capability-adjacent and must be protected with the bootstrap URL.
