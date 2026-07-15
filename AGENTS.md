# Repository Instructions

Singleserve is a public Go library for single-binary, browser-based local applications. v0.2.x is the supported release line; v0.2.0 introduced the released security contract. Describe a specific version as supported only once its matching tag exists.

## Required context

Read the relevant repo-local facts before changing code, docs, scripts, roadmap, CI, or release behavior:

- Product: `.agents/facts/product.md`
- Architecture: `.agents/facts/architecture.md`
- Engineering: `.agents/facts/engineering.md`
- Go: `.agents/facts/go.md`
- Testing: `.agents/facts/testing.md`
- Security: `.agents/facts/security.md`
- Technical debt: `.agents/facts/find-tech-debt.md`
- Documentation: `.agents/facts/docs.md`
- Roadmap and planning: `.agents/facts/roadmap.md`
- Git workflow: `.agents/facts/git.md`
- Release policy: `.agents/facts/release.md`

## Product boundaries

- Security is a release requirement, not an optional mode or compatibility tradeoff. A known credential, origin, isolation, or lifecycle vulnerability blocks release.
- Keep the server strictly loopback-only. Do not add an opt-out.
- Generate authentication material per process launch and never persist or log it accidentally.
- Keep lifetime policy independent from browser-opening policy.
- Preserve application ownership of its `http.Handler`, routes, domain logic, frontend framework, and assets.
- Keep the browser client framework-neutral and dependency-free.
- Do not import consumer application packages or let a consumer dictate package architecture.
- Add requirements only when they are demonstrated as generic to Singleserve's lifecycle boundary.
- Reject remote access, TLS termination, account identity, desktop webviews, frontend tooling, and application persistence unless the product direction is explicitly changed.

## Security invariants

- Treat remote web pages, sibling loopback services, and untrusted local processes as outside the trust boundary; do not rely on a random port as an origin, cookie, or credential-isolation boundary.
- Never write authentication capabilities to `localStorage`, `sessionStorage`, IndexedDB, caches, or other script-readable browser persistence.
- Keep browser authentication material out of public JavaScript return values and application-owned handlers, middleware, logs, errors, caches, referrers, and clean navigation history.
- Prefer server-owned, short-lived bootstrap and HttpOnly cookie flows over application-JavaScript credential handling.
- Validate the effective request host and browser origin at the Singleserve boundary. Security-sensitive requests must fail closed when their provenance is missing, malformed, or inconsistent with their authentication mode.
- Do not retain a weaker legacy security mode merely for compatibility. Before 1.0, make the breaking change, document the migration, and release a new minor version.
- Treat real-browser authentication, reload, multi-tab, cross-origin, and sibling-loopback isolation tests as mandatory repository-owned release evidence for security changes. The canonical Playwright Chromium contract may run in CI or as a recorded local fallback at the exact candidate commit; CI is preferred, but its runner OS and hosted availability are not security properties. Missing or failing browser evidence blocks release, but missing local browser automation never blocks ordinary development.

## Package boundaries

- The intentional public API is the root `singleserve` package.
- Do not create `pkg/` or extra public packages without a demonstrated consumer need and a spec update.
- Keep implementation details under `internal/` only when a real responsibility justifies extraction.
- Keep the canonical plain-ES-module browser client under `client/`; the Go package embeds and serves that exact source.
- Every Go package must have a package comment describing ownership and exclusions.

## Planning gate

The released v0.2 security contract is specified by `docs/dev/specs/v0.2-api.md`; patch preparation is governed by the changelog and release policy. Any additional material expansion must:

1. review `docs/dev/specs/v0.1-api.md` and `docs/dev/decisions.md`;
2. keep work within the accepted decisions or amend them explicitly;
3. restate goal, acceptance criteria, scope, dependencies, risks, and deferrals; and
4. obtain explicit scope approval.

Do not add unrelated work to v0.2.x maintenance without repeating that gate.

## Working rules

- Prefer the Go standard library. A new runtime dependency requires a recorded decision.
- Pass `context.Context` through blocking operations and subprocess boundaries.
- Inject clocks, listeners, and browser command runners where needed for deterministic tests.
- Avoid package-level mutable state.
- Treat auth, origin checks, shutdown, and concurrent tab tracking as security- and correctness-sensitive.
- Keep comments focused on invariants and non-obvious reasons.
- Update specs, facts, roadmap, changelog, and release governance with behavior or policy changes.

## Validation

- Default repository validation: `scripts/check.sh`
- Networked vulnerability/static/supply-chain validation: `scripts/check-security.sh`
- Mandatory v0.2.x candidate browser evidence: `scripts/check-browser-security.sh` through repository-owned Playwright Chromium, with CI preferred and a recorded exact-commit local run accepted as fallback. Extra engines and platforms are diagnostic unless a concrete compatibility issue makes them relevant.
- Release-candidate policy validation: `scripts/release-check.sh`
- Docs-only changes: `scripts/check-docs.sh` and `git diff --check`
- Raw Go commands in managed sandboxes must use a writable cache, for example `GOCACHE="$PWD/.cache/go-build" go test ./...`.
- Once runtime code exists, maintain at least 80% statement coverage overall and focused race coverage for lifecycle and tab tracking.

## Git and release

- Use feature branches and pull requests targeting `main`.
- Keep commits focused and stage the facts/docs that explain each behavior change with it.
- Never create a repository, push, publish, tag, or release unless the user explicitly requests that exact action.
- Releases are Go module tags, not binary artifacts. See `docs/dev/ops/release-governance.md`.
