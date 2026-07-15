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

- **Decision:** use Node 24 or newer and its built-in test runner for deterministic `client/singleserve.js` checks; ordinary validation requires no npm installation or frontend build.
- **Why:** the browser client needs deterministic contract tests, while the shipped source remains plain JavaScript and the Go runtime remains standard-library-only.
- **Boundary:** Node is a repository validation prerequisite, not a consumer runtime requirement.

## Greenlit v0.2 direction

### Platform-neutral browser evidence

- **Decision:** run the repository-owned Go browser-security contract through pinned Playwright Chromium in CI. The CI runner OS is an implementation detail, not part of the release requirement or product support claim. Retain direct WebDriver and other Playwright engines only as optional diagnostic paths.
- **Why:** cookie acceptance, `.localhost` handling, navigation, referrer, storage, and tab lifecycle are exercised by the real browser contract. One maintained browser path gives repeatable evidence without imposing a browser/OS cross-product or requiring maintainers to weaken workstation controls.
- **Boundary:** Playwright is a test-only development dependency used by the networked candidate gate. `scripts/check.sh` is the complete local-development gate and requires no npm install, browser, network, or particular operating system. Additional engines or platforms become required only when a concrete compatibility finding or an explicit support claim demonstrates the need.

### Security takes precedence over pre-1.0 compatibility

- **Decision:** make v0.2.0 a security-only release and accept documented breaking changes when they remove a weaker security contract.
- **Why:** Singleserve owns the authentication boundary for every consumer handler. An insecure compatibility option would preserve the exact footgun the release is intended to remove and would make the secure behavior dependent on each consumer noticing and selecting it.
- **Rejected:** add opt-in `connect({ tokenStorage: "memory" })` while retaining session storage by default; retain an explicit legacy session-storage mode indefinitely; mix unrelated post-v0.1 features into the security release.

### No browser-readable credential persistence

- **Decision:** v0.2.0 must not write a Singleserve authentication capability to `localStorage`, `sessionStorage`, IndexedDB, Cache Storage, service workers, or another script-readable persistence mechanism. The browser client must not return the capability as public session state.
- **Why:** Web Storage is readable by every script executing in the origin and survives the document closure that would otherwise bound an in-memory value. The existing HttpOnly cookie can support reload only after its cross-port isolation problem is resolved.
- **Rejected:** treat `sessionStorage` as sufficiently secure because it is tab-scoped or shorter-lived than `localStorage`.

### Server-owned clean bootstrap

- **Decision:** the security properties for v0.2.0 require Singleserve to consume capability material and navigate to a clean application URL before consumer HTML or JavaScript runs.
- **Why:** application-JavaScript scrubbing leaves capability lifetime dependent on script order, permits earlier same-origin scripts to read the URL, and allows consumer behavior to weaken referrer or cache protection.
- **Implementation gate:** the v0.2 specification must settle one-time/replay semantics, manual recovery, distinct credential roles, and the final programmatic-authentication contract before runtime implementation.

### Per-launch browser-origin isolation

- **Decision:** a random TCP port is not an acceptable cookie-isolation boundary. v0.2.0 must use a tested per-launch host/origin and exact Host validation so sibling loopback services cannot receive or overwrite a browser session cookie.
- **Why:** RFC 6265 explicitly states that cookies do not provide isolation by port. Launch-unique cookie names avoid collisions but do not prevent disclosure to another service on the same host.
- **Implementation gate:** prefer a high-entropy `.localhost` host if RFC 6761 behavior is interoperable across supported platforms and browsers; otherwise choose a tested distinct loopback-address design. If no design satisfies the isolation contract, do not release v0.2.0.

### Mandatory browser and security release evidence

- **Decision:** real-browser authentication and sibling-loopback isolation tests, vulnerability scanning, static security analysis, and CI supply-chain review are v0.2.0 release requirements.
- **Why:** Node mocks cannot prove cookie, redirect, referrer, history, or Web Storage behavior in an actual browser. Security claims require evidence at the boundary where they apply.
- **Rejected:** carry the v0.1.0 optional-browser skip into v0.2.0 or waive a security check because its infrastructure is unavailable.

### Fragment bootstrap, owner-controlled renewal, and private programmatic client

- **Decision:** put the independent two-minute bootstrap capability in the URL fragment, exchange it from a fixed Singleserve-owned page, consume it atomically, and provide `Launch.NewBootstrapURL()` so trusted owner code can rotate the single active capability and reset its full two-minute window without changing server lifetime or browser-session state. Expose programmatic access through a launch-bound `http.Client` whose credential is not returned to callers.
- **Why:** fragments do not enter HTTP request targets or referrers. The fixed page can scrub the fragment before any consumer code runs. Explicit rotation preserves a short exposure window while allowing a consumer to recover immediately from a browser-opener failure. A launch-bound client can enforce exact origin and bypass environment proxies while keeping the programmatic secret private.
- **Rejected:** query-token redirects that still place the capability in HTTP request targets; an indefinitely valid or lifetime-configurable initial bootstrap; automatic renewal; a public `Launch.Token()`; continued bearer authentication; asking callers to parse `Launch.URL()`.

### High-entropy localhost origin

- **Decision:** use a 128-bit high-entropy host below `.localhost`, validate the exact Host on every request, and restrict configured bind hosts to `127.0.0.1`, `::1`, or `localhost`.
- **Why:** RFC 6761 reserves `.localhost` and its subdomains for loopback resolution. A host-only cookie on an unguessable launch host is not sent to ordinary sibling services addressed as `127.0.0.1`, `localhost`, or another launch host.
- **Boundary:** knowledge of the random hostname is capability-adjacent. The full bootstrap URL remains secret until exchange; the design does not claim protection against process-memory inspection or an actor that already obtained that URL.

### Loopback HTTP cookie

- **Decision:** keep v0.2.0 on loopback HTTP but require a host-only, HttpOnly, Secure, SameSite=Strict, non-persistent cookie with a `__Host-` name prefix.
- **Why:** supported modern browsers treat `.localhost` as a potentially trustworthy local origin. The prefix makes the browser enforce `Secure`, `Path=/`, and absence of `Domain`, preventing a related `.localhost` service from injecting the same cookie name at the parent domain. High-entropy host and cookie names remain defense in depth.
- **Release consequence:** real-browser acceptance of the Secure cookie and rejection of a `Domain=localhost` injection are mandatory. If a supported engine cannot satisfy both, v0.2.0 is blocked; removing `Secure` or the prefix is not the compatibility fallback. Local TLS and certificate management remain outside this release.

## Unresolved after v0.1

- Whether a later release should publish the browser client to npm with TypeScript declarations.
- Whether a later release should expose tab event observers rather than snapshots only.
- Whether configurable control prefixes are justified by a real route collision.
- Whether minimum-Go support should expand below the declared Go 1.26.5 baseline.
- Whether GitHub tag/release-note automation is worthwhile after the first manual release.

None of these questions should delay v0.1 implementation or add speculative API now.

They remain deferred during the security-only v0.2.0 work unless the active security ExecPlan explicitly promotes one as necessary to close a security gap.
