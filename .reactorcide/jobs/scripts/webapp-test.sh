#!/usr/bin/env bash
# Type-check + unit-test the SolidJS SPA (webapp/). No Docker, no DB — just
# Node + the project's npm scripts. Mirrors the developer's local
# `npm run typecheck && npm test` flow so CI catches what they'd catch.
set -euo pipefail

echo "================================================"
echo "Longhouse Webapp (SPA) Test"
echo "================================================"

cd "${REACTORCIDE_REPOROOT:-/job/src}"
export HOME="${HOME:-/home/runner}"

# The runnerbase image may not have Node, or may have a different
# version. Install a pinned LTS under $HOME the same way the api job
# installs Go.
NODE_VERSION="${NODE_VERSION:-22.11.0}"
if ! command -v node >/dev/null || ! node --version 2>/dev/null | grep -q "v${NODE_VERSION%%.*}\."; then
    echo "=== Installing Node ${NODE_VERSION} ==="
    sudo apt-get update -qq
    sudo apt-get install -y --no-install-recommends curl xz-utils ca-certificates
    curl -fsSL "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-x64.tar.xz" -o /tmp/node.tar.xz
    mkdir -p "$HOME/.local"
    rm -rf "$HOME/.local/node"
    tar -xJf /tmp/node.tar.xz -C "$HOME/.local"
    mv "$HOME/.local/node-v${NODE_VERSION}-linux-x64" "$HOME/.local/node"
    rm /tmp/node.tar.xz
fi
export PATH="$HOME/.local/node/bin:${PATH}"
node --version
npm --version

echo "=== Installing webapp dependencies ==="
( cd webapp && npm ci )

echo "=== Typechecking ==="
( cd webapp && npm run typecheck )

echo "=== Running tests ==="
( cd webapp && npm test )

echo ""
echo "================================================"
echo "All webapp checks passed"
echo "================================================"
