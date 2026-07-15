# Engineering Facts

- Prefer clear, boring Go and the standard library.
- Pass `context.Context` through blocking IO, HTTP drain, shutdown guards, and browser subprocess boundaries.
- Keep lifecycle state private, synchronized, race-tested, and idempotent.
- Inject clocks/tickers, listeners, token entropy, and command runners only at narrow test seams.
- Never launch URLs through a shell.
- Never log, persist, or echo launch tokens except through the explicit capability URL returned to the consumer.
- Never store browser authentication capabilities in Web Storage or expose them through public JavaScript state. Keep bootstrap handling server-owned and credentials inaccessible to application JavaScript wherever the browser model permits.
- Do not treat a random loopback port as cookie or origin isolation; validate the effective host and prove sibling-service isolation.
- Security-sensitive paths fail closed. Compatibility options must not preserve a weaker credential mode.
- Treat loopback validation, auth, origin checks, cookie isolation, tab expiry, guard evaluation, and graceful shutdown as security/correctness-sensitive.
- Do not add speculative configuration or consumer-domain abstractions.
- Public examples must compile once implementation exists.
- Runtime behavior changes must remain within the greenlit v0.1 specification or update the specification and decision log in the same review.
- The v0.2.0 security implementation follows `docs/dev/specs/v0.2-api.md`; candidate evidence and release authorization are governed by the release facts and release governance.
