# Release Facts

## Maturity and support

- Current maturity: v0.2.x is the supported release line; v0.2.0 introduced the released security contract, and each patch requires matching candidate evidence and a tag.
- First release: `v0.1.0` after two representative repository-owned compatibility harnesses and explicit release authorization. Real-browser smoke is conditional on a supported browser and stable harness.
- Supported release shape: Go module source, root package, and canonical browser ES module.
- Unsupported: binaries, archives, checksums, signing, notarization, SBOMs, attestations, installers, npm, TLS, remote access, and hosted service claims.

## Versioning and notes

- Use Semantic Versioning tags `vMAJOR.MINOR.PATCH`; prereleases use forms such as `v0.1.0-rc.1`.
- Pre-1.0 breaking changes require a minor version and migration notes.
- Security takes precedence over pre-1.0 compatibility. v0.2.0 may remove the v0.1 browser token-storage and exposure contract rather than retain an insecure compatibility mode.
- Changelog: `CHANGELOG.md`; release notes come from the matching curated section.
- Governance: `docs/dev/ops/release-governance.md`.
- Durable release/support decisions: `docs/dev/decisions.md`.

## Validation and publishing

- Default validation: `scripts/check.sh`.
- Release candidate: `scripts/release-check.sh <version>` plus the applicable specification gates.
- Core auth, tab, lifetime, shutdown, race, and coverage gates cannot be skipped.
- The historic v0.1 browser prerequisite skip remains documented. v0.2.x requires one passing repository-owned Playwright Chromium contract at the exact candidate commit; CI is preferred and a recorded local run is an accepted fallback.
- Extra engine/platform runs are not release prerequisites. The mandatory v0.2.x Chromium evidence covers authentication, URL scrubbing, reload, multi-tab behavior, and sibling-loopback isolation.
- CodeQL remains hosted defense-in-depth. Any actionable result blocks release; unavailable hosted analysis alone does not block when the candidate's required browser, test, vulnerability, static, supply-chain, and secret checks pass.
- Consumer migration is post-tag validation. Repository-owned compatibility harnesses guard the public API before release; a gap first demonstrated by SQLRise or another consumer is corrected in a subsequent patch release.
- A manual, read-only hosted workflow can validate a release candidate but cannot tag or publish. No hosted publication workflow exists; tag, push, and GitHub Release creation require explicit permission.
- Module releases produce no project-owned artifact beyond source at the tag.
- Published tags are immutable; recover with a new version.
- Ordinary validation requires no credentials. Future hosted publishing should need only narrowly scoped GitHub contents permission.

## Release blockers

- Non-loopback binding, auth/origin/token leakage, cookie collision, tab isolation, guard, or drain failures.
- Client source differing from served bytes.
- Inaccurate implemented/planned status in docs.
- A browser validation skip without its missing prerequisite and stated risk.
- Secrets, private consumer data, local paths, caches, or generated private artifacts in tracked content.
- Browser credentials are persisted in script-readable storage or exposed through the browser-client API.
- Capability URLs reach application content, referrers, caches, or clean navigation history before server-owned scrubbing.
- Session cookies can be disclosed to a sibling loopback service under the supported launch origin.
- Effective Host, Origin, authentication-mode, opener, vulnerability-scan, static-analysis, or required browser-security evidence is missing or failing.
