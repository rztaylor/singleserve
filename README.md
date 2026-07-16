# Singleserve

Singleserve is a lightweight Go toolkit for single-binary, browser-based local applications. It provides the local process boundary—loopback HTTP hosting, per-launch authentication, browser startup, browser-presence tracking, lifetime policy, shutdown guards, and graceful drain—while applications keep ownership of their router, business logic, frontend framework, and embedded assets.

> [!IMPORTANT]
> `v0.2.x` is the current supported release line. `v0.2.0` introduced the hardened browser-authentication contract; use `v0.2.1` or later for Windows-compatible programmatic clients. See the [v0.2 API specification](docs/dev/specs/v0.2-api.md), [v0.1-to-v0.2 migration guide](docs/dev/guides/migration.md), [release governance](docs/dev/ops/release-governance.md), and [roadmap](docs/dev/roadmap.md).

## Intended use

Singleserve targets local applications that:

- ship as one Go binary;
- serve a browser UI from a loopback-only HTTP listener;
- need a fresh authentication secret for every process launch;
- may live until explicitly stopped or until their browser tabs disconnect;
- need application-specific vetoes before a user- or browser-driven shutdown; and
- do not want a desktop webview, hosted account system, or prescribed frontend stack.

It is not a router, asset pipeline, desktop shell, remote-access server, application framework, or business-logic layer.

## Install

Add the released module to an application:

```sh
go get github.com/rztaylor/singleserve@v0.2.1
```

## Runnable example

Run the executable specification from the repository root:

```sh
go run ./examples/minimal
```

It starts a browser-bound loopback server and opens a live lifecycle dashboard. The page shows each heartbeat and its timestamp, the next-heartbeat countdown, consecutive failures, a manual health probe, backend-loss detection timing, and the normal/fallback tab-close shutdown windows. Shutting down or losing the backend moves the page into a terminal state and attempts to close the tab; if the browser blocks script closure, the page clearly asks the user to close it. Normal output identifies only the non-secret listener address. If browser opening fails, the command prints the authenticated URL once for manual opening. `Ctrl+C` remains an owner-forced shutdown path.

When building a new application, treat `examples/minimal` as the canonical consumer integration pattern, not just a demo. The [consumer app guide](docs/dev/guides/building-consumer-apps.md) includes the required frontend/backend lifecycle checklist and a prompt block for AI-assisted builds.

## Usage

The public Go API lives in the root package:

```go
import "github.com/rztaylor/singleserve"
```

The startup flow is deliberately explicit:

```go
srv, err := singleserve.New(singleserve.Options{
    Handler:  appRouter,
    Lifetime: singleserve.BrowserBoundLifetime(),
    Guard: singleserve.ShutdownGuardFunc(func(ctx context.Context, request singleserve.ShutdownRequest) error {
        if workIsRunning() {
            return singleserve.DenyShutdown("work_running", "Wait for the current operation to finish")
        }
        return nil
    }),
})
if err != nil {
    return err
}

launch, err := srv.Start(ctx)
if err != nil {
    return err
}
if err := launch.OpenBrowser(ctx); err != nil {
    manualURL, renewErr := launch.NewBootstrapURL()
    if renewErr != nil {
        return renewErr
    }
    fmt.Fprintln(os.Stderr, "Open this URL within two minutes:", manualURL)
}
result, err := launch.Wait()
```

`Start` binds before returning. `OpenBrowser` is optional. If it fails, `NewBootstrapURL` invalidates the earlier unconsumed capability and returns a fresh two-minute, one-time manual URL. Renewal does not extend or change the server's lifetime policy, and an already established browser session remains valid. `Wait` returns only after the server has drained or reports a serving/drain error.

The application can import the dependency-free browser client directly from the running server:

```html
<script type="module">
  import { connect } from "/_singleserve/client.js";

  const session = await connect({
    onServerUnavailable: () => showRecoveryUI(),
  });

  const response = await session.fetch("/api/items");
</script>
```

