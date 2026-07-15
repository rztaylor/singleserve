# Changelog

All notable changes to Singleserve will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-07-15

### Security

- Replaced browser-readable launch-token persistence with an expiring, atomically consumed fragment bootstrap that establishes an independent `HttpOnly`, `Secure`, `SameSite=Strict`, non-persistent `__Host-` session cookie before consumer content runs.
- Added a 128-bit high-entropy `.localhost` launch host with exact Host validation, a high-entropy cookie name, duplicate-cookie rejection, and `__Host-` enforcement against related-domain cookie injection.
- Split bootstrap, browser-session, and programmatic authentication into independent 256-bit credentials and removed query and bearer authentication.
- Added authentication-mode-specific exact Origin enforcement, ambiguous header/cookie rejection, credential stripping before application delegation, no-store/nosniff control responses, control-body rejection, and HTTP header/idle bounds.
- Removed shell-mediated WSL browser opening and raised the minimum toolchain to Go 1.26.5 after vulnerability scanning identified a standard-library issue in Go 1.26.4.
- Stopped the minimal example from logging the capability-adjacent random launch hostname during normal startup; only the explicit manual-launch error path prints the complete secret URL.
- Added race/adversarial coverage, a non-skippable platform-neutral Playwright Chromium release contract with optional local and extra-engine diagnostic paths, pinned `govulncheck` v1.6.0, CodeQL, immutable GitHub Action SHAs, weekly Dependabot review, and release-time security/secret gates.

### Added

- Added `Launch.Client()`, an origin-bound, no-proxy programmatic HTTP client whose independent credential is not exposed to callers.
- Added `Launch.NewBootstrapURL()` for owner-controlled rotation of the single active bootstrap capability with a fresh two-minute window and no change to server lifetime or established browser sessions.
- Added secure same-tab handling for a renewed fragment on an existing failed bootstrap page, preserving immediate scrubbing before exchange.
- Added the fixed embedded `client/bootstrap.js` resource and the normative v0.2 API specification.

### Changed

- `Launch.URL()` now returns `http://ss-<random>.localhost:<port>/_singleserve/bootstrap#<capability>`; `Launch.BaseURL()` uses the same isolated clean origin.
- Supported configured bind hosts are now exactly `127.0.0.1`, `::1`, and `localhost`.
- The canonical browser client relies exclusively on the browser-managed session cookie while preserving fetch, health, heartbeat, reconnect, disconnect, shutdown, and unavailable-server behavior.

### Removed

- Removed `session.token`, `TOKEN_STORAGE_KEY`, `TOKEN_QUERY`, the browser `TOKEN_HEADER` export, Web Storage fallback, browser token headers, direct bearer authentication, and query-token bootstrap.

## [0.1.0] - 2026-07-14

### Changed

- Made first-release consumer validation self-contained through two representative repository-owned compatibility harnesses and removed application-specific provenance from project documentation.

### Added

- Repository foundation, product and architecture facts, public documentation, validation scripts, CI proposal, release governance, roadmap, and the v0.1 implementation ExecPlan.
- v0.1 API and browser-client specification with explicit acceptance criteria and migration guidance.
- MIT licence decision for the planned public library.
- Standard-library Go runtime with strict loopback binding, per-launch authentication, protected handler composition, health and lifecycle control endpoints, platform browser opening, and graceful shutdown.
- Authentication credential consumption before application delegation so token queries, headers, matching bearer credentials, launch cookies, and `RequestURI` data do not leak into consumer middleware.
- Per-tab heartbeat, disconnect, expiry, bounded tracking, explicit and browser-bound lifetime policies, and typed application shutdown guards.
- Dependency-free browser ES module with token scrubbing, same-origin authenticated fetch, health checks, heartbeat failure reporting, disconnect tracking, and guarded shutdown requests.
- Deterministic unit and loopback integration tests, race validation, browser-client tests using Node's built-in runner, and an enforced 80% Go statement-coverage gate.
- Runnable `examples/minimal` executable specification with a live lifecycle dashboard, one application API, browser launch, observable heartbeat/health timing, backend and tab-close detection windows, terminal stopped state, shutdown, and CI smoke coverage using only the public API.
- Browser-client `onHeartbeat` observation callback with status, consecutive failure count, tab ID, and completion timestamp for lifecycle presentation without duplicating heartbeat scheduling.

[Unreleased]: https://github.com/rztaylor/singleserve/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/rztaylor/singleserve/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/rztaylor/singleserve/releases/tag/v0.1.0
