# Development guide

## Current state

`v0.1.0` is the first supported module release. Two repository-owned public-API compatibility harnesses cover browser-bound and explicit-lifetime applications. A real-browser smoke run remains conditional on a supported browser and stable harness; the first-release source records its local skip and associated risk in the release governance.

## Prerequisites

- Go 1.26 or newer within the declared 1.26 release line;
- POSIX shell for repository scripts;
- Git for diff checks; and
- Node.js 24 or newer for dependency-free browser-client tests; and
- an optional supported browser when real-browser smoke validation is available.

Ordinary checks require no network, npm install, Docker, database, browser, or external service.

## Commands

```sh
scripts/check-docs.sh
scripts/check.sh
scripts/release-check.sh v0.1.0
go run ./examples/minimal
```

`check.sh` is the default local and CI gate. It builds and smoke-tests the minimal example, then verifies documentation, formatting, Go unit/integration and race tests, at least 80% Go statement coverage, vet, browser-client tests, and diff whitespace. `release-check.sh` validates version shape, governance files, the broad check, and a clean module package list; it never tags or publishes.

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

## Documentation workflow

Behavior changes update the normative v0.1 spec first or in the same change. Ownership changes update architecture. Public compatibility and support changes update the changelog, decisions, and release governance. New planned work belongs in the active roadmap rather than a release history.
