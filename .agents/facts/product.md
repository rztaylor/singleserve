# Product Facts

- Name: Singleserve.
- Module: `github.com/rztaylor/singleserve`.
- Product shape: public Go library plus a dependency-free browser ES module for single-binary, browser-based local applications.
- Current maturity: v0.1.0 release source; the initial tag is created from this reviewed commit.
- Primary audience: maintainers of local-first Go applications that use the system browser as their UI.

## Product boundaries

- Own loopback hosting, per-launch authentication, browser opening, health, per-tab presence, lifetime policy, graceful shutdown, and application shutdown guards.
- Consumers own their `http.Handler`, router, business logic, frontend framework, embedded assets, persistence, jobs, transactions, recovery, and user experience.
- v0.1 is loopback-only with no remote-access escape hatch.
- v0.1 must remain framework-neutral and standard-library-only on the Go side.
- Consumer applications are integration validators, not dependencies or architecture templates.

## Explicit non-goals

- Hosted or multi-user service, account identity, TLS, LAN mode, desktop webview, router, asset pipeline, application framework, application persistence, npm package, or consumer-specific compatibility layer.

## Decision state

- The v0.1 API/auth/client/timing choices in `docs/dev/decisions.md` are greenlit and implemented; changing them requires a spec and decision update.
