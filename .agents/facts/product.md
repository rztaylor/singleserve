# Product Facts

- Name: Singleserve.
- Module: `github.com/rztaylor/singleserve`.
- Product shape: public Go library plus a dependency-free browser ES module for single-binary, browser-based local applications.
- Current maturity: v0.1.0 is the supported tag; v0.2.0 security behavior is implemented in the working source but remains unsupported until its matching tag exists.
- Primary audience: maintainers of local-first Go applications that use the system browser as their UI.
- Security posture: security is a non-negotiable release requirement and takes precedence over pre-1.0 compatibility and schedule.

## Product boundaries

- Own loopback hosting, per-launch authentication, browser opening, health, per-tab presence, lifetime policy, graceful shutdown, and application shutdown guards.
- Consumers own their `http.Handler`, router, business logic, frontend framework, embedded assets, persistence, jobs, transactions, recovery, and user experience.
- v0.1 is loopback-only with no remote-access escape hatch.
- v0.1 must remain framework-neutral and standard-library-only on the Go side.
- Consumer applications are integration validators, not dependencies or architecture templates.
- Remote web pages, sibling loopback services, and unrelated local processes are not trusted merely because the listener is local.

## Explicit non-goals

- Hosted or multi-user service, account identity, TLS, LAN mode, desktop webview, router, asset pipeline, application framework, application persistence, npm package, or consumer-specific compatibility layer.

## Decision state

- The v0.1 API/auth/client/timing choices in `docs/dev/decisions.md` are greenlit and implemented; changing them requires a spec and decision update.
- v0.2.0 is an implemented, unreleased security-only minor release in final release preparation. Its scope is closed; no unrelated feature may enter it.
