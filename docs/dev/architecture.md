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
| Authentication | Generate and rotate the single active bootstrap capability; own browser-session and programmatic credentials plus the isolated origin | Protect explicit browser URLs; decide whether to expose owner-controlled renewal; use `Launch.Client` for programmatic requests |
| Browser | Serve and open the fixed clean bootstrap before consumer content | Decide whether and when to open it; report launch errors |
| Presence | Track each browser-client tab heartbeat and disconnect | Choose explicit or browser-bound lifetime |
| Shutdown | Evaluate guards, stop accepting work, and gracefully drain HTTP | Define guard conditions and clean up domain resources after `Wait` |
| Frontend | Supply a plain ES-module lifecycle client | Choose framework, bundler, UI, routes, and assets |
| Persistence | None | Own all application state and recovery |

## Package layout

The public surface remains intentionally one package:

```text
github.com/rztaylor/singleserve
├── package singleserve          public server, policy, guard, result, and opener API
├── client/
    ├── bootstrap.js             fixed exchange before consumer content
    ├── singleserve.js           canonical dependency-free ES module
    ├── *.test.mjs               Node built-in bootstrap and lifecycle tests
    ├── playwright-driver.mjs    test-only adapter for hosted browser engines
    └── package.json             module/test metadata; Playwright dev dependency
├── browser_security_test.go    build-tagged real-browser release contract
├── internal/testtransport/     test-only browser-like .localhost resolution
└── examples/minimal/            executable consumer and HTTP smoke fixture
```

No project `cmd/` entrypoint is appropriate for a library. `examples/minimal` is an application-owned consumer demonstration, not a distributed Singleserve binary. No `pkg/` directory is needed because the module root is already the intentional import path. Runtime files remain cohesive around one lifecycle object, with private seams for time, entropy, listening, and platform commands. `internal/testbrowserjar` models modern browsers accepting Secure `__Host-` cookies on trustworthy `.localhost` HTTP origins, while `internal/testtransport` gives repository HTTP harnesses browser-like `.localhost` loopback resolution on every CI platform; production code imports neither. Future extraction must follow demonstrated ownership pressure rather than conceptual layering.

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

The released v0.2 contract retains the single-local-user product boundary but does not trust remote pages, sibling loopback services, or unrelated local processes.

- Listener configuration accepts only `127.0.0.1`, `::1`, or `localhost`; every request must use the exact high-entropy `ss-<128-bit>.localhost:<port>` launch Host.
- Each launch generates independent 256-bit bootstrap, browser-session, and programmatic credentials with `crypto/rand`; comparisons are constant time after exact length validation.
- `Launch.URL()` puts the current two-minute, one-time bootstrap capability in a fragment. `Launch.NewBootstrapURL()` atomically replaces any earlier unconsumed capability and starts a fresh two-minute window without changing lifetime or session state. A fixed no-store/no-referrer page scrubs the fragment before exchange and replaces itself with `/` before consumer content runs.
- Browser authentication thereafter uses only a host-isolated, `HttpOnly`, `Secure`, `SameSite=Strict`, non-persistent `__Host-` cookie. The prefix prevents related-domain injection; the lifecycle module reads no credential and uses no browser persistence.
- `Launch.Client()` privately applies the independent programmatic header only to the exact clean origin, disables environment proxies, refuses off-origin redirects, and dials the known listener address directly while preserving the isolated request Host.
- Missing, invalid, duplicate, or ambiguous credentials fail closed. Browser-cookie unsafe methods and bootstrap exchange require the exact Origin; programmatic requests may omit Origin but cannot supply a mismatch.
- Query and bearer authentication are removed. Singleserve removes its headers, session cookie, tab metadata, and legacy query keys before application delegation while preserving application credentials.
- Control responses are non-cacheable and non-sensitive, control requests accept no body, and the HTTP server bounds header time, header bytes, and idle connections.
- Opener commands receive the URL as one argument and never invoke a shell or command interpreter.

### Known v0.1.0 browser-authentication limitation

The v0.1.0 client persists the launch capability in script-readable `sessionStorage`, returns it as `session.token`, and relies on application JavaScript to scrub the launch URL. Its session cookie is launch-unique by name but host-scoped, so another service on a different port of the same loopback host can receive it. These are limitations of the released v0.1.0 design, not properties to preserve.

The released v0.2 contract implements the replacement. A high-entropy hostname protects the cookie from ordinary sibling origins; knowledge of that hostname is capability-adjacent. `HttpOnly` prevents credential reads but does not stop trusted same-origin consumer code from issuing authenticated requests. Real-browser, test, vulnerability, static, supply-chain, and secret evidence remain release-blocking for v0.2.x patch candidates; hosted CodeQL availability is defense-in-depth rather than a release property.

## Failure boundaries

- Browser-launch failure does not stop the server; trusted owner code can issue and show a fresh manual URL. Singleserve does not provide or imply a renewal UI.
- A client declares the server unreachable after three consecutive failed heartbeats, while the server independently expires a tab after its heartbeat timeout.
- The browser client can report completed heartbeat observations to presentation code without giving the application control of scheduling or tab state.
- A typed shutdown denial returns HTTP 409 with a stable code and user-safe message.
- An unexpected guard error returns HTTP 500 and does not shut down.
- A browser-bound automatic shutdown denied by the guard is retried on later lifecycle checks so the process can exit when application work becomes safe.
- HTTP drain timeout returns an error from `Wait`; application cleanup remains the consumer's responsibility.
- A page may attempt `window.close()` after shutdown, but browser policy can reject closure of tabs not created by script; consumer UI must provide a terminal close-this-tab state.

## Non-goals for v0.2

- non-loopback or remote access;
- TLS and certificate management;
- multi-user authentication or authorization;
- WebSocket or server-sent-event transport;
- a router, web framework, asset builder, or frontend component system;
- desktop webviews, tray integration, auto-update, or background daemonization;
- application state, recovery files, locking, jobs, transactions, or business-specific guards; and
- compatibility layers for legacy consumer endpoint or header names.
