# Building consumer applications

This guide describes the expected integration shape for a Go backend and
browser frontend built with Singleserve v0.2.x. It is written for both human
implementers and AI-assisted coding agents that need a concrete checklist.

Singleserve supplies the local process boundary: loopback hosting,
per-launch browser authentication, health, heartbeat, tab presence, guarded
shutdown, lifetime policy, and graceful drain. The application still owns its
router, API routes, business logic, frontend framework, UI copy, persistence,
and domain cleanup.

Use `examples/minimal` as the executable reference. It demonstrates the
lifecycle behavior that a real app should adapt into its own UI.

## Required integration checklist

A generated or hand-written Singleserve app should include all of these pieces
unless the product brief explicitly chooses a different lifetime or user flow:

- Create the application `http.Handler` first, then pass it to
  `singleserve.New` through `Options.Handler`.
- Choose `singleserve.BrowserBoundLifetime()` when the Go process should stop
  after the last browser tab closes. Use `singleserve.ExplicitLifetime()` only
  when an owner process, parent context, or CLI command should control exit.
- Add a `ShutdownGuardFunc` when unsafe work can be in progress. Return stable,
  user-safe denial codes and messages with `singleserve.DenyShutdown`.
- Call `Start`, optionally call `OpenBrowser`, provide a manual URL fallback
  when browser opening fails, then wait on `Launch.Wait`.
- In browser code, import `connect` from `/_singleserve/client.js`.
- Call `connect` once for the page or app shell and keep the returned session in
  the frontend state boundary.
- Provide `onHeartbeat` and `onServerUnavailable` handlers so the UI can show
  backend status, heartbeat failures, and backend-loss recovery.
- Use `session.fetch` for application API requests. Do not use raw `fetch` for
  same-origin backend calls that require Singleserve browser authentication.
- Provide a visible backend status or health affordance. A manual health button
  can call `session.health()`, while background status can come from
  `onHeartbeat`.
- Provide a quit or shutdown button wired to `session.requestShutdown()`.
- Handle shutdown denial by keeping the app open and showing the guard's
  user-safe message.
- After accepted shutdown or backend loss, enter a terminal UI state, call
  `session.stop()`, attempt `window.close()`, and show clear manual close
  instructions because many browsers block script-initiated tab closure.
- Exercise initial launch, reload, a clean new tab, two-tab close behavior,
  guarded shutdown denial and acceptance, backend loss, and manual URL fallback
  before shipping a consumer app.

Do not implement replacement routes under `/_singleserve/`. That path is
reserved for Singleserve's health, heartbeat, disconnect, shutdown, bootstrap,
and browser client endpoints.

## Go backend pattern

Keep startup explicit so browser-opening policy, lifetime policy, and process
drain remain understandable:

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

Use the `result` from `Wait` to run application-owned cleanup and choose the
owner-facing exit message exactly once. Parent-context cancellation and
programmatic owner shutdown bypass guards; browser and user-requested shutdown
evaluate them.

Programmatic backend code should use `launch.Client()` for authenticated health,
control, or application requests. Do not parse `Launch.URL()` or extract
credentials.

## Browser frontend pattern

The browser client owns scheduling for heartbeat and disconnect. Presentation
code should observe it and render state, not duplicate the lifecycle protocol:

```html
<script type="module">
  import { connect } from "/_singleserve/client.js";

  let session;
  let stopped = false;

  function enterTerminal(heading, detail) {
    if (stopped) return;
    stopped = true;
    session?.stop();
    showStoppedState(heading, detail);
    window.close();
  }

  session = await connect({
    onHeartbeat(result) {
      showBackendStatus({
        healthy: result.ok,
        failures: result.failures,
        checkedAt: result.checkedAt,
      });
    },
    onServerUnavailable({ failures }) {
      enterTerminal("Backend connection lost", `${failures} consecutive heartbeat failures.`);
    },
  });

  document.querySelector("#health").addEventListener("click", async () => {
    showHealth(await session.health());
  });

  document.querySelector("#quit").addEventListener("click", async () => {
    try {
      await session.requestShutdown();
      enterTerminal("Backend shutdown accepted", "The local server is draining.");
    } catch (error) {
      showShutdownDenied(error.message);
    }
  });
</script>
```

In the terminal stopped state, keep enough UI visible for the user to understand
what happened and manually close the tab if `window.close()` is blocked.

## Prompt checklist for AI-assisted builds

When asking an LLM or coding agent to build a Singleserve app, include a prompt
block like this:

```text
Build a local Go backend with a browser frontend using
github.com/rztaylor/singleserve v0.2.x.

Required Singleserve integration:
- Use singleserve.New with the app http.Handler.
- Use BrowserBoundLifetime unless explicitly told the backend must outlive the browser.
- Start the server, open the browser, provide a manual URL fallback, then wait for shutdown.
- In browser code, import connect from /_singleserve/client.js.
- Use connect with onHeartbeat and onServerUnavailable.
- Use session.fetch for same-origin app API calls.
- Provide visible backend status, a health check, backend-lost UI, and a quit/shutdown button.
- Wire quit to session.requestShutdown and handle shutdown denial.
- On accepted shutdown or backend loss, enter a terminal state, stop the session, attempt window.close, and show manual close instructions.
- Do not parse Launch.URL, expose tokens, use localStorage/sessionStorage for Singleserve auth, send X-Singleserve-Token from browser JavaScript, or implement replacement /_singleserve/ control endpoints.
```

For production-quality work, also ask the agent to verify the consumer app in a
real browser with initial launch, reload, new tab, last-tab close, guard-denied
shutdown, accepted shutdown, and backend-loss flows.

## Security boundaries for consumer apps

Do not move Singleserve credentials into application state. In v0.2.x the
browser session is established by a fixed Singleserve bootstrap page before
consumer content runs, then maintained by a host-isolated `HttpOnly`, `Secure`,
`SameSite=Strict`, non-persistent `__Host-` cookie. Application JavaScript
should not parse launch URLs, persist credentials, set `X-Singleserve-Token`, or
try to recreate the bootstrap flow.

Do not expose the manual bootstrap URL through untrusted UI, logs, telemetry, or
browser storage. Treat it as a short-lived secret until bootstrap succeeds.

## What remains application-owned

Singleserve does not choose the app's frontend framework, component library,
copy, domain model, persistence, recovery files, or work-safety rules. Those
must be implemented at the consumer boundary. The required lifecycle UI can be
styled and placed however the app needs, but the behavior should remain present:
observable backend status, authenticated API calls, guarded quit, shutdown
denial handling, backend-loss handling, and a terminal close-this-tab state.
