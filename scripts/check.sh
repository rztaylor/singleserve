#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"

export GOCACHE="$repo_root/.cache/go-build"
export GOMODCACHE="$repo_root/.cache/go-mod"
mkdir -p "$GOCACHE" "$GOMODCACHE"

scripts/check-docs.sh

unformatted=$(find . -path './.cache' -prune -o -type f -name '*.go' -exec gofmt -l {} +)
if [ -n "$unformatted" ]; then
    echo "gofmt required for:" >&2
    echo "$unformatted" >&2
    exit 1
fi

go mod tidy -diff
node --check examples/minimal/static/app.js
node --check client/playwright-driver.mjs
go build -o "$repo_root/.cache/minimal-demo" ./examples/minimal
go test -run '^TestSmoke$' -count=1 ./examples/minimal
go test ./...
go test -race ./...
coverage_file="$repo_root/.cache/coverage.out"
go test -coverprofile="$coverage_file" ./...
coverage=$(go tool cover -func="$coverage_file" | awk '/^total:/ { gsub("%", "", $3); print $3 }')
awk -v coverage="$coverage" 'BEGIN { if (coverage + 0 < 80) exit 1 }' || {
    echo "Go statement coverage is ${coverage}%; require at least 80%" >&2
    exit 1
}
go vet ./...
node --test client/*.test.mjs
git diff --check

echo "repository checks: ok (${coverage}% Go statement coverage)"
