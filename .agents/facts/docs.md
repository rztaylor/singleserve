# Documentation Facts

- Repository orientation: `README.md`.
- Curated changelog: `CHANGELOG.md`, Keep a Changelog categories under `Unreleased` until release.
- Developer architecture: `docs/dev/architecture.md`.
- Durable decisions: `docs/dev/decisions.md`.
- Normative specs: `docs/dev/specs/`.
- Contributor guides and migration: `docs/dev/guides/`.
- Validation, CI, security, and release operations: `docs/dev/ops/`.
- Active roadmap index: `docs/dev/roadmap.md`.
- Roadmap briefs: `docs/dev/roadmap-items/<id>.md` using stable kebab-case IDs.
- Active ExecPlans, when needed: `docs/dev/plans/NNN-short-title.md`, with monotonic zero-padded IDs.
- Planned behavior must be labelled proposed and never documented as implemented.
- Behavior or wire changes update the normative spec; ownership changes update architecture; compatibility/release changes update changelog, decisions, and release governance.
- Remove completed roadmap items and stale ExecPlans after durable outcomes move to long-lived docs.
