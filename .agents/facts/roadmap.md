# Roadmap Facts

- Active roadmap: `docs/dev/roadmap.md`.
- Active statuses: `Pending`, `Partial`, and `Blocked`; `Done` is only transitional before removal.
- IDs are short, unique, stable, and kebab-case.
- Detailed scope belongs in `docs/dev/roadmap-items/`; implementation sequencing belongs in `docs/dev/plans/`.
- `security-hardening` is the sole active, Partial roadmap item for v0.2.0, with ExecPlan `docs/dev/plans/000-v0.2.0-security-hardening.md`; implementation and platform-neutral Playwright Chromium automation are present, while candidate evidence and closure remain.
- v0.2.0 is security-only; unrelated deferred features do not share its implementation or release gate.
- The completed first-release validation used two representative repository-owned compatibility harnesses that exercise only the public Singleserve API.
- Real-browser smoke remains conditional on a supported browser and stable harness; v0.1.0 records its unavailable-browser skip and residual risk in release governance.
- No release action is implied by roadmap completion; tag/publish requires explicit authorization.
- After a plan completes, move durable behavior/rationale to specs, decisions, changelog, and release docs, then delete the plan and close/remove the active roadmap item.
