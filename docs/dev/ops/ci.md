# Continuous integration

The checked-in workflow at `.github/workflows/ci.yml` uses read-only permissions and performs no publishing.

## Pull request and main checks

- Ubuntu runs `scripts/check.sh`, including the `examples/minimal` build and public-API HTTP smoke test, docs, format, tests, coverage, vet, race, and browser-client checks.
- Node 24 runs the dependency-free browser-client tests without package installation in the ordinary validation job.
- A platform matrix runs `go test ./...` on Ubuntu, macOS, and Windows so platform opener files compile and platform-specific tests execute.
- Go is selected from `go.mod`. Dependency caching stays disabled while the standard-library-only module has no `go.sum`; enable it only if a real dependency creates one.
- No secrets or write permissions are required.
- A dedicated hosted job installs the pinned Playwright development dependency and runs the mandatory real-browser security contract on Chromium. Its current runner OS is an implementation detail and may change without altering support policy; a recorded exact-commit local Playwright Chromium run is the fallback when hosted execution is unavailable.
- A manual workflow dispatch with a version runs the complete release-candidate policy through the same hosted Chromium contract. It validates only and never tags or publishes.

## Deferred CI

- Dependency-licence reporting, coverage upload, and release automation remain deferred until they have a demonstrated release need.
- Required status-check and branch-protection configuration requires a real GitHub repository and explicit authorization.

## v0.2 security CI

The v0.2 release line promotes the following checks from optional follow-up to release requirements:

- a repository-owned real-browser harness covering bootstrap and owner-controlled renewal, clean URL, no Web Storage, cookie reload, cross-origin rejection, and sibling-loopback isolation;
- a pinned official Go vulnerability check with recorded database freshness;
- CodeQL or a documented equivalent static security analysis with minimal workflow permissions; and
- third-party GitHub Actions pinned to reviewed immutable commit SHAs, with Dependabot updates preserving reviewable version context.

The repository implements these checks in `scripts/check-browser-security.sh`, `scripts/check-security.sh`, `.github/workflows/ci.yml`, and `.github/workflows/security.yml`. The Go browser contract has no third-party Go dependency; hosted browser execution uses the exact Playwright version in `client/package-lock.json`, with weekly Dependabot review. The browser job is the preferred repository-owned candidate evidence, not a local contributor prerequisite, and no browser/OS cross-product is required without a concrete compatibility reason. `govulncheck` is pinned to v1.6.0 and reports its database timestamp. CodeQL uses only `security-events: write` in its analysis job. Every third-party action reference is an immutable reviewed commit SHA with its human-readable major version in a comment. Mandatory candidate evidence and actionable findings block release; hosted CodeQL availability by itself does not.

## Dependabot

`.github/dependabot.yml` performs weekly grouped checks for Go modules and GitHub Actions. The runtime remains standard-library-only; module checks still catch future test/tool dependencies, while action updates require review and repinning to an immutable SHA.