### Consumer app checklist

A Singleserve-backed Go/web application should wire the lifecycle details into its own UI:

- import `connect` from `/_singleserve/client.js` and keep the returned session in the app's frontend state boundary;
- use `session.fetch` for same-origin application API calls that need browser authentication;
- observe backend health with `onHeartbeat`, `onServerUnavailable`, and optionally `session.health()`;
- provide a user-facing quit or shutdown button that calls `session.requestShutdown()`;
- show shutdown-denial messages from the application `ShutdownGuardFunc`;
- enter a terminal stopped state after accepted shutdown or backend loss, call `session.stop()`, attempt `window.close()`, and tell the user to close the tab if the browser blocks it; and
- verify initial launch, reload, clean new tab, last-tab close, guarded shutdown, backend loss, and manual URL fallback in a real browser before shipping.

Do not parse `Launch.URL()`, expose Singleserve credentials, store lifecycle authentication in Web Storage, send `X-Singleserve-Token` from browser JavaScript, or implement replacement `/_singleserve/` control routes. See [Building consumer applications](docs/dev/guides/building-consumer-apps.md) for a copy-pasteable AI prompt checklist.

In v0.2, `launch.URL()` opens a fixed Singleserve bootstrap page on a high-entropy per-launch `.localhost` origin. Its two-minute, one-time capability is carried only in the URL fragment, scrubbed before the network exchange, and replaced with a clean application URL before consumer content runs. Owner code may call `launch.NewBootstrapURL()` when it needs to replace an unconsumed or expired manual URL; consumers remain responsible for deciding whether their own interface exposes that recovery action. Reloads and new tabs use only a host-isolated, `HttpOnly`, `Secure`, `SameSite=Strict`, `__Host-` session cookie. The lifecycle client never reads or writes Web Storage and exposes no authentication capability.

> **Security notice for v0.1.0:** its browser client stores the launch capability in script-readable `sessionStorage` and exposes it as `session.token`. That release is superseded by v0.2, which removes the weaker behavior rather than retaining a compatibility mode; see the [migration guide](docs/dev/guides/migration.md).

Programmatic callers in v0.2 use a launch-bound client instead of parsing a browser URL:

```go
response, err := launch.Client().Get(launch.BaseURL() + "api/items")
```

The client carries an independent private credential, bypasses environment proxies, refuses requests or redirects outside the exact launch origin, and dials the bound loopback listener directly so programmatic use does not depend on operating-system `.localhost` DNS behavior.

## Design boundaries

- Applications pass an `http.Handler`; Singleserve wraps it but never constructs or mutates the application router.
- The reserved `/_singleserve/` endpoints are the only HTTP routes owned by the toolkit.
- Authentication, browser state, and lifecycle state are process-local and never persisted.
- The framework-neutral browser client is a plain ES module with no npm or UI-framework dependency.
- The runtime implementation is standard-library-only.
- Non-loopback listeners, TLS, remote access, multi-user identity, WebSockets, and application persistence remain outside v0.2.

## Repository map

- [Architecture](docs/dev/architecture.md)
- [v0.1 consumer requirements](docs/dev/specs/consumer-requirements.md)
- [v0.1 API specification and acceptance criteria](docs/dev/specs/v0.1-api.md)
- [v0.2 security specification](docs/dev/specs/v0.2-api.md)
- [Building consumer applications](docs/dev/guides/building-consumer-apps.md)
- [Migration strategy](docs/dev/guides/migration.md)
- [Roadmap](docs/dev/roadmap.md)
- [Decisions and unresolved choices](docs/dev/decisions.md)
- [Release governance](docs/dev/ops/release-governance.md)
- [Development workflow](docs/dev/guides/development.md)

## Licence

Singleserve uses the [MIT License](LICENSE). The choice is intended to keep adoption simple for open-source and proprietary local applications while retaining the standard warranty disclaimer.
