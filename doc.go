// Package singleserve provides the local HTTP, browser authentication,
// browser-presence, and graceful-shutdown boundary for single-binary browser
// applications while leaving routing, assets, and business logic to callers.
//
// A Server wraps an application-owned [net/http.Handler], binds only to
// loopback, and produces a short-lived browser bootstrap URL on an isolated
// per-launch origin. Trusted owner code can rotate that URL with
// [Launch.NewBootstrapURL]. Browser authentication becomes an HttpOnly session
// before application content runs; programmatic callers use [Launch.Client].
// Applications choose an explicit or browser-bound lifetime and may supply a
// shutdown guard.
package singleserve
