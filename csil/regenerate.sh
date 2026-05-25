#!/usr/bin/env bash
# Regenerate api types/services (Go) AND client TypeScript types from
# longhouse.csil. Run from any directory.
#
#   Go types  → api/internal/csil/    (consumed by the api)
#   TS types  → client/src/api/        (consumed by the SPA via LiveRepo)
#
# The Go target emits `package api`; we rewrite it to `package csil` so the
# import path's last segment matches the package name.
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SPEC="$REPO/csil/longhouse.csil"
GO_OUT="$REPO/api/internal/csil"
TS_OUT="$REPO/client/src/api"

# csilgen is installed via `cargo install` and lives at ~/.cargo/bin/csilgen.
# That directory isn't always on the non-interactive PATH (e.g. shells spawned
# by editors/tools that don't source the user rc), so add it here rather than
# forcing every caller to fix their environment.
if [ -d "$HOME/.cargo/bin" ]; then
    export PATH="$HOME/.cargo/bin:$PATH"
fi

if ! command -v csilgen >/dev/null; then
    echo "ERROR: csilgen not on PATH (looked in \$PATH and ~/.cargo/bin)" >&2
    echo "       install with: cargo install --git https://github.com/catalystcommunity/csilgen" >&2
    exit 1
fi

csilgen validate --input "$SPEC"

# ---- Go (api) ----
mkdir -p "$GO_OUT"
csilgen generate --input "$SPEC" --target go --output "$GO_OUT"
for f in "$GO_OUT"/*.gen.go; do
    sed -i 's/^package api$/package csil/' "$f"
done
if command -v gofmt >/dev/null; then
    gofmt -w "$GO_OUT"
fi

# ---- TypeScript client (SPA) ----
# Emits both types.gen.ts (interfaces for every CSIL type) and client.gen.ts
# (one class per service with typed methods that call into a pluggable
# ServiceTransport). The SPA implements the transport once and gets typed
# RPC stubs for every endpoint. The transport currently shims to the
# existing REST routes; when the api grows CSIL-RPC endpoints, only the
# transport implementation has to swap.
mkdir -p "$TS_OUT"
csilgen generate --input "$SPEC" --target typescript-client --output "$TS_OUT"

echo "Regenerated:"
echo "  Go: $(ls "$GO_OUT"/*.gen.go | wc -l) file(s) in $GO_OUT"
echo "  TS: $(ls "$TS_OUT"/*.ts 2>/dev/null | wc -l) file(s) in $TS_OUT"
