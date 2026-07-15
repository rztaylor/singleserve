# Development guide

## Current state

`v0.1.0` is the current supported module release. Security-only v0.2.0 is implemented and in release preparation. Two repository-owned public-API compatibility harnesses cover browser-bound and explicit-lifetime applications.

## Prerequisites

- Go 1.26.5 or newer within the declared 1.26 release line;
- POSIX shell for repository scripts;
- Git for diff checks; and
- Node.js 24 or newer for dependency-free browser-client tests; and
- optional Playwright test dependencies or a supported WebDriver for real-browser validation.

Ordinary checks require no network, npm install, Docker, database, browser, or external service.

## Commands

```sh
scripts/check-docs.sh
scripts/check.sh
SINGLESERVE_BROWSER_BACKEND=playwright SINGLESERVE_BROWSER_NAME=chromium scripts/release-check.sh v0.2.0
go run ./examples/minimal
```

`check.sh` is the default local and CI gate. It builds and smoke-tests the minimal example, then verifies documentation, formatting, Go unit/integration and race tests, at least 80% Go statement coverage, vet, browser-client tests, and diff whitespace. `release-check.sh` validates version shape, governance files, the broad check, and a clean module package list; it never tags or publishes.

The real-browser contract is intentionally separate from ordinary validation. `scripts/check.sh` is the complete required local-development gate on macOS, Linux, and Windows and needs no browser, npm install, or network. Repository-owned CI installs pinned Playwright Chromium and is the preferred candidate evidence; the runner OS is an implementation detail rather than a contributor or release-platform requirement. A recorded exact-commit local Playwright Chromium run is an accepted fallback. WebDriver and extra engine/platform runs are optional diagnostics for concrete compatibility investigations.

In managed sandboxes use the scripts, which set writable repository-local Go caches. Generated cache content lives under `.cache/` and is ignored.

## Coding conventions

- Keep the exported API small and documented. Examples that promise usage must compile.
- Use `http.Handler` rather than choosing a router.
- Use typed errors only when callers need stable control flow.
- Pass contexts through listener, drain, guard, and subprocess boundaries.
- Inject time and process seams narrowly; do not build general abstraction frameworks.
- Keep mutable lifecycle state private and synchronized. Run the race detector for any change to auth, tabs, policies, or shutdown.
- Prefer table-driven tests, `httptest`, real loopback listeners for integration, and deterministic fake clocks for time transitions.
- Never invoke browser URLs through a shell.
- Never include real launch tokens in docs, logs, test failures, or fixtures.
- Never store browser authentication capabilities in Web Storage or expose them through public JavaScript state.
- Do not treat a random loopback port as cookie isolation. Validate the effective Host and prove isolation from sibling loopback services.
- Browser security behavior requires one exact-candidate repository-owned Playwright Chromium result for v0.2.0; CI is preferred and a recorded local run is accepted, while missing local browser automation does not block development.

## Documentation workflow

Behavior changes update the applicable normative versioned spec first or in the same change. Ownership changes update architecture. Public compatibility and support changes update the changelog, decisions, and release governance. New planned work belongs in the active roadmap rather than a release history. No implementation plan is active during v0.2.0 release preparation; unrelated features remain deferred.
