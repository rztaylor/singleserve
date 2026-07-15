# Testing Facts

- Default validation: `scripts/check.sh`.
- Documentation foundation validation: `scripts/check-docs.sh`.
- Release policy validation: `scripts/release-check.sh <version>`; it never tags or publishes.
- CI: `.github/workflows/ci.yml` has read-only permissions, broad checks, one canonical Playwright Chromium real-browser contract, and Go tests on Ubuntu, macOS, and Windows; `.github/workflows/security.yml` adds pinned vulnerability and CodeQL analysis. The browser runner OS is an implementation detail, not a release or local-development requirement.
- Ordinary validation requires Go 1.26.5, POSIX shell, Git, and Node.js 24 or newer. It must not require network, npm install, Docker, databases, a browser, credentials, or external services.
- `scripts/check.sh` includes docs, formatting, the minimal-example build and smoke test, Go unit/integration tests, race tests, 80% coverage, vet, Node built-in client tests, and diff checks.
- Maintain at least 80% overall Go statement coverage, with focused coverage for auth, tabs, lifetime, and shutdown.
- Use deterministic fake time for policy transitions and real race-enabled synchronization tests for concurrency.
- v0.1.0 recorded its browser prerequisite skip. The v0.2 Go harness is `browser_security_test.go`, invoked by `scripts/check-browser-security.sh`; repository-owned CI drives the same assertions through pinned Playwright Chromium. Local Playwright/WebDriver and additional engine/platform runs are optional diagnostics. Missing candidate CI evidence blocks release; missing local browser infrastructure does not block development.
- For v0.2.0, browser authentication, URL scrubbing, cookie reload, multi-tab behavior, and sibling-loopback isolation are mandatory real-browser release gates. If the harness or supported browser is unavailable, v0.2.0 is not releasable.
- Security validation must cover exact Host and Origin enforcement, authentication-mode distinctions, credential non-storage/non-echo, capability replay and owner-controlled renewal, Secure `__Host-` cookie scope across ports/hosts/related domains, malformed inputs, and control-endpoint bounds.
- The v0.2.0 release evidence must include a vulnerability scan, static security analysis, and reviewed immutable CI action references or a documented equivalent.
- The complete v0.1 acceptance matrix is in `docs/dev/specs/v0.1-api.md`.
