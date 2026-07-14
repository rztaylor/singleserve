# Release Facts

## Maturity and support

- Current maturity: v0.1.0 release source. The initial tag is created from this reviewed commit.
- First release: `v0.1.0` after two representative repository-owned compatibility harnesses and explicit release authorization. Real-browser smoke is conditional on a supported browser and stable harness.
- Supported release shape: Go module source, root package, and canonical browser ES module.
- Unsupported: binaries, archives, checksums, signing, notarization, SBOMs, attestations, installers, npm, TLS, remote access, and hosted service claims.

## Versioning and notes

- Use Semantic Versioning tags `vMAJOR.MINOR.PATCH`; prereleases use forms such as `v0.1.0-rc.1`.
- Pre-1.0 breaking changes require a minor version and migration notes.
- Changelog: `CHANGELOG.md`; release notes come from the matching curated section.
- Governance: `docs/dev/ops/release-governance.md`.
- Durable release/support decisions: `docs/dev/decisions.md`.

## Validation and publishing

- Default validation: `scripts/check.sh`.
- Release candidate: `scripts/release-check.sh <version>` plus the applicable specification and compatibility gates.
- Core auth, tab, lifetime, shutdown, race, and coverage gates cannot be skipped.
- Browser prerequisite skips must be explicit.
- No hosted release workflow exists. The first release is manual and requires explicit permission for tag, push, and GitHub Release creation.
- Module releases produce no project-owned artifact beyond source at the tag.
- Published tags are immutable; recover with a new version.
- Ordinary validation requires no credentials. Future hosted publishing should need only narrowly scoped GitHub contents permission.

## Release blockers

- Non-loopback binding, auth/origin/token leakage, cookie collision, tab isolation, guard, or drain failures.
- Client source differing from served bytes.
- Inaccurate implemented/planned status in docs.
- A browser validation skip without its missing prerequisite and stated risk.
- Secrets, private consumer data, local paths, caches, or generated private artifacts in tracked content.
