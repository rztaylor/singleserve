# security-hardening — v0.2.0 security hardening

Status: Partial. Runtime, deterministic tests, a shared Playwright/WebDriver browser harness, platform-neutral Chromium security automation, and migration documentation are implemented; candidate browser/hosted-analysis evidence and release validation remain.

## Goal

Make v0.2.0 a security-only release that removes script-readable browser credential persistence and closes the related bootstrap, origin-isolation, request-validation, opener, resource-exhaustion, and release-evidence gaps without weakening Singleserve's loopback-only product boundary.

## Scope

- Replace browser Web Storage token retention and public token exposure with a server-owned bootstrap and HttpOnly session flow.
- Ensure application content never runs at a capability-bearing URL and browser reloads authenticate without JavaScript-readable credentials.
- Isolate each launch's session cookie from sibling loopback services; a random port alone is not acceptable isolation.
- Validate the effective request host, browser origin, and authentication mode before application delegation.
- Review capability lifetime and replay, browser-opener argument exposure, shell-mediated opener fallbacks, control-endpoint input bounds, HTTP resource limits, token non-echo, and cleanup behavior.
- Add non-skippable real-browser security validation plus vulnerability, static-analysis, and CI supply-chain gates for v0.2.0.
- Publish a breaking-change migration from the v0.1.0 browser client without retaining an insecure compatibility mode.

## Acceptance

- No Singleserve browser credential is stored in or recovered from script-readable browser persistence.
- The browser-client public result exposes no authentication capability.
- Capability material is consumed and scrubbed by Singleserve before consumer HTML or JavaScript runs and cannot leak through consumer middleware, referrers, caches, or error output. Any unavoidable opener or browser-history residue is non-replayable under the specified bootstrap contract.
- Trusted owner code can replace an unusable manual bootstrap URL without lengthening the capability lifetime: renewal invalidates the earlier unconsumed URL, starts a fresh two-minute window, and leaves server lifetime and existing sessions unchanged.
- Reload, new-tab, heartbeat, fetch, disconnect, shutdown, unavailable-server, and browser-bound lifetime behavior continue through an isolated HttpOnly cookie.
- A sibling loopback service cannot receive, set, overwrite, or replay another launch's cookie under the supported browser origin design.
- Host, Origin, authentication-mode, malformed-input, capability-replay, and reserved-endpoint bounds fail closed under unit, integration, race, fuzz-seed, and real-browser tests.
- Browser-launch implementations do not pass capability material through shell interpretation and minimize useful exposure in process arguments.
- Required security tooling and browser evidence pass; missing evidence blocks v0.2.0.
- The v0.2 specification, architecture, decisions, migration guide, README, security policy, changelog, facts, release governance, examples, and compatibility harnesses agree with implementation.

## Important details

- The requirement was demonstrated while evaluating SQLRise as a consumer. SQLRise forbids credentials in Web Storage, and the constraint is generic to browser-based local applications rather than application-specific behavior.
- RFC 6265 states that cookies do not provide isolation by port. v0.1.0's launch-unique cookie name prevents collisions but does not prevent a sibling service on the same host from receiving the cookie header.
- `HttpOnly` prevents JavaScript from reading a cookie, but it does not stop malicious same-origin JavaScript from issuing authenticated requests. The v0.2 threat model and documentation must state that distinction precisely.
- The implementation sequence and unresolved design gates live in `docs/dev/plans/000-v0.2.0-security-hardening.md`.
- If the canonical browser contract and deterministic platform tests cannot demonstrate a secure launch-origin and bootstrap design, v0.2.0 remains blocked. This does not require a browser/OS cross-product; the fallback is not to restore the v0.1.0 storage behavior.
