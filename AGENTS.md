# Longhouse

Coordination system for small-to-medium organizations and neighborhoods.
Tasks, calendar, projects, members, groups, and skills under a per-house
membership model.

## Architecture

- **api/**: Go API server. Single HTTP entry point — the CSIL-RPC v1 carrier at
  `POST /api/csil/v1/rpc` (self-routing CBOR envelope; the path is not semantic).
  Each op dispatches through a generated typed service interface
  (`api/internal/csil`, e.g. `TaskService`) via a small `csilrpc.Route[Req,Resp]`
  adapter backed by the generated per-op codec; the dispatcher (`csilrpc`) adds
  only bearer auth, audit, and the `ServiceError` arm. Service implementations
  live in `api/internal/csilservices`. Two extra non-RPC routes: `GET
  /api/v1/auth/start` (302 to linkkeys for the browser flow) and `GET
  /api/health` for k8s probes. No REST.
- **webapp/**: SolidJS + Vite SPA (the web frontend). Talks to the api through
  the generated client package `@longhouse/client` (the async client surface),
  wired in `webapp/src/data/clients.ts`, over a CBOR CSIL-RPC carrier
  (`webapp/src/transport/csilrpc.ts`). The carrier is a "dumb byte seam":
  the generated codec owns payload (de)serialization, the carrier owns only
  the envelope + `fetch` + auth. (Built into a static image by `Dockerfile.web`.)
- **clients/**: generated client libraries, one self-contained package per
  language, under `clients/<lang>/`. The SPA consumes `clients/typescript`
  (`@longhouse/client`) via a Vite/tsconfig path alias to its source. Generated
  by `clients/regenerate.sh` (don't hand-edit). Each package is dependency-free
  — the generated `codec.gen.*` owns the wire, so there is no separate transport
  library to vendor or pin.
- **csil/**: CSIL service definitions and types (`longhouse.csil`),
  source of truth for the Go server stubs AND every clients/ language.
  Run `./csil/regenerate.sh` after any spec change (it regenerates the Go
  server, then delegates to `clients/regenerate.sh` for all clients).
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
iteration loop. It `helm upgrade`s the `longhouse` release against the
foundry cluster (`~/.foundry/kubeconfig`). Two environments share that
release name, differing only by namespace + values file, and the script
hits **both by default**:

- **todandlorna** → namespace `longhouse` (`deploy/values-todandlorna.yaml`),
  the dogfood site at `https://longhouse.todandlorna.com`
- **catalystsquad** → namespace `longhouse-catalystsquad`
  (`deploy/values-catalystsquad.yaml`)

By default it deploys the **latest release** (the tag in
`version/VERSION.txt`) with no build. Building a fresh
`dev-<UTC>-<git-sha>[-dirty]` image is opt-in via `--dev`.

Common flows:

```bash
~/longhouse-redeploy.sh                 # deploy latest release (VERSION.txt)
                                        #   to BOTH envs, no build
~/longhouse-redeploy.sh --dev           # build local images → push to
                                        #   containers.catalystsquad.com under a
                                        #   dev tag → deploy that tag
~/longhouse-redeploy.sh --tag X.Y.Z     # pin to an explicit tag (no build)
~/longhouse-redeploy.sh --no-build      # config-only re-deploy (keeps each
                                        #   env's currently-running tag)
~/longhouse-redeploy.sh --env todandlorna     # restrict to one env
                                              #   (todandlorna|catalystsquad|both)
~/longhouse-redeploy.sh --logs          # tail api logs at the end (per env)
```

`--dev` is the only mode that pushes, so it's the only one needing
registry auth. One-time setup:

```bash
# Auth to the registry (token persists in ~/.docker/config.json):
REACTORCIDE_SECRETS_PASSWORD="$(cat ~/.reactorcide-pass)" \
  reactorcide secrets get catalystcommunity/registry password \
  | docker login containers.catalystsquad.com \
      -u "$(REACTORCIDE_SECRETS_PASSWORD="$(cat ~/.reactorcide-pass)" \
            reactorcide secrets get catalystcommunity/registry user)" \
      --password-stdin
```

CI (`.reactorcide/jobs/`) builds + pushes release images on push-to-main
bumps of `version/VERSION.txt`; the default `redeploy` then just rolls
those released tags, while `--dev` lets you iterate against the dogfood
namespaces without going through a release.

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

- **CSIL-first.** The spec drives the Go server stubs and every clients/
  language. Don't hand-modify generated files; edit `csil/longhouse.csil` and
  rerun `./csil/regenerate.sh`. `csilgen` is a required **system tool** (like
  docker/gofmt) — install it from its repo if missing (the regenerate scripts
  print the command). Consumer-side needs from csilgen are filed as markdown
  in `csilgen/docs/csilgen-requests/`. Status: all 14 generators emit; **13
  compile** (typescript, go, rust, python, ruby, elixir, csharp, dart, ocaml,
  zig, c, java, kotlin — verified via the `catalyst-tools` toolchains). Only
  **swift** is unverified, purely for lack of a local `swiftc` (it's a system
  install). `clients/regenerate.sh` keeps a `KNOWN_BROKEN` hook (currently
  empty) for any generator that stops emitting.
- **CSIL-RPC wire format.** `application/cbor` in both directions. Server
  fields are snake_case (Go JSON tags); the generated `codec.gen.*` emits those
  snake_case keys directly and maps them back to the camelCase generated types,
  so there is no camel↔snake bridging in the carrier anymore. The carrier moves
  only the envelope (`service`/kebab-`op`/tag-24 payload) + `fetch` + auth.
  Errors are CBOR-encoded `ServiceError { code, message }` with the matching
  HTTP status.
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

