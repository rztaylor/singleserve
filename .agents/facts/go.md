# Go Facts

- Module path: `github.com/rztaylor/singleserve`.
- Minimum Go baseline: Go 1.26.5, declared in `go.mod`; security releases raise the patch floor when the Go vulnerability database identifies an applicable toolchain issue.
- The module root is the public `singleserve` package. No project binary is distributed; `examples/minimal` is the executable public-API specification.
- Every Go package must have an accurate package comment and clear ownership boundary.
- Prefer `http.Handler`, small consumer-boundary interfaces, typed errors only for caller control flow, and immutable/copying snapshot APIs.
- Avoid package-level mutable state.
- Run `gofmt` on changed Go files.
- Use repo-local caches in managed sandboxes: `GOCACHE="$PWD/.cache/go-build"` and `GOMODCACHE="$PWD/.cache/go-mod"`.
- No third-party runtime dependency is approved. Adding one requires a spec and decision update.
