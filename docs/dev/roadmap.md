# Roadmap

`v0.1.0` is the current supported release. Security-only v0.2.0 is implemented but remains unreleased until every browser and security release gate passes.

## Execution sequence

1. `security-hardening` — establish and implement the v0.2.0 browser-authentication, loopback-isolation, request-validation, opener, resource-hardening, and release-evidence contract.

## Active items

### security-hardening

- **Status:** Partial
- **Release:** unreleased v0.2.0
- **Why now:** v0.1.0 persists its launch capability in script-readable `sessionStorage`, exposes it from the browser-client session, relies on application JavaScript to scrub the launch URL, and uses a host cookie on a random port even though cookies do not isolate ports.
- **Dependencies:** review the repository-owned Playwright Chromium result plus CodeQL at the candidate commit, validate consumer migration, and pass the clean-candidate release check. The runner OS and optional diagnostic engines are not standing requirements. None of the listed candidate gates is optional.
- **Brief:** [Security hardening](roadmap-items/security-hardening.md)
- **ExecPlan:** [v0.2.0 security hardening](plans/000-v0.2.0-security-hardening.md)

## Deferred direction

Do not schedule npm packaging, remote access, control-path customization, event observers, TLS termination, WebSockets, or unrelated lifecycle features during v0.2.0. Durable rationale lives in `docs/dev/decisions.md`.
