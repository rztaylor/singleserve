#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"

export GOCACHE="$repo_root/.cache/go-build"
export GOMODCACHE="$repo_root/.cache/go-mod"
export SINGLESERVE_BROWSER_SECURITY=1
mkdir -p "$GOCACHE" "$GOMODCACHE"

case "${SINGLESERVE_BROWSER_BACKEND:-webdriver}" in
    playwright)
        if ! command -v node >/dev/null 2>&1; then
            echo "Playwright browser security checks require Node.js" >&2
            exit 1
        fi
        if [ ! -f "$repo_root/client/node_modules/playwright/package.json" ]; then
            echo "Playwright browser security checks require 'npm ci --prefix client --ignore-scripts'" >&2
            exit 1
        fi
        SINGLESERVE_PLAYWRIGHT_DRIVER="$repo_root/client/playwright-driver.mjs"
        export SINGLESERVE_PLAYWRIGHT_DRIVER
        ;;
    webdriver)
        if [ -z "${SINGLESERVE_WEBDRIVER:-}" ]; then
            for candidate in chromedriver geckodriver safaridriver; do
                if command -v "$candidate" >/dev/null 2>&1; then
                    SINGLESERVE_WEBDRIVER=$(command -v "$candidate")
                    export SINGLESERVE_WEBDRIVER
                    break
                fi
            done
        fi

        if [ -z "${SINGLESERVE_WEBDRIVER:-}" ]; then
            echo "browser security checks require Playwright or chromedriver, geckodriver, or safaridriver" >&2
            exit 1
        fi
        ;;
    *)
        echo "unsupported browser security backend: ${SINGLESERVE_BROWSER_BACKEND}" >&2
        exit 2
        ;;
esac

go test -tags=browsersecurity -run '^TestRealBrowserSecurity$' ./
