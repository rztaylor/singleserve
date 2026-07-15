#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"

export GOCACHE="$repo_root/.cache/go-build"
export GOPATH="$repo_root/.cache/go"
export GOMODCACHE="$repo_root/.cache/go-mod"
mkdir -p "$GOCACHE" "$GOPATH" "$GOMODCACHE"

go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 -version
go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...

unpinned_actions=$(grep -RhoE 'uses: [^[:space:]]+' .github/workflows | sed 's/^uses: //' | grep -Ev '@[0-9a-f]{40}$' || true)
if [ -n "$unpinned_actions" ]; then
    echo "GitHub Actions must use immutable commit SHAs:" >&2
    echo "$unpinned_actions" >&2
    exit 1
fi

secret_files=$(git grep -IlE '(-----BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY-----|gh[pousr]_[A-Za-z0-9_]{20,}|AKIA[0-9A-Z]{16}|xox[baprs]-[A-Za-z0-9-]{10,})' -- . || true)
if [ -n "$secret_files" ]; then
    echo "possible secret material found in:" >&2
    echo "$secret_files" >&2
    exit 1
fi

echo "security checks: ok"
