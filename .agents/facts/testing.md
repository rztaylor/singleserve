# Testing Facts

- Default validation: `scripts/check.sh`.
- Documentation foundation validation: `scripts/check-docs.sh`.
- Release policy validation: `scripts/release-check.sh <version>`; it never tags or publishes.
- CI: `.github/workflows/ci.yml` has read-only permissions, broad checks, one canonical Playwright Chromium real-browser contract, and Go tests on Ubuntu, macOS, and Windows; `.github/workflows/security.yml` adds pinned vulnerability and CodeQL analysis. The browser runner OS and hosted availability are not release or local-development security properties.
- Ordinary validation requires Go 1.26.5, POSIX shell, Git, and Node.js 24 or newer. It must not require network, npm install, Docker, databases, a browser, credentials, or external services.
- `scripts/check.sh` includes docs, formatting, the minimal-example build and smoke test, Go unit/integration tests, race tests, 80% coverage, vet, Node built-in client tests, and diff checks.
- Maintain at least 80% overall Go statement coverage, with focused coverage for auth, tabs, lifetime, and shutdown.
- Use deterministic fake time for policy transitions and real race-enabled synchronization tests for concurrency.
- v0.1.0 recorded its browser prerequisite skip. The v0.2 Go harness is `browser_security_test.go`, invoked by `scripts/check-browser-security.sh`; repository-owned CI drives the same assertions through pinned Playwright Chromium. One passing Playwright Chromium run at the exact candidate commit is required; CI is preferred and a recorded local run is an accepted fallback. Additional engine/platform runs are optional diagnostics, and missing local browser infrastructure does not block development.
- For v0.2.x, browser authentication, URL scrubbing, cookie reload, multi-tab behavior, and sibling-loopback isolation are mandatory real-browser release gates. If the harness or supported browser is unavailable, the candidate is not releasable.
- Security validation must cover exact Host and Origin enforcement, authentication-mode distinctions, credential non-storage/non-echo, capability replay and owner-controlled renewal, Secure `__Host-` cookie scope across ports/hosts/related domains, malformed inputs, and control-endpoint bounds.
- v0.2.x release evidence includes a vulnerability scan, repository static checks, and reviewed immutable CI action references. CodeQL remains defense-in-depth: an actionable finding blocks release, while unavailable hosted analysis does not if the required local checks and browser evidence pass.
- The complete v0.1 acceptance matrix is in `docs/dev/specs/v0.1-api.md`.
