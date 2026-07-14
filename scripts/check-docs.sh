#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"

required_files="
AGENTS.md
README.md
LICENSE
CHANGELOG.md
CONTRIBUTING.md
SECURITY.md
.agents/facts/product.md
.agents/facts/architecture.md
.agents/facts/engineering.md
.agents/facts/go.md
.agents/facts/testing.md
.agents/facts/find-tech-debt.md
.agents/facts/docs.md
.agents/facts/roadmap.md
.agents/facts/git.md
.agents/facts/release.md
docs/dev/architecture.md
docs/dev/decisions.md
docs/dev/roadmap.md
docs/dev/specs/consumer-requirements.md
docs/dev/specs/v0.1-api.md
docs/dev/guides/development.md
docs/dev/guides/migration.md
docs/dev/ops/ci.md
docs/dev/ops/release-governance.md
client/package.json
client/singleserve.js
client/singleserve.test.mjs
examples/minimal/main.go
examples/minimal/main_test.go
examples/minimal/static/index.html
examples/minimal/static/styles.css
examples/minimal/static/app.js
compatibility_browserbound_test.go
compatibility_explicit_test.go
"

for path in $required_files; do
    if [ ! -s "$path" ]; then
        echo "missing or empty required file: $path" >&2
        exit 1
    fi
done

grep -q 'module github.com/rztaylor/singleserve' go.mod
grep -q 'Status: \*\*Released in v0.1.0\*\*' docs/dev/specs/v0.1-api.md
grep -q 'There are no active roadmap items.' docs/dev/roadmap.md
grep -q 'MIT License' LICENSE

if grep -rEn '/Users/|/home/|C:\\Users\\' -- AGENTS.md README.md CONTRIBUTING.md SECURITY.md CHANGELOG.md .agents/facts docs; then
    echo "documentation contains a machine-local absolute path" >&2
    exit 1
fi

echo "documentation foundation: ok"
