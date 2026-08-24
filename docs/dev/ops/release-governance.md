# Release governance

## Maturity and supported surface

Singleserve's current supported release line is `v0.2.x`. `v0.2.0` introduced the hardened browser-authentication contract; each patch becomes supported only after its candidate gates pass and its matching tag is explicitly authorized and created.

The supported release surface will be:

- the Go module at `github.com/rztaylor/singleserve`;
- the root `singleserve` package;
- the canonical browser ES module included in the module and served by the Go package;
- documented behavior on macOS, Linux/WSL, and Windows; and
- Go 1.26.6 as the minimum toolchain baseline; a security release may raise the patch floor to avoid a known toolchain vulnerability.

Remote hosting, TLS, npm distribution, binaries, installers, signing, notarization, SBOMs, attestations, hosted services, and compatibility aliases are unsupported.

## Versioning and compatibility

- Use Semantic Versioning tags in `vMAJOR.MINOR.PATCH` form.
- Before `v1.0.0`, breaking public API or wire-contract changes require a minor version and explicit changelog migration notes.
- Patch releases must remain backward compatible within their minor line except for urgent security corrections that cannot be made safely otherwise.
- The Go module path has no major suffix until a future `v2`.
- The browser client and reserved HTTP wire contract version together with the Go module.

## Changelog and release notes

- `CHANGELOG.md` is the curated release-note source.
- Keep pending entries under `Unreleased` using Keep a Changelog categories.
- For a release, add a dated version heading and update comparison links.
- GitHub Release notes should be copied from the matching changelog section, not generated from raw commit subjects.
- Compatibility, security, support, or release-policy changes also update `docs/dev/decisions.md` when rationale must remain durable.

## Validation gates

Default validation:

```sh
scripts/check.sh
```

Release-candidate validation:

```sh
scripts/release-check.sh v0.2.2
```

For v0.2.x, the manual hosted candidate workflow runs that command with the pinned networked security scan and mandatory Playwright Chromium harness. CI is preferred, but a recorded local run at the exact candidate commit is an accepted fallback. Candidate validation fails closed when its browser, vulnerability database, or other required evidence is unavailable.

Before a release, also require:

- every applicable acceptance criterion in `docs/dev/specs/v0.2-api.md`;
- race-enabled lifecycle tests and at least 80% overall Go statement coverage;
- browser smoke validation when its declared prerequisites are available;
- the two representative repository-owned public-API compatibility harnesses;
- secret/private-data scan of tracked files and generated release notes;
- a clean worktree at the intended commit; and
- explicit user authorization for tag and publication.

Skipped validation must name the missing prerequisite and risk. Required unit, race, auth, lifetime, and shutdown gates may not be skipped for a release. For v0.1.0, no supported real browser is installed in the release environment, so the real-browser two-tab smoke is skipped. The remaining risk is browser-engine behavior around token bootstrap, tab-close delivery, and the example's terminal recovery UI; HTTP smoke and browser-client unit tests remain required and pass.

For v0.2.x, `scripts/check-browser-security.sh` covers real-browser bootstrap and owner-controlled renewal, clean URL/referrer behavior, no browser persistence or token API, Secure `__Host-` cookie acceptance, reload/new-tab behavior, multi-tab tracking, capability replay, ordinary sibling isolation, and related `.localhost` domain-cookie injection. One passing Playwright Chromium run at the exact candidate commit is mandatory. Repository-owned CI is preferred; a recorded local run is an accepted fallback. The runner OS is an implementation detail and no browser/OS cross-product is a standing release requirement. WebDriver and extra engine/platform runs are optional diagnostics unless a concrete compatibility finding or explicit support claim makes them relevant. `scripts/check-security.sh` runs vet, pinned `govulncheck` v1.6.0 with database metadata, immutable-action verification, and the tracked-file secret scan. CodeQL remains defense-in-depth: actionable findings block release, while unavailable hosted analysis alone does not when all mandatory candidate checks pass.

## Artifacts and publishing

Singleserve publishes source through the Go module proxy when a Git tag becomes public. It produces no binary archives or checksums. Consumers own binary packaging.

The manual hosted release-candidate workflow validates only and has read-only repository permissions. There is no hosted publication workflow; release actions remain intentionally manual:

1. verify the proposed tag does not already exist;
2. run release checks at the exact commit;
3. review the changelog excerpt and module contents;
4. create an annotated release tag only with explicit permission;
5. push the tag only with explicit permission; and
6. create matching GitHub Release notes when a repository exists.

Ordinary validation requires no credentials. Publishing requires existing Git/GitHub credentials but must never ask for tokens in chat.

## Hosted workflow policy

If release automation is added later, it must live under `.github/workflows/`, trigger only from an existing `v*` tag or explicit manual dispatch, use `contents: write` only in the publish job, regenerate notes from the changelog, refuse tag/version mismatch, and never create or move tags implicitly. No release secrets beyond the scoped GitHub token should be necessary.

## Prereleases, reruns, and rollback

- Release candidates use tags such as `v0.1.0-rc.1` and GitHub prerelease status.
- A failed unpublished run may be rerun at the same commit.
- Published tags are immutable. Fix errors with a new patch or prerelease tag; never move or overwrite a public tag.
- If a release is unsafe, mark it clearly in GitHub, document the issue under `Security` or the appropriate changelog category, and publish a corrective version. Removing a tag does not reliably retract module-proxy content.

## Release blockers

Do not release when:

- a listener can bind non-loopback;
- authentication, origin, token non-echo, tab isolation, guard, or drain tests fail;
- browser credentials remain in script-readable storage or public JavaScript state;
- capability-bearing navigation reaches consumer content before server-owned scrubbing;
- a sibling loopback service can receive, overwrite, or replay a launch cookie;
- exact Host, authentication-mode-specific Origin, opener, resource-bound, or required v0.2.x browser/security evidence is missing or failing;
- the client and served embedded source differ;
- the public docs claim unimplemented behavior;
- a repository-owned public compatibility harness exposes an abstraction gap;
- CodeQL or another analyzer reports an actionable security finding;
- tracked files contain launch tokens, credentials, private consumer data, local paths, caches, or generated private artifacts; or
- licensing or copyright metadata is inconsistent.
