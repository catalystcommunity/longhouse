#!/usr/bin/env bash
# Build the webapp module and run its tests. Webapp has no DB dependency
# (handler tests use a fake api.Client), so we don't need postgres.
#
# Run-local parity: ${REACTORCIDE_REPOROOT:-/job/src} resolves to runnerlib's
# clone in CI and to the bind-mounted working tree under
# `reactorcide run-local`. No git ops, no gh ops — Bucket B per
# reactorcide/linkkeys-runlocal-migration.md.
set -euo pipefail

echo "================================================"
echo "Longhouse Webapp Test"
echo "================================================"

cd "${REACTORCIDE_REPOROOT:-/job/src}"
export HOME="${HOME:-/home/runner}"

# `sudo -E` is rejected by the runner's sudoers (no preserve-env), so pass
# nothing through env. apt-get -y is non-interactive enough on its own.
echo "=== Installing system packages ==="
sudo apt-get update -qq
sudo apt-get install -y --no-install-recommends build-essential curl ca-certificates

echo "=== Installing Go toolchain ==="
GO_VERSION="${GO_VERSION:-1.23.8}"
if ! command -v go >/dev/null || ! go version 2>/dev/null | grep -q "go${GO_VERSION}"; then
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o /tmp/go.tar.gz
    mkdir -p "$HOME/.local"
    rm -rf "$HOME/.local/go"
    tar -C "$HOME/.local" -xzf /tmp/go.tar.gz
    rm /tmp/go.tar.gz
fi
export PATH="$HOME/.local/go/bin:$HOME/go/bin:${PATH}"
go version

echo "=== Building webapp ==="
( cd webapp && go build ./... )

echo "=== Running webapp tests ==="
( cd webapp && go test ./... )

echo ""
echo "================================================"
echo "All webapp tests passed"
echo "================================================"
