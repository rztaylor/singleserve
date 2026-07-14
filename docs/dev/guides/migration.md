# Consumer migration strategy

## Principles

Migrations replace only duplicated local-server infrastructure. Each application keeps its router, business services, embedded assets, CLI, user messages, persistence, and domain cleanup.

Because a consumer ships its Go server and browser assets together, migration should be atomic within that application's binary. v0.1 does not carry aliases for legacy tokens, headers, routes, or payloads.

## Replace

- local launch-token generation and lifecycle authentication middleware;
- process-wide browser-presence timestamps and idle watchdogs;
- generic health, heartbeat, disconnect, and shutdown handlers;
- direct operating-system browser command selection; and
- generic frontend token, heartbeat, disconnect, and shutdown plumbing.

## Keep

- the application `http.Handler`, router, and API routes;
- domain services, background work, and application state;
- CLI and parent/child process orchestration;
- frontend framework, recovery UI, and user-facing messages;
- persistence, locks, transactions, and cleanup; and
- application-owned embedded assets.

## Adapt

- Construct the application router without lifecycle middleware and pass it as `Options.Handler`.
- Choose `BrowserBoundLifetime()` only when browser presence should own process lifetime; otherwise use `ExplicitLifetime()`.
- Map application work-safety checks into a `ShutdownGuardFunc` with stable, user-safe denial codes and messages.
- Wrap the dependency-free browser client in the application's existing frontend state or API boundary.
- Use `Wait`'s result reason to coordinate application-owned recovery and cleanup exactly once.
- Keep detached-process startup, manual URL reporting, and exit presentation in the consumer's CLI boundary.

## Migration checks

- Existing unsafe-shutdown scenarios remain denied and user-visible.
- Explicit lifetime never exits merely because no browser tab contacted it.
- Browser launch failure still presents a usable authenticated manual URL.
- Packaged assets load after query-token scrubbing.
- Closing one of two tabs does not stop the application; closing the last tab follows the configured policy.
- A browser-bound process with no first contact exits within its configured deadline.
- Browser absence while a guard is active does not discard work; automatic shutdown proceeds after the guard clears.
- Recovery and cleanup happen exactly once for every shutdown reason.
- Simultaneous application processes do not collide in authentication cookies.

## Validation sequence

1. Implement and test Singleserve behind the v0.1 specification.
2. Add one repository-owned compatibility harness and feed only generic abstraction gaps back into the specification.
3. Add a second repository-owned harness with materially different routing, frontend, and shutdown concerns.
4. Resolve only requirements that belong to the local browser lifecycle boundary.
5. Run Singleserve release checks and both compatibility harnesses.
6. Tag `v0.1.0` only after the public API survives both validations.

Future consumers should adopt the released API and motivate changes only from real implementation evidence.
