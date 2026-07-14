# Changelog

All notable changes to Singleserve will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/rztaylor/singleserve/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/rztaylor/singleserve/releases/tag/v0.1.0
