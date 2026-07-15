# Consumer requirements

## Evidence policy

Singleserve admits only requirements that are generic to its lifecycle boundary and supported by implementation or tests. It does not import consumer application packages or copy an application's package architecture.

## Demonstrated common requirements

| Requirement | Demonstrated need | v0.1 conclusion |
| --- | --- | --- |
| Loopback hosting | Local browser applications need random-port HTTP hosting without exposing the server to a network | Reject all non-loopback binds |
| Random launch port | Applications need collision-free concurrent launches | Default to `127.0.0.1:0` |
| Launch token | Browser assets and API calls need one per-launch authentication contract | Use one generic token contract with a launch-unique cookie |
| Browser startup | The listener must be ready before a platform browser command runs | Provide an injectable, cross-platform opener |
| Health and browser presence | Frontends need authenticated liveness checks and the backend needs per-tab presence | Separate authenticated health and heartbeat control endpoints |
| Browser-bound shutdown | Some applications should stop after browser absence while others remain owner-controlled | Make lifetime an explicit policy, independent of browser opening |
| Graceful stop | In-flight application requests need a bounded drain period | Make graceful stop a core lifecycle invariant |
| Shutdown veto | Applications may have work that makes browser-driven shutdown unsafe | Provide a typed application-owned `ShutdownGuard` |
| Framework-neutral integration | Consumers use different frontend stacks and API layers | Ship a plain ES module below framework adapters |

## Responsibilities that remain consumer-owned

- Domain-specific rules deciding whether shutdown is safe.
- Recovery files, locks, transactions, jobs, and other application cleanup.
- Parent/child process orchestration and startup handshakes.
- Application routes, error payloads, user messages, and recovery experiences.
- Frontend state management and framework integration.

Singleserve therefore exposes generic shutdown requests, typed denials, lifecycle results, and tab snapshots rather than domain-specific callbacks.

## Deliberate toolkit boundaries

v0.1 identifies and expires tabs separately, then bases browser-bound shutdown on the aggregate connected-tab set.

Browser opening and browser-bound lifetime are separate decisions. Opening a page does not implicitly grant that page ownership of the process lifetime.

The launch token is retained only for the browser session through per-tab `sessionStorage` and a launch-unique session cookie, avoiding intentional persistence beyond the session and collisions across simultaneous local servers.

## Demonstrated post-v0.1 security requirement

Evaluation against a real consumer demonstrated that retaining an authentication capability in either `localStorage` or `sessionStorage` is not an acceptable generic browser-security contract. The v0.1.0 cookie-name design prevents simultaneous Singleserve processes from authenticating with the wrong cookie, but a random port does not prevent a sibling loopback service on the same host from receiving that cookie.

The unreleased v0.2.0 contract therefore:

- keeps Singleserve browser credentials out of script-readable persistence and public JavaScript state;
- consumes and scrubs launch capability material before consumer content runs;
- authenticates reloads through an HttpOnly session isolated to one high-entropy launch host;
- validates effective Host, browser Origin, and authentication mode before consumer delegation; and
- preserves the existing generic lifecycle client outcomes without a weaker compatibility mode.

This is implemented but unreleased behavior, not the contract of the supported v0.1.0 tag. See `docs/dev/specs/v0.2-api.md`, the active roadmap item, and the ExecPlan.
