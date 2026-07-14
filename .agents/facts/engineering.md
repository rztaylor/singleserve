# Engineering Facts

- Prefer clear, boring Go and the standard library.
- Pass `context.Context` through blocking IO, HTTP drain, shutdown guards, and browser subprocess boundaries.
- Keep lifecycle state private, synchronized, race-tested, and idempotent.
- Inject clocks/tickers, listeners, token entropy, and command runners only at narrow test seams.
- Never launch URLs through a shell.
- Never log, persist, or echo launch tokens except through the explicit capability URL returned to the consumer.
- Treat loopback validation, auth, origin checks, cookie isolation, tab expiry, guard evaluation, and graceful shutdown as security/correctness-sensitive.
- Do not add speculative configuration or consumer-domain abstractions.
- Public examples must compile once implementation exists.
- Runtime behavior changes must remain within the greenlit v0.1 specification or update the specification and decision log in the same review.
