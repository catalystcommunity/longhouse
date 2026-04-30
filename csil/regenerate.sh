#!/usr/bin/env bash
# Regenerate Go types/services from longhouse.csil. Run from any directory.
# Output goes under api/internal/csil/. The csilgen Go target hard-codes
# `package api` regardless of the CSIL options.package value, so we rewrite
# the package line on the way out — keeps the import path's last segment
# matching the package name.
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SPEC="$REPO/csil/longhouse.csil"
OUT="$REPO/api/internal/csil"

if ! command -v csilgen >/dev/null; then
    echo "ERROR: csilgen not on PATH" >&2
    exit 1
fi

mkdir -p "$OUT"
csilgen validate --input "$SPEC"
csilgen generate --input "$SPEC" --target go --output "$OUT"

# Rename package so it matches the directory.
for f in "$OUT"/*.gen.go; do
    sed -i 's/^package api$/package csil/' "$f"
done

# gofmt the result for stable diffs.
if command -v gofmt >/dev/null; then
    gofmt -w "$OUT"
fi

echo "Regenerated $(ls "$OUT" | wc -l) file(s) in $OUT"
