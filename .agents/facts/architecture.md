# Architecture Facts

- The intentional public API is one root package, `singleserve`.
- `Options.Handler` accepts the consumer-owned `http.Handler`; Singleserve wraps it without mutating or introspecting its router.
- `/_singleserve/` is the reserved control path for health, heartbeat, disconnect, shutdown, and the browser client.
- One `Server` represents one launch and is single-use. `Start` binds before returning a `Launch`; `Wait` returns the winning lifecycle result.
- Toolkit state is process-local and non-persistent.
- Authentication uses a fresh 256-bit token, a launch-unique strict session cookie, and documented token/tab headers.
- Tab state is tracked independently and aggregated only for browser-bound lifetime decisions.
- Parent context cancellation and forced owner shutdown bypass guards; browser/user-driven shutdown evaluates them.
- The canonical browser client lives at `client/singleserve.js`, is served byte-for-byte by the Go package, and exposes heartbeat observations without delegating lifecycle scheduling.
- Do not create `pkg/`, a project `cmd/`, or empty future packages. `examples/minimal` is a consumer demonstration, not a library-owned command. Extract `internal/` packages only after real implementation responsibilities justify them.
- Detailed ownership and state models live in `docs/dev/architecture.md`; the normative API lives in `docs/dev/specs/v0.1-api.md`.
