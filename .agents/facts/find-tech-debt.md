# Technical Debt Facts

- Technical debt uses dedicated pending items in `docs/dev/roadmap.md` with briefs under `docs/dev/roadmap-items/`; do not bury cleanup inside feature items.
- `Fix now` findings that affect authentication, loopback safety, lifecycle correctness, public API, or release validation block review handoff and release.
- `Track` findings need a demonstrated current-code maintenance or correctness cost. Speculative future separation remains `Watch` and is not added to the roadmap.
- Use package files and routine names in durable entries, not line numbers.
- Real-browser smoke and consumer validation are declared product validation gates, not technical debt.
- No unresolved technical-debt item is registered for the v0.1.0 release source.
- Confirmed security gaps are not deferred as ordinary debt. The v0.1 browser-token persistence, client token exposure, application-JavaScript URL scrubbing, and cookie port-isolation concerns are release-blocking scope in `security-hardening` for v0.2.0.
