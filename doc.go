// Package singleserve provides the local HTTP, browser authentication,
// browser-presence, and graceful-shutdown boundary for single-binary browser
// applications while leaving routing, assets, and business logic to callers.
//
// A Server wraps an application-owned [net/http.Handler], binds only to
// loopback, and produces a per-launch capability URL. Applications choose an
// explicit or browser-bound lifetime and may supply a shutdown guard.
package singleserve
