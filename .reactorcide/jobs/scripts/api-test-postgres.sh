#!/usr/bin/env bash
# Run the api module's tests against a real PostgreSQL spawned inline in
# this job. Mirrors linkkeys/.reactorcide/jobs/test-postgres.yaml: install
# postgres + Go via apt, initdb + pg_ctl as the runner user, point the
# integration tests at it via LONGHOUSE_TEST_DB_URI.
#
# Run-local parity: this script reads its source from
# ${REACTORCIDE_REPOROOT:-/job/src}. In CI, runnerlib clones into /job/src;
# in `reactorcide run-local`, the bind mount populates the same path. No
# git clone / push / gh-release here, so no REACTORCIDE_WORKER_MODE branch
# is needed (Bucket B per reactorcide/linkkeys-runlocal-migration.md).
set -euo pipefail

echo "================================================"
echo "Longhouse API Postgres Test"
echo "================================================"

cd "${REACTORCIDE_REPOROOT:-/job/src}"
export HOME="${HOME:-/home/runner}"

# `sudo -E` is rejected by the runner's sudoers (no preserve-env), so pass
# nothing through env. apt-get -y is non-interactive enough on its own.
echo "=== Installing system packages ==="
sudo apt-get update -qq
sudo apt-get install -y --no-install-recommends postgresql postgresql-client build-essential curl ca-certificates

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

# postgres binaries live at /usr/lib/postgresql/<version>/bin on Debian.
PG_BIN="$(find /usr/lib/postgresql -maxdepth 2 -name bin -type d | head -1)"
export PATH="${PG_BIN}:${PATH}"

RUN_USER="$(id -un)"
PGDATA="/tmp/pgdata"
PGLOG="/tmp/pg.log"

echo "=== Initializing and starting PostgreSQL (user=${RUN_USER}) ==="
initdb -D "${PGDATA}" --auth=trust --username="${RUN_USER}" --encoding=UTF8 >/dev/null
pg_ctl -D "${PGDATA}" -l "${PGLOG}" -o "-k /tmp -h 127.0.0.1 -p 5432" -w start
createdb -h /tmp -U "${RUN_USER}" longhouse_test

cleanup() {
    pg_ctl -D "${PGDATA}" stop -m fast >/dev/null 2>&1 || true
}
trap cleanup EXIT

export LONGHOUSE_TEST_DB_URI="postgres://${RUN_USER}@127.0.0.1:5432/longhouse_test?sslmode=disable"

echo "=== Running api unit tests ==="
( cd api && go test ./... )

echo "=== Running api integration tests (real Postgres) ==="
( cd api && go test -tags=integration ./... )

echo ""
echo "================================================"
echo "All api tests passed"
echo "================================================"
