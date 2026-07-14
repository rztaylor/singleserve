# Architecture

## Purpose

Singleserve owns the process-local boundary between a Go application and the browser used as its UI. It composes around an application-provided `http.Handler`; it does not own application routing or domain behavior.

```text
process context
      |
      v
Singleserve Server -- binds loopback, authenticates, tracks tabs, owns shutdown
      |
      +-- /_singleserve/* control endpoints and browser client
      |
      `-- application http.Handler
              |
              +-- application API and business logic
              `-- application-owned embedded frontend assets
```

## Ownership boundaries

| Concern | Singleserve | Consumer application |
| --- | --- | --- |
| Listener | Validate and bind a loopback TCP address | Choose an optional loopback address |
| HTTP router | Reserve and serve `/_singleserve/`; wrap the supplied handler | Construct all application routes |
| Authentication | Generate and validate the per-launch token; bootstrap a launch-unique cookie | Avoid logging the token; use the client or documented headers |
| Browser | Construct the authenticated URL and launch the OS browser on request | Decide whether and when to open it; report launch errors |
| Presence | Track each browser-client tab heartbeat and disconnect | Choose explicit or browser-bound lifetime |
| Shutdown | Evaluate guards, stop accepting work, and gracefully drain HTTP | Define guard conditions and clean up domain resources after `Wait` |
| Frontend | Supply a plain ES-module lifecycle client | Choose framework, bundler, UI, routes, and assets |
| Persistence | None | Own all application state and recovery |

## Package layout

The v0.1 public surface is intentionally one package:

```text
github.com/rztaylor/singleserve
├── package singleserve          public server, policy, guard, result, and opener API
├── client/
    ├── singleserve.js           canonical dependency-free ES module
    ├── singleserve.test.mjs     Node built-in contract tests
    └── package.json             module/test metadata; no dependencies
└── examples/minimal/            executable consumer and HTTP smoke fixture
```

No project `cmd/` entrypoint is appropriate for a library. `examples/minimal` is an application-owned consumer demonstration, not a distributed Singleserve binary. No `pkg/` directory is needed because the module root is already the intentional import path. No `internal/` package is currently justified: root files remain cohesive around one lifecycle object, with private seams for time, entropy, listening, and platform commands. Future extraction must follow demonstrated ownership pressure rather than conceptual layering.

## Runtime state model

One `Server` represents one process launch and is single-use. `Start` binds first, then starts serving, and only then returns a `Launch` containing assigned address and URLs.

Server states are:

```text
new -> starting -> serving -> draining -> stopped
              \-> failed
```

- `Start` may be called once.
- A listener or serve failure is surfaced, never silently converted to a normal stop.
- Only one shutdown transition wins. Concurrent context cancellation, browser timeout, and explicit requests are idempotent.
- Parent-context cancellation and forced programmatic shutdown bypass application guards.
- Browser/user shutdown and browser-bound lifetime shutdown evaluate the guard.
- Graceful HTTP shutdown has a five-second default deadline.

## Tab model

The browser client creates a cryptographically random tab-instance ID. The server records first contact, last heartbeat, explicit disconnect, and heartbeat expiry per ID.

An immediate heartbeat marks a tab connected. A best-effort keepalive request on `pagehide` marks it disconnected; missed disconnects are resolved by heartbeat expiry. Reloads may create a new tab instance, so browser-bound shutdown includes a short disconnect grace window.

Tab tracking is active in both lifetime modes. Only browser-bound policy turns absence into a shutdown request. Recently disconnected records are kept briefly for snapshots and then pruned with a bounded in-memory policy; toolkit state never persists.

## Security model

The v0.1 trust boundary assumes a single local user but does not assume every local process or web page is trusted.

- Binding is restricted to IP loopback addresses or `localhost`; wildcard and non-loopback names are rejected.
- Each launch uses 32 bytes from `crypto/rand`, encoded with raw URL-safe base64.
- Token comparison is constant time after equal-length validation.
- A token-bearing initial safe navigation establishes a launch-unique, `HttpOnly`, `SameSite=Strict`, session cookie so application assets can load without query tokens.
- The browser client also sends `X-Singleserve-Token` and `X-Singleserve-Tab` headers.
- Query-token authentication is accepted only for `GET` and `HEAD`; the client removes it from the address bar immediately.
- Browser-originated unsafe requests with an `Origin` that does not equal the launch origin are rejected.
- Singleserve sends no permissive CORS headers.
- Reserved endpoint responses are `Cache-Control: no-store` and never echo the launch token.
- Singleserve-owned query, header, bearer, and cookie credentials are removed before delegation to application middleware; unrelated application credentials are preserved.
- URLs containing the token are capabilities. Applications may print one for manual launch but must not persist it in diagnostics or logs.

## Failure boundaries

- Browser-launch failure does not stop the server; the application can show the manual URL.
- A client declares the server unreachable after three consecutive failed heartbeats, while the server independently expires a tab after its heartbeat timeout.
- The browser client can report completed heartbeat observations to presentation code without giving the application control of scheduling or tab state.
- A typed shutdown denial returns HTTP 409 with a stable code and user-safe message.
- An unexpected guard error returns HTTP 500 and does not shut down.
- A browser-bound automatic shutdown denied by the guard is retried on later lifecycle checks so the process can exit when application work becomes safe.
- HTTP drain timeout returns an error from `Wait`; application cleanup remains the consumer's responsibility.
- A page may attempt `window.close()` after shutdown, but browser policy can reject closure of tabs not created by script; consumer UI must provide a terminal close-this-tab state.

## Non-goals for v0.1

- non-loopback or remote access;
- TLS and certificate management;
- multi-user authentication or authorization;
- WebSocket or server-sent-event transport;
- a router, web framework, asset builder, or frontend component system;
- desktop webviews, tray integration, auto-update, or background daemonization;
- application state, recovery files, locking, jobs, transactions, or business-specific guards; and
- compatibility layers for legacy consumer endpoint or header names.
