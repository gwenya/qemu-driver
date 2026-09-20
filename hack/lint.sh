#!/usr/bin/env bash
# lint and check formatting, warning first if the golangci-lint on this
# machine has drifted from the version ci pins in .golangci-version
set -euo pipefail

cd "$(dirname "$0")/.."

want="$(cat .golangci-version)"
installed="$(golangci-lint version --short 2>/dev/null || echo none)"

if [ "$installed" != "$want" ]; then
    echo "warning: golangci-lint $installed installed, ci pins $want" >&2
fi

golangci-lint run ./...
