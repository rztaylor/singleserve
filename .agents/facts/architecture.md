# Architecture Facts

- The intentional public API is one root package, `singleserve`.
- `Options.Handler` accepts the consumer-owned `http.Handler`; Singleserve wraps it without mutating or introspecting its router.
- `/_singleserve/` is the reserved control path for health, heartbeat, disconnect, shutdown, and the browser client.
- One `Server` represents one launch and is single-use. `Start` binds before returning a `Launch`; `Wait` returns the winning lifecycle result.
- Toolkit state is process-local and non-persistent.
- The unreleased v0.2 source generates independent 256-bit bootstrap, browser-session, and programmatic credentials plus a 128-bit high-entropy `.localhost` origin label.
- v0.2 browser bootstrap is a two-minute, atomically consumed URL-fragment exchange owned by fixed Singleserve HTML/JavaScript; `Launch.NewBootstrapURL()` lets trusted owner code rotate the single active unconsumed capability and reset only that window; consumer content first runs at the clean base URL.
- Browser authentication uses only a host-isolated HttpOnly, Secure, strict `__Host-` session cookie. Exact Host and authentication-mode-specific Origin validation occur before delegation.
- Programmatic callers use `Launch.Client()`, which privately authenticates only the exact launch origin and bypasses environment proxies.
- Tab state is tracked independently and aggregated only for browser-bound lifetime decisions.
- Parent context cancellation and forced owner shutdown bypass guards; browser/user-driven shutdown evaluates them.
- The canonical browser client lives at `client/singleserve.js`, is served byte-for-byte by the Go package, and exposes heartbeat observations without delegating lifecycle scheduling.
- The v0.2.0 client keeps lifecycle scheduling but contains no Web Storage, public token state, or application-JavaScript bootstrap responsibility.
- Do not create `pkg/`, a project `cmd/`, or empty future packages. `examples/minimal` is a consumer demonstration, not a library-owned command. `internal/testbrowserjar` is narrowly owned test support for Secure localhost cookie semantics; extract other `internal/` packages only after real responsibilities justify them.
- Detailed ownership and state models live in `docs/dev/architecture.md`; versioned normative APIs live under `docs/dev/specs/`.
