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

if ! command -v csilgen >/dev/null; then
    echo "ERROR: csilgen not on PATH" >&2
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

# ---- TypeScript types (client) ----
# We only need the type declarations here; the SPA's LiveRepo is hand-written
# against the existing REST endpoints today (the full RPC client comes later,
# after the api grows CSIL-RPC endpoints).
mkdir -p "$TS_OUT"
csilgen generate --input "$SPEC" --target typescript-typesonly --output "$TS_OUT"

echo "Regenerated:"
echo "  Go: $(ls "$GO_OUT"/*.gen.go | wc -l) file(s) in $GO_OUT"
echo "  TS: $(ls "$TS_OUT"/*.ts 2>/dev/null | wc -l) file(s) in $TS_OUT"
