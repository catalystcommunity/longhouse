# Longhouse

Coordination system for small-to-medium organizations and neighborhoods.
Tasks, calendar, projects, members, groups, and skills under a per-house
membership model.

## Architecture

- **api/**: Go API server. Single HTTP entry point at
  `POST /api/csil/{service}/{method}` with CBOR request/response. One
  extra non-RPC route: `GET /api/v1/auth/start` (302 redirect to linkkeys
  for the browser flow) and `GET /api/health` for k8s probes. No REST.
- **client/**: SolidJS + Vite SPA. Talks to the api through the generated
  typescript-client (`client/src/api/client.gen.ts`) using a CBOR
  transport (`client/src/transport/csilrpc.ts`).
- **csil/**: CSIL service definitions and types (`longhouse.csil`),
  source of truth for both the Go server stubs and the TypeScript client.
  Run `./csil/regenerate.sh` after any spec change.
- **coredb/**: Embedded SQL migrations module (goose).
- **helm_chart/**: Kubernetes Helm chart with Gateway API HTTPRoutes
  (api + web; the api also rides under `/api/*` on the web hostname so
  the SPA's relative fetches work).

## Auth

Uses linkkeys as external IDP. Local member records cache identity data.
Trusted domains and explicit user lists control access. Per-resource
sharing supports external READ access via linkkeys identity. The bearer
token snapshots per-house membership + roles at mint time so authz
needs no DB lookup per request; staleness is bounded by `exp` and the
`AuthService.Refresh` op.

## Dev setup

```bash
./tools dev        # postgres (docker, host port 5433) + api + vite, sourcing .env.dev
./tools dev-down   # tear down (keeps the postgres volume)
```

`tools dev` foregrounds a `tail -f` of `logs/api.log` + `logs/vite.log`;
Ctrl-C silences the tail without stopping anything. Vite proxies
`/api/*` → `localhost:6080`, so the SPA at `http://localhost:5173`
talks to the api directly.

`.env.dev` is gitignored — the file in the repo root is the local-only
config (postgres port, JWT secret, `LONGHOUSE_DEV_AUTH_ENABLED=true`,
initial admin identity). When the api boots with `LONGHOUSE_ENV=dev` and
dev-auth enabled, the SPA's `/dev-login` page lists members so you can
sign in without a live linkkeys RP.

Postgres is on **host port 5433** (not 5432) because 5432 is often
already in use on dev machines. The `LONGHOUSE_DB_URI` in `.env.dev`
already reflects that.

## Deploy / dogfood loop

The dogfood site lives at `https://longhouse.todandlorna.com`. Changes
are validated there before opening a PR.

`~/longhouse-redeploy.sh` (personal helper, not in repo) is the
iteration loop: build local images → push to
`containers.catalystsquad.com` under a `dev-<UTC>-<git-sha>[-dirty]`
tag → `helm upgrade` the `longhouse` namespace against the foundry
cluster (`~/.foundry/kubeconfig`).

One-time setup:

```bash
# Auth to the registry (token persists in ~/.docker/config.json):
REACTORCIDE_SECRETS_PASSWORD="$(cat ~/.reactorcide-pass)" \
  reactorcide secrets get catalystcommunity/registry password \
  | docker login containers.catalystsquad.com \
      -u "$(REACTORCIDE_SECRETS_PASSWORD="$(cat ~/.reactorcide-pass)" \
            reactorcide secrets get catalystcommunity/registry user)" \
      --password-stdin
```

Common flows:

```bash
~/longhouse-redeploy.sh                 # build + push + helm upgrade
~/longhouse-redeploy.sh --no-build      # config-only re-deploy (keeps the
                                        # currently-deployed image tag)
~/longhouse-redeploy.sh --tag X.Y.Z     # pin to an explicit released tag
~/longhouse-redeploy.sh --logs          # tail api logs at the end
```

CI (`.reactorcide/jobs/`) does the same thing on push-to-main bumps of
`version/VERSION.txt`; the local script just lets you iterate against
the dogfood namespace without going through a release.

## Browser-driving (optional, Claude Code only)

If you're using Claude Code, the `chrome-devtools` MCP server (Google's
official) is the easiest way to drive the SPA from prompts —
sign-in flows, screenshots, network traces. Install it at user scope so
it doesn't pollute the repo:

```bash
claude mcp add chrome-devtools -s user -- npx -y chrome-devtools-mcp@latest
```

Once installed it's available across all your projects.

## Environment Variables

All api env prefixed with `LONGHOUSE_`. See
`api/internal/config/config.go` for the full list, including the
dev-auth and recurrence-worker knobs.

## Conventions

- **CSIL-first.** The spec drives both server stubs and client. Don't
  hand-modify generated files; edit `csil/longhouse.csil` and rerun
  `./csil/regenerate.sh`.
- **CSIL-RPC wire format.** `application/cbor` in both directions.
  Server fields are snake_case (Go JSON tags); the SPA's transport
  auto-bridges camel↔snake so generated typescript-client classes see
  the camelCase types they were promised. Errors are CBOR-encoded
  `ServiceError { code, message }` with the matching HTTP status.
- **Authorization is resource-based**, not URL-based. Each CSIL method
  looks up the resource → reads its `house_id` → checks the bearer's
  identity has the required role in that house. There is no `/houses/{id}/...`
  URL convention.
- **Recurrence (tasks AND events) is server-spawned**, not client-
  expanded. The recurrence worker (60s tick) reads roots, creates real
  child rows up to a configurable horizon (2 years for events), and
  bumps `next_recurrence_at` forward. Deleting an instance hard-deletes
  that row; "delete this & future" also clears recurrence on the root.
- **Dependencies (tasks AND projects) are one-directional, stored once.**
  An edge flows from the dependent to the dependency it requires; either end
  is a task or project. The reverse ("what depends on X") is computed, never
  stored. Writes need `edit` on the dependent + `read` on the dependency;
  cycles are rejected in the DB (recursive CTE). The SPA edits only the
  forward direction. See `docs/dependencies.md`.
- **PostgreSQL only** (no SQLite). goose for migrations, GORM for ORM.
- **Multi-module Go project**: `api`, `coredb` are separate modules.
- **No external CLI framework** (hand-rolled docopt-style).
- **Never echo secrets** in commits, scripts, or shell history. Reach
  for `--password-stdin`-style stdin pipes; for reactorcide-managed
  secrets, set `REACTORCIDE_SECRETS_PASSWORD` inline per command from
  `~/.reactorcide-pass` rather than exporting.

