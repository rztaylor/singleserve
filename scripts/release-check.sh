#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"

export GOCACHE="$repo_root/.cache/go-build"
export GOMODCACHE="$repo_root/.cache/go-mod"
mkdir -p "$GOCACHE" "$GOMODCACHE"

if [ "$#" -ne 1 ]; then
    echo "usage: scripts/release-check.sh vMAJOR.MINOR.PATCH[-PRERELEASE]" >&2
    exit 2
fi

version=$1
if ! printf '%s\n' "$version" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$'; then
    echo "invalid release version: $version" >&2
    exit 2
fi

if [ -n "$(git status --porcelain)" ]; then
    echo "release checks require a clean worktree" >&2
    exit 1
fi

if git rev-parse -q --verify "refs/tags/$version" >/dev/null; then
    echo "release tag already exists locally: $version" >&2
    exit 1
fi

scripts/check.sh

case "$version" in
    v0.2.*)
        scripts/check-security.sh
        scripts/check-browser-security.sh
        ;;
esac

changelog_version=${version#v}
if ! grep -Fq "## [$changelog_version]" CHANGELOG.md; then
    echo "CHANGELOG.md has no release heading for $version" >&2
    exit 1
fi

secret_files=$(git grep -IlE '(-----BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY-----|gh[pousr]_[A-Za-z0-9_]{20,}|AKIA[0-9A-Z]{16}|xox[baprs]-[A-Za-z0-9-]{10,})' -- . || true)
if [ -n "$secret_files" ]; then
    echo "possible secret material found in:" >&2
    echo "$secret_files" >&2
    exit 1
fi

go list ./... >/dev/null

echo "release policy checks: ok for $version"
echo "no tag or publication was performed"
