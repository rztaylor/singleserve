# CI proposal

The checked-in workflow at `.github/workflows/ci.yml` uses read-only permissions and performs no publishing.

## Pull request and main checks

- Ubuntu runs `scripts/check.sh`, including the `examples/minimal` build and public-API HTTP smoke test, docs, format, tests, coverage, vet, race, and browser-client checks.
- Node 24 is installed only for the dependency-free browser-client tests; CI runs no package installation.
- A platform matrix runs `go test ./...` on Ubuntu, macOS, and Windows so platform opener files compile and platform-specific tests execute.
- Go is selected from `go.mod`. Dependency caching stays disabled while the standard-library-only module has no `go.sum`; enable it only if a real dependency creates one.
- No secrets or write permissions are required.

## Deferred CI

- Real-browser smoke tests remain conditional until a supported browser and stable harness are available. v0.1.0 records this as an explicit skipped release check rather than claiming browser-engine coverage.
- CodeQL, `govulncheck`, dependency licences, coverage upload, and release automation should be evaluated after runtime code exists.
- Required status-check and branch-protection configuration requires a real GitHub repository and explicit authorization.

## Dependabot proposal

`.github/dependabot.yml` proposes monthly grouped checks for Go modules and GitHub Actions with a low pull-request limit. The current runtime dependency goal is standard-library-only; Go module checks still catch future test/tool dependencies.
