# Security Facts

## Priority and threat boundary

- Security is a release requirement and overrides backward compatibility, convenience, and schedule before 1.0.
- A known authentication, credential-exposure, host/origin, cookie-isolation, loopback, shutdown-integrity, or security-validation gap blocks release.
- The supported boundary trusts the Singleserve process, the selected browser, and consumer code supplied intentionally to `Options.Handler`; it does not trust remote web pages, sibling loopback services, or unrelated local processes.
- OS administrator access, process-memory inspection, permission to inspect the short-lived browser-launch command line, a compromised browser or extension, and arbitrary code execution inside the trusted consumer are outside the preventable boundary and must be stated rather than implied away.

## Credential invariants

- Generate independent authentication material with `crypto/rand`; never reuse a bootstrap capability as a long-lived browser or programmatic credential without an explicit reviewed decision.
- Never persist authentication capabilities in `localStorage`, `sessionStorage`, IndexedDB, Cache Storage, service workers, or other script-readable browser state.
- Do not expose browser authentication capabilities through returned JavaScript objects, globals, events, logs, errors, application handlers, middleware, referrers, or caches.
- Browser reload authentication uses a launch-specific `HttpOnly`, `Secure`, `SameSite=Strict`, non-persistent `__Host-` cookie only when real browsers prove both acceptance on the `.localhost` origin and rejection of related-domain cookie injection.
- A random port is not cookie isolation. Cookie and effective-host behavior must be tested against simultaneous and mutually distrusting loopback services.
- Capability-bearing launch URLs must be server-scrubbed before application content runs, must be non-cacheable and non-referring, and must have bounded replay through one-time, expiring, or equivalently strong bootstrap semantics proven in supported browsers.
- Owner-controlled bootstrap renewal must rotate the single active unconsumed capability, retain the fixed short expiry, and remain independent of server lifetime and established browser-session state. Singleserve does not automatically renew or prescribe a consumer UI.

## Request and release invariants

- Validate loopback binding, the effective request host, browser origin, and authentication mode before delegating to the consumer handler.
- Cookie-authenticated unsafe browser requests require exact launch-origin evidence. Explicit programmatic credentials may omit browser headers but must never weaken browser checks.
- Singleserve-owned control endpoints use strict methods, bounded input, non-cacheable responses, and non-sensitive errors.
- Security-sensitive behavior requires deterministic unit and integration tests plus repository-owned real-browser validation. One passing Playwright Chromium run at the exact v0.2.x candidate commit is mandatory; CI is preferred and a recorded local run is an accepted fallback. Missing local browser infrastructure does not block ordinary development.
- `scripts/check.sh` remains the complete offline local-development gate. `scripts/check-security.sh` pins `govulncheck` v1.6.0, runs vet, scans secrets, and rejects mutable action references. Repository-owned CI runs `scripts/check-browser-security.sh` through pinned Playwright Chromium; its runner OS is not a security requirement. Optional WebDriver and extra engine/platform runs are diagnostic. CodeQL runs with minimal workflow permissions as defense-in-depth; actionable findings block release, but hosted availability alone does not.
- The Go patch baseline is 1.26.5 because the v0.2 security scan found GO-2026-5856 in Go 1.26.4.
- Published tags are immutable. Correct an unsafe release with a new version and clear security guidance; never move a tag.
