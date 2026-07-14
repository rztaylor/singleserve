# Singleserve

Singleserve is a lightweight Go toolkit for single-binary, browser-based local applications. It provides the local process boundary—loopback HTTP hosting, per-launch authentication, browser startup, browser-presence tracking, lifetime policy, shutdown guards, and graceful drain—while applications keep ownership of their router, business logic, frontend framework, and embedded assets.

> [!IMPORTANT]
> `v0.1.0` is the first supported release. See the [v0.1 API specification](docs/dev/specs/v0.1-api.md), [release governance](docs/dev/ops/release-governance.md), and [roadmap](docs/dev/roadmap.md).

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
go get github.com/rztaylor/singleserve@v0.1.0
```

## Runnable example

Run the executable specification from the repository root:

```sh
go run ./examples/minimal
```

It starts a browser-bound loopback server and opens a live lifecycle dashboard. The page shows each heartbeat and its timestamp, the next-heartbeat countdown, consecutive failures, a manual health probe, backend-loss detection timing, and the normal/fallback tab-close shutdown windows. Shutting down or losing the backend moves the page into a terminal state and attempts to close the tab; if the browser blocks script closure, the page clearly asks the user to close it. If browser opening fails, the command prints the authenticated URL for manual opening. `Ctrl+C` remains an owner-forced shutdown path.

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
    fmt.Fprintln(os.Stderr, "Open this URL manually:", launch.URL())
}
result, err := launch.Wait()
```

`Start` binds before returning. `OpenBrowser` is optional; if it fails, the listener remains available and `launch.URL()` can be shown for manual opening. `Wait` returns only after the server has drained or reports a serving/drain error.

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

The initial capability URL establishes a launch-unique strict session cookie for ordinary asset loads. The client retains the per-tab token in `sessionStorage`, removes it from the address bar, authenticates same-origin requests, sends heartbeats, and reports server loss without rendering UI.

## Design boundaries

- Applications pass an `http.Handler`; Singleserve wraps it but never constructs or mutates the application router.
- The reserved `/_singleserve/` endpoints are the only HTTP routes owned by the toolkit.
- Authentication, browser state, and lifecycle state are process-local and never persisted.
- The framework-neutral browser client is a plain ES module with no npm or UI-framework dependency.
- The initial implementation is standard-library-only.
- Non-loopback listeners, TLS, remote access, multi-user identity, WebSockets, and application persistence are outside v0.1.

## Repository map

- [Architecture](docs/dev/architecture.md)
- [v0.1 consumer requirements](docs/dev/specs/consumer-requirements.md)
- [v0.1 API specification and acceptance criteria](docs/dev/specs/v0.1-api.md)
- [Migration strategy](docs/dev/guides/migration.md)
- [Roadmap](docs/dev/roadmap.md)
- [Decisions and unresolved choices](docs/dev/decisions.md)
- [Release governance](docs/dev/ops/release-governance.md)
- [Development workflow](docs/dev/guides/development.md)

## Licence

Singleserve uses the [MIT License](LICENSE). The choice is intended to keep adoption simple for open-source and proprietary local applications while retaining the standard warranty disclaimer.
