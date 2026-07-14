# Decision log

## Decided foundation

### Public module and package

- **Decision:** use module `github.com/rztaylor/singleserve` and one root public package named `singleserve`.
- **Why:** the toolkit is one cohesive lifecycle boundary; `pkg/` or multiple public packages would add navigation and compatibility cost before real separation exists.

### Licence

- **Decision:** MIT License, copyright Robert Taylor, 2026.
- **Why:** Singleserve is integration infrastructure intended for broad use in open-source and proprietary local apps. MIT keeps obligations simple and includes a standard warranty disclaimer.
- **Rejected:** no licence (prevents clear public reuse), GPL-family copyleft (stronger downstream conditions than intended), and Apache-2.0 (useful explicit patent language, but more text/notice machinery than this small library currently needs).

### Local-only security boundary

- **Decision:** v0.1 rejects non-loopback binds with no escape hatch.
- **Why:** launch-token browser apps are not designed as remote services. A “dangerous but explicit” option would create a support and security surface outside the v0.1 product boundary.

### Router and asset ownership

- **Decision:** accept and wrap an application `http.Handler`; reserve only `/_singleserve/`.
- **Why:** applications own their routers and embedded frontend handling. Taking router registration or asset embedding into the toolkit would couple unrelated application architecture.

### Dependency policy

- **Decision:** standard library only for the Go runtime; plain browser APIs for the client.
- **Why:** the required cryptography, networking, HTTP, subprocess, timing, and embedding behavior are all available without dependencies.

### Lifetime semantics

- **Decision:** explicit lifetime is the safe zero-value; browser opening and browser-bound lifetime are independent.
- **Why:** libraries should not terminate an owner process merely because a browser was opened. Consumers opt into browser ownership deliberately.

### Release shape

- **Decision:** releases are source module tags and GitHub release notes, not binaries or archives.
- **Why:** Singleserve is imported into consumer binaries. Checksums, signing, installers, and platform archives belong to those applications.

## Greenlit v0.1 choices

The implementation greenlight accepted the following choices. They are now part of the v0.1 release contract.

### Start/Launch API rather than a blocking Run callback

- **Recommendation:** keep `New -> Start -> optional OpenBrowser -> Wait`.
- **Tradeoff:** it adds a `Launch` type, but avoids readiness callbacks, exposes the assigned port before browser opening, and gives consumers a clean manual-URL recovery path.
- **Alternative:** a single blocking `Run` with callbacks is smaller but harder to compose and test.

### Dual cookie and header authentication

- **Recommendation:** use a launch-unique strict session cookie for ordinary asset loading plus token headers for the browser client and programmatic requests.
- **Tradeoff:** cookie logic needs careful multi-process tests, but fully protecting an arbitrary app handler is otherwise incompatible with normal browser asset requests.
- **Alternative:** leave index/assets public and protect only application APIs, which Singleserve cannot identify without taking over routing.

### Embedded canonical ES module

- **Recommendation:** keep `client/singleserve.js` as canonical source and serve the identical bytes at `/_singleserve/client.js`.
- **Tradeoff:** consumers can import it directly with no frontend build, while bundler users may vendor the file; there is no npm package in v0.1.
- **Alternative:** Go-only helpers would leave duplicated heartbeat/auth logic in each consumer.

### Browser-bound timing defaults

- **Recommendation:** 2-minute first contact, 15-second heartbeat expiry, 5-second disconnect grace, and 1-second lifecycle checks.
- **Tradeoff:** final-tab close normally stops within about 20 seconds; the grace prevents reload churn from terminating the process.
- **Alternative:** retain a five-minute process-wide idle timeout, which is slower and cannot model tabs.

### Consumer validation before v0.1.0

- **Decision:** require two representative repository-owned compatibility harnesses before `v0.1.0`.
- **Outcome:** `TestBrowserBoundConsumerHarness` and `TestExplicitConsumerHarness` satisfy the requirement using only the public API.
- **Tradeoff:** this delayed v0.1.0 but proved the abstraction against independently designed application shapes while keeping release evidence inside this repository.
- **Alternative:** tag after library tests alone and accept higher public-API revision risk.

### Test-only JavaScript runtime

- **Decision:** use Node 24 or newer and its built-in test runner for `client/singleserve.js`; add no npm dependencies and no frontend build.
- **Why:** the browser client needs deterministic contract tests, while the shipped source remains plain JavaScript and the Go runtime remains standard-library-only.
- **Boundary:** Node is a repository validation prerequisite, not a consumer runtime requirement.

## Unresolved after v0.1

- Whether a later release should publish the browser client to npm with TypeScript declarations.
- Whether a later release should expose tab event observers rather than snapshots only.
- Whether configurable control prefixes are justified by a real route collision.
- Whether minimum-Go support should expand below the declared Go 1.26 baseline.
- Whether GitHub tag/release-note automation is worthwhile after the first manual release.

None of these questions should delay v0.1 implementation or add speculative API now.
