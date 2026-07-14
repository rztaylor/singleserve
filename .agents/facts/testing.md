# Testing Facts

- Default validation: `scripts/check.sh`.
- Documentation foundation validation: `scripts/check-docs.sh`.
- Release policy validation: `scripts/release-check.sh <version>`; it never tags or publishes.
- CI proposal: `.github/workflows/ci.yml` with read-only permissions, Ubuntu broad checks, and Go tests on Ubuntu, macOS, and Windows.
- Ordinary validation requires Go 1.26, POSIX shell, Git, and Node.js 24 or newer. It must not require network, npm install, Docker, databases, a browser, credentials, or external services.
- `scripts/check.sh` includes docs, formatting, the minimal-example build and smoke test, Go unit/integration tests, race tests, 80% coverage, vet, Node built-in client tests, and diff checks.
- Maintain at least 80% overall Go statement coverage, with focused coverage for auth, tabs, lifetime, and shutdown.
- Use deterministic fake time for policy transitions and real race-enabled synchronization tests for concurrency.
- Real-browser smoke tests are conditional until a supported browser and stable harness are available; v0.1.0 explicitly records the unavailable-browser skip and residual risk, while core unit/race/security gates cannot be skipped.
- The complete v0.1 acceptance matrix is in `docs/dev/specs/v0.1-api.md`.
