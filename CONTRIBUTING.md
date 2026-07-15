# Contributing

Singleserve v0.1.0 is the first supported release. Specification corrections, design feedback, and focused fixes are welcome; follow the documented pre-1.0 versioning policy for public compatibility changes.

## Before changing the repository

1. Read `AGENTS.md` and the relevant `.agents/facts/` files.
2. Read `docs/dev/specs/v0.1-api.md` for the supported release and `docs/dev/specs/v0.2-api.md` for unreleased v0.2.0 behavior. Security work also requires the active roadmap item and ExecPlan.
3. Use a feature branch and keep the change focused.
4. Update durable docs and the changelog when public behavior or policy changes.

## Validation

Run:

```sh
scripts/check.sh
```

The check must be non-interactive and must not require network access, npm installation, Docker, a browser, or external services. It includes race-enabled tests and the Node built-in browser-client suite.

`scripts/check.sh` is the complete required local-development gate on macOS, Linux, and Windows; contributors are not required to install a browser driver, use a particular OS, enable Safari automation, or run downloaded browsers outside their preferred controls. Security changes also receive repository-owned `scripts/check-security.sh` and Playwright Chromium evidence in CI. Missing candidate CI evidence blocks release, while local Playwright/WebDriver and extra engine/platform runs remain optional diagnostics.

## Public API changes

Before 1.0, breaking changes are permitted only in a minor release and must be documented. Changing an exported name, control endpoint, wire shape, authentication rule, or lifetime semantic requires updates to:

- the applicable versioned specification under `docs/dev/specs/`;
- `docs/dev/architecture.md` when ownership changes;
- `docs/dev/decisions.md` when rationale changes;
- an active roadmap item and ExecPlan when the change needs planning; and
- `CHANGELOG.md`.

## Security reports

Do not open a public issue for a suspected vulnerability. Follow [SECURITY.md](SECURITY.md).
